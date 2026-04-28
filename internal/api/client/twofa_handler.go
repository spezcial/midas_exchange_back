package client

import (
	"encoding/json"
	"net/http"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
)

// TwoFAHandler covers:
//   - Phone OTP management (send / verify / remove)
//   - Login 2FA completion via Telegram code
//   - Action challenge / verify (for withdraw, password change, etc.)
//   - Forgot-password flow
type TwoFAHandler struct {
	authService  *service.AuthService
	twofaService service.TwoFactorService
}

func NewTwoFAHandler(authService *service.AuthService, twofaService service.TwoFactorService) *TwoFAHandler {
	return &TwoFAHandler{
		authService:  authService,
		twofaService: twofaService,
	}
}

// ─── Phone management ────────────────────────────────────────────────────────

// SendPhoneOTP sends a verification code to the given phone number.
// Does NOT save the phone — use VerifyPhone to confirm and persist it.
func (h *TwoFAHandler) SendPhoneOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.SendPhoneOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.authService.SetPhoneSendOTP(r.Context(), userID, req.Phone); err != nil {
		switch err {
		case service.ErrOTPRateLimited:
			respondError(w, http.StatusTooManyRequests, err.Error())
		default:
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
}

// VerifyPhone confirms the OTP and saves the phone number as verified.
func (h *TwoFAHandler) VerifyPhone(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.VerifyPhoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.authService.VerifyAndSavePhone(r.Context(), userID, req.Phone, req.Code); err != nil {
		switch err {
		case service.ErrOTPInvalid, service.ErrOTPMaxAttempts:
			respondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Phone number verified and saved"})
}

// RemovePhone removes the user's phone number.
// Blocked if the user has no passkeys registered.
func (h *TwoFAHandler) RemovePhone(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.authService.RemovePhone(r.Context(), userID); err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Phone number removed"})
}

// ─── Login 2FA completion ────────────────────────────────────────────────────

// SendLoginTelegram sends a Telegram OTP for a pending login session.
// Called when the user explicitly selects the Telegram method on the frontend.
func (h *TwoFAHandler) SendLoginTelegram(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TempToken string `json:"temp_token" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.authService.SendLoginTelegramOTP(r.Context(), req.TempToken); err != nil {
		switch err {
		case service.ErrOTPRateLimited:
			respondError(w, http.StatusTooManyRequests, err.Error())
		default:
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
}

// VerifyLoginTelegram completes a pending login by verifying a Telegram OTP.
func (h *TwoFAHandler) VerifyLoginTelegram(w http.ResponseWriter, r *http.Request) {
	var req models.Verify2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve the pending session to get the phone number.
	// We peek at the pending state to find the user's phone, verify the OTP, then complete login.
	// This is done via a dedicated method that keeps the temp token alive if OTP fails.
	result, err := h.verifyTelegramAndComplete(r, req.TempToken, req.Code)
	if err != nil {
		switch err {
		case service.ErrOTPInvalid, service.ErrOTPMaxAttempts:
			respondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respondError(w, http.StatusUnauthorized, err.Error())
		}
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

// verifyTelegramAndComplete peeks at the pending session, verifies the OTP against
// the user's phone, then (only on success) completes login.
func (h *TwoFAHandler) verifyTelegramAndComplete(r *http.Request, tempToken, code string) (*service.LoginResult, error) {
	ctx := r.Context()

	// We need the user's phone to verify the OTP. Use a read-only peek method.
	userID, phone, err := h.authService.PeekPending2FAPhone(ctx, tempToken)
	if err != nil {
		return nil, err
	}

	// Verify the code — this is atomic (see twofa_service).
	if err := h.twofaService.VerifyOTP(ctx, phone, code, domain.OTPPurposeLogin); err != nil {
		return nil, err
	}

	// OTP is valid — consume the pending session and issue tokens.
	_ = userID // userID confirmed via PeekPending2FAPhone; CompleteTwoFALogin reads it from temp token
	return h.authService.CompleteTwoFALogin(ctx, tempToken)
}

// ─── Action challenge / verify ───────────────────────────────────────────────

// ActionChallengeTelegram sends an OTP to the user's phone for a sensitive action.
// The client should then call ActionVerifyTelegram with the received code.
func (h *TwoFAHandler) ActionChallengeTelegram(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ActionChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	phone, err := h.authService.GetVerifiedPhone(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusConflict, "No verified phone number on file — use passkey confirmation instead")
		return
	}

	if err := h.twofaService.SendOTP(r.Context(), userID, phone, domain.OTPPurposeAction); err != nil {
		switch err {
		case service.ErrOTPRateLimited:
			respondError(w, http.StatusTooManyRequests, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "Failed to send verification code")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
}

// ActionVerifyTelegram validates the OTP and returns a single-use action token.
func (h *TwoFAHandler) ActionVerifyTelegram(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ActionVerifyTelegramRequest
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

	phone, err := h.authService.GetVerifiedPhone(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusConflict, "No verified phone number on file")
		return
	}

	if err := h.twofaService.VerifyOTP(r.Context(), phone, req.Code, domain.OTPPurposeAction); err != nil {
		switch err {
		case service.ErrOTPInvalid, service.ErrOTPMaxAttempts:
			respondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "Verification failed")
		}
		return
	}

	token, err := h.twofaService.IssueActionToken(r.Context(), userID, action)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to issue action token")
		return
	}

	respondJSON(w, http.StatusOK, models.ActionTokenResponse{ActionToken: token})
}

// ─── Forgot-password ─────────────────────────────────────────────────────────

// ForgotPasswordSend initiates the password-reset flow by sending an OTP to
// the registered phone. Always returns 200 to prevent user enumeration.
func (h *TwoFAHandler) ForgotPasswordSend(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Intentionally ignore error — see ForgotPasswordSend docstring.
	_ = h.authService.ForgotPasswordSend(r.Context(), req.Email)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If an account with that email exists and has a phone number configured, you will receive a verification code.",
	})
}

// ForgotPasswordVerify validates the OTP and returns a one-time reset token.
func (h *TwoFAHandler) ForgotPasswordVerify(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	resetToken, err := h.authService.ForgotPasswordVerify(r.Context(), req.Phone, req.Code)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "Invalid or expired verification code")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"reset_token": resetToken})
}

// ForgotPasswordReset sets a new password using the one-time reset token.
func (h *TwoFAHandler) ForgotPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.authService.ForgotPasswordReset(r.Context(), req.ResetToken, req.NewPassword); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "Invalid or expired reset token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func isValidAction(a domain.ActionType) bool {
	switch a {
	case domain.ActionWithdraw, domain.ActionChangePassword, domain.ActionResetPassword:
		return true
	}
	return false
}
