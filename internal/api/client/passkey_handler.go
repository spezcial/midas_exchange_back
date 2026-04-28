package client

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
	"github.com/go-chi/chi/v5"
)

// PasskeyHandler manages WebAuthn / FIDO2 passkey registration, listing, deletion,
// and authentication (used as the 2FA method during login and action confirmation).
type PasskeyHandler struct {
	webAuthnService service.WebAuthnService
	authService     *service.AuthService
	twofaService    service.TwoFactorService
}

func NewPasskeyHandler(
	webAuthnService service.WebAuthnService,
	authService *service.AuthService,
	twofaService service.TwoFactorService,
) *PasskeyHandler {
	return &PasskeyHandler{
		webAuthnService: webAuthnService,
		authService:     authService,
		twofaService:    twofaService,
	}
}

// ─── Passkey registration ────────────────────────────────────────────────────

// BeginRegistration starts the WebAuthn credential creation ceremony.
// Returns creation options JSON + a session_id to track the ceremony.
func (h *PasskeyHandler) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.BeginPasskeyRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load user")
		return
	}

	creation, sessionID, err := h.webAuthnService.BeginRegistration(r.Context(), user, req.Name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"session_id":       sessionID,
		"creation_options": creation,
	})
}

// FinishRegistration completes the WebAuthn credential creation ceremony.
// The raw attestation body from the authenticator is forwarded via the HTTP request.
func (h *PasskeyHandler) FinishRegistration(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	cred, err := h.webAuthnService.FinishRegistration(r.Context(), userID, sessionID, r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, cred)
}

// ─── Passkey list & delete ───────────────────────────────────────────────────

// ListPasskeys returns all passkeys registered for the current user.
func (h *PasskeyHandler) ListPasskeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	creds, err := h.webAuthnService.ListCredentials(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, creds)
}

// DeletePasskey removes a passkey by ID.
// Blocked if the user has no verified phone number (would lock them out).
func (h *PasskeyHandler) DeletePasskey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid passkey ID")
		return
	}

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load user")
		return
	}

	if !user.PhoneVerified || user.Phone == nil {
		respondError(w, http.StatusConflict, "Cannot remove passkey: you must have a verified phone number as a backup")
		return
	}

	if err := h.webAuthnService.DeleteCredential(r.Context(), id, userID); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Passkey removed"})
}

// ─── Passkey authentication (login 2FA) ──────────────────────────────────────

// BeginLoginAuth starts the WebAuthn assertion ceremony for completing a pending login.
func (h *PasskeyHandler) BeginLoginAuth(w http.ResponseWriter, r *http.Request) {
	var req models.BeginPasskeyAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Peek at the pending session to get the userID without consuming it.
	userID, _, err := h.authService.PeekPending2FAPhone(r.Context(), req.TempToken)
	if err != nil {
		// PeekPending2FAPhone returns an error if no phone is set; that's fine
		// here — we just need the userID. Use a separate method.
		pending, peekErr := h.twofaService.PeekPending2FA(r.Context(), req.TempToken)
		if peekErr != nil {
			respondError(w, http.StatusUnauthorized, "Invalid or expired 2FA session")
			return
		}
		userID = pending.UserID
		_ = err
	}

	assertion, sessionID, err := h.webAuthnService.BeginAuthentication(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"session_id":        sessionID,
		"assertion_options": assertion,
	})
}

// FinishLoginAuth completes the WebAuthn assertion and issues JWT tokens.
func (h *PasskeyHandler) FinishLoginAuth(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	tempToken := r.URL.Query().Get("temp_token")
	if sessionID == "" || tempToken == "" {
		respondError(w, http.StatusBadRequest, "session_id and temp_token are required")
		return
	}

	// Get userID from pending session (non-consuming).
	pending, err := h.twofaService.PeekPending2FA(r.Context(), tempToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid or expired 2FA session")
		return
	}

	if err := h.webAuthnService.FinishAuthentication(r.Context(), pending.UserID, sessionID, r); err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Passkey assertion passed — complete the login.
	result, err := h.authService.CompleteTwoFALogin(r.Context(), tempToken)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.LoginResponse{
		Status:           "ok",
		AccessToken:      result.Auth.AccessToken,
		RefreshToken:     result.Auth.RefreshToken,
		User:             result.Auth.User,
		TwoFactorEnabled: result.TwoFactorEnabled,
		PasskeyEnabled:   result.PasskeyEnabled,
	})
}

// ─── Passkey action authentication ───────────────────────────────────────────

// BeginActionAuth starts a WebAuthn assertion ceremony for a sensitive action.
func (h *PasskeyHandler) BeginActionAuth(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ActionWebAuthnBeginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !isValidAction(domain.ActionType(req.Action)) {
		respondError(w, http.StatusBadRequest, "Invalid action type")
		return
	}

	assertion, sessionID, err := h.webAuthnService.BeginAuthentication(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"session_id":        sessionID,
		"assertion_options": assertion,
	})
}

// FinishActionAuth validates the assertion and returns a single-use action token.
func (h *PasskeyHandler) FinishActionAuth(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ActionWebAuthnFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	action := domain.ActionType(req.Action)
	if !isValidAction(action) {
		respondError(w, http.StatusBadRequest, "Invalid action type")
		return
	}

	if err := h.webAuthnService.FinishAuthentication(r.Context(), userID, req.SessionID, r); err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	token, err := h.twofaService.IssueActionToken(r.Context(), userID, action)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to issue action token")
		return
	}

	respondJSON(w, http.StatusOK, models.ActionTokenResponse{ActionToken: token})
}
