package models

import "github.com/caspianex/exchange-backend/internal/domain"

type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

type LoginRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
	RememberMe bool   `json:"remember_me"`
}

// LoginResponse is returned by the login endpoint.
// When Status is "pending_2fa" the client must complete one of the 2FA flows.
// When Status is "ok" the full auth tokens are included.
type LoginResponse struct {
	Status           string       `json:"status"`                    // "ok" | "pending_2fa"
	TempToken        string       `json:"temp_token,omitempty"`      // pending_2fa only
	Methods          []string     `json:"methods,omitempty"`         // pending_2fa only: ["telegram","passkey"]
	AccessToken      string       `json:"access_token,omitempty"`    // ok only
	RefreshToken     string       `json:"refresh_token,omitempty"`   // ok only
	User             *domain.User `json:"user,omitempty"`            // ok only
	TwoFactorEnabled bool         `json:"two_factor_enabled"`        // true if user has a verified phone
	PasskeyEnabled   bool         `json:"passkey_enabled"`           // true if user has at least one passkey
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *domain.User `json:"user"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
	ActionToken     string `json:"action_token" validate:"required"`
}

// --- 2FA flows ---------------------------------------------------------------

type Verify2FARequest struct {
	TempToken string `json:"temp_token" validate:"required"`
	Code      string `json:"code" validate:"required,len=6"`
}

type ActionChallengeRequest struct {
	Action string `json:"action" validate:"required"`
}

type ActionVerifyTelegramRequest struct {
	Action string `json:"action" validate:"required"`
	Code   string `json:"code" validate:"required,len=6"`
}

type ActionTokenResponse struct {
	ActionToken string `json:"action_token"`
}

// --- Phone management --------------------------------------------------------

type SendPhoneOTPRequest struct {
	Phone string `json:"phone" validate:"required"`
}

type VerifyPhoneRequest struct {
	Phone string `json:"phone" validate:"required"`
	Code  string `json:"code" validate:"required,len=6"`
}

// --- Forgot password ---------------------------------------------------------

type ForgotPasswordSendRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ForgotPasswordVerifyRequest struct {
	Phone string `json:"phone" validate:"required"`
	Code  string `json:"code" validate:"required,len=6"`
}

type ForgotPasswordResetRequest struct {
	ResetToken  string `json:"reset_token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// --- WebAuthn / Passkey ------------------------------------------------------

type BeginPasskeyRegistrationRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type BeginPasskeyAuthRequest struct {
	TempToken string `json:"temp_token" validate:"required"` // from pending_2fa login
}

type WebAuthnSessionResponse struct {
	SessionID string `json:"session_id"`
}

type ActionWebAuthnBeginRequest struct {
	Action string `json:"action" validate:"required"`
}

type ActionWebAuthnFinishRequest struct {
	Action    string `json:"action" validate:"required"`
	SessionID string `json:"session_id" validate:"required"`
}
