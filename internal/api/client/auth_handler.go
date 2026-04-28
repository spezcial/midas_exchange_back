package client

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
)

type AuthHandler struct {
	authService    *service.AuthService
	authMiddleware *middleware.AuthMiddleware
}

func NewAuthHandler(authService *service.AuthService, authMiddleware *middleware.AuthMiddleware) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		authMiddleware: authMiddleware,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	clientIP := extractClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	result, err := h.authService.Login(r.Context(), &req, clientIP, userAgent)
	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if result.TwoFARequired {
		respondJSON(w, http.StatusOK, models.LoginResponse{
			Status:           "pending_2fa",
			TempToken:        result.TempToken,
			Methods:          result.Methods,
			TwoFactorEnabled: result.TwoFactorEnabled,
			PasskeyEnabled:   result.PasskeyEnabled,
		})
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

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshTokenRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req models.LogoutRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Blocklist the access token so it can't be reused after logout.
	if rawToken, ok := middleware.GetRawToken(r.Context()); ok {
		_ = h.authMiddleware.BlockRawToken(r.Context(), rawToken)
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	result, err := h.authService.GetMe(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	user := *result.User
	user.PasswordHash = ""

	respondJSON(w, http.StatusOK, map[string]any{
		"user":               &user,
		"two_factor_enabled": result.TwoFactorEnabled,
		"passkey_enabled":    result.PasskeyEnabled,
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.authService.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword, req.ActionToken); err != nil {
		switch err {
		case service.ErrNoTwoFAMethod:
			respondError(w, http.StatusForbidden, err.Error())
		case service.ErrActionTokenInvalid:
			respondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

// extractClientIP retrieves the real client IP respecting common proxy headers.
// Only X-Forwarded-For and X-Real-IP are checked; RemoteAddr is the fallback.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the leftmost (original client) IP.
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

