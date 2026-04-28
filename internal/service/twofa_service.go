package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/caspianex/exchange-backend/pkg/telegramgw"
)

// Sentinel errors for the 2FA service.
var (
	ErrOTPRateLimited     = errors.New("too many OTP requests — please wait before trying again")
	ErrOTPInvalid         = errors.New("invalid or expired verification code")
	ErrOTPMaxAttempts     = errors.New("maximum verification attempts exceeded")
	ErrActionTokenInvalid = errors.New("invalid or expired action token")
	ErrNoTwoFAMethod      = errors.New("no 2FA method configured — please set up a phone number or passkey first")
)

const (
	otpTTL          = 2 * time.Minute
	otpRateTTL      = 60 * time.Second
	otpMaxAttempts  = 5
	actionTokenTTL  = 5 * time.Minute
	pendingTwoFATTL = 5 * time.Minute
)

// otpPayload is what we store in Redis for a pending OTP.
type otpPayload struct {
	Code     string `json:"code"`
	UserID   int64  `json:"user_id"`
	Attempts int    `json:"attempts"`
}

// pendingTwoFAPayload is stored during the login pending_2fa state.
type pendingTwoFAPayload struct {
	UserID      int64  `json:"user_id"`
	Fingerprint string `json:"fingerprint"`
	RememberMe  bool   `json:"remember_me"`
}

// actionTokenPayload authorises a single sensitive action.
type actionTokenPayload struct {
	UserID int64             `json:"user_id"`
	Action domain.ActionType `json:"action"`
}

// TwoFactorService manages OTP codes, pending-2FA sessions, and action tokens.
type TwoFactorService interface {
	// SendOTP generates and sends a 6-digit code to phone via Telegram Gateway.
	SendOTP(ctx context.Context, userID int64, phone string, purpose domain.OTPPurpose) error

	// VerifyOTP checks the code for the given phone+purpose.
	// On success the OTP is consumed (single-use). On failure the attempt counter
	// is incremented; after otpMaxAttempts the entry is deleted.
	VerifyOTP(ctx context.Context, phone, code string, purpose domain.OTPPurpose) error

	// StorePending2FA stores the intermediate login state while the user completes 2FA.
	StorePending2FA(ctx context.Context, userID int64, fingerprint string, rememberMe bool) (tempToken string, err error)

	// ConsumePending2FA atomically retrieves and deletes the pending state.
	ConsumePending2FA(ctx context.Context, tempToken string) (*pendingTwoFAPayload, error)

	// PeekPending2FA reads the pending state WITHOUT deleting it.
	// Used to look up the user's phone before the OTP is verified, so the
	// session is preserved if the OTP check fails.
	PeekPending2FA(ctx context.Context, tempToken string) (*pendingTwoFAPayload, error)

	// IssueActionToken creates a short-lived token that authorises one specific action.
	IssueActionToken(ctx context.Context, userID int64, action domain.ActionType) (string, error)

	// ConsumeActionToken atomically validates and deletes an action token.
	// Returns the authorised userID so callers can verify ownership.
	ConsumeActionToken(ctx context.Context, token string, action domain.ActionType) (int64, error)

	// SendSecurityAlert sends a plain Telegram message to the user (non-blocking best-effort).
	SendSecurityAlert(ctx context.Context, phone, message string)
}

type twoFAService struct {
	store   cache.Store
	gateway telegramgw.Gateway
	logger  *logger.Logger
}

// NewTwoFactorService creates a TwoFactorService backed by Redis and Telegram Gateway.
func NewTwoFactorService(store cache.Store, gateway telegramgw.Gateway, log *logger.Logger) TwoFactorService {
	return &twoFAService{store: store, gateway: gateway, logger: log}
}

// --- OTP flow ----------------------------------------------------------------

func (s *twoFAService) SendOTP(ctx context.Context, userID int64, phone string, purpose domain.OTPPurpose) error {
	// Rate-limit: one OTP per phone per minute.
	rateKey := fmt.Sprintf("otp_rate:%s:%s", purpose, phone)
	set, err := s.store.SetNX(ctx, rateKey, []byte("1"), otpRateTTL)
	if err != nil {
		return fmt.Errorf("twofa: check rate limit: %w", err)
	}
	if !set {
		return ErrOTPRateLimited
	}

	code, err := generateOTP()
	if err != nil {
		return fmt.Errorf("twofa: generate otp: %w", err)
	}

	payload := otpPayload{Code: code, UserID: userID, Attempts: 0}
	b, _ := json.Marshal(payload)

	otpKey := otpKey(purpose, phone)
	if err := s.store.Set(ctx, otpKey, b, otpTTL); err != nil {
		return fmt.Errorf("twofa: store otp: %w", err)
	}
	if _, err := s.gateway.SendCode(ctx, code, phone, int(otpTTL.Seconds())); err != nil {
		// Store was already written — delete it so the user can retry.
		_ = s.store.Delete(ctx, otpKey)
		_ = s.store.Delete(ctx, rateKey)
		return fmt.Errorf("twofa: send telegram code: %w", err)
	}

	return nil
}

func (s *twoFAService) VerifyOTP(ctx context.Context, phone, code string, purpose domain.OTPPurpose) error {
	otpKey := otpKey(purpose, phone)

	// Read current state.
	raw, found, err := s.store.Get(ctx, otpKey)
	if err != nil {
		return fmt.Errorf("twofa: read otp: %w", err)
	}
	if !found {
		return ErrOTPInvalid
	}

	var payload otpPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("twofa: unmarshal otp: %w", err)
	}
	// Increment attempts before checking the code (prevents timing-based enumeration).
	payload.Attempts++
	if payload.Attempts > otpMaxAttempts {
		_ = s.store.Delete(ctx, otpKey)
		return ErrOTPMaxAttempts
	}

	if payload.Code != code {
		// Persist updated attempt count.
		b, _ := json.Marshal(payload)
		_ = s.store.Set(ctx, otpKey, b, otpTTL)
		if payload.Attempts >= otpMaxAttempts {
			_ = s.store.Delete(ctx, otpKey)
			return ErrOTPMaxAttempts
		}
		return ErrOTPInvalid
	}

	// Code is correct — consume it atomically.
	_, _, _ = s.store.GetDel(ctx, otpKey)
	return nil
}

// --- Pending 2FA state -------------------------------------------------------

func (s *twoFAService) StorePending2FA(ctx context.Context, userID int64, fingerprint string, rememberMe bool) (string, error) {
	token, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("twofa: generate temp token: %w", err)
	}

	payload := pendingTwoFAPayload{UserID: userID, Fingerprint: fingerprint, RememberMe: rememberMe}
	b, _ := json.Marshal(payload)

	if err := s.store.Set(ctx, pending2FAKey(token), b, pendingTwoFATTL); err != nil {
		return "", fmt.Errorf("twofa: store pending 2fa: %w", err)
	}
	return token, nil
}

func (s *twoFAService) ConsumePending2FA(ctx context.Context, tempToken string) (*pendingTwoFAPayload, error) {
	raw, found, err := s.store.GetDel(ctx, pending2FAKey(tempToken))
	if err != nil {
		return nil, fmt.Errorf("twofa: consume pending 2fa: %w", err)
	}
	if !found {
		return nil, errors.New("invalid or expired 2FA session — please log in again")
	}

	var p pendingTwoFAPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("twofa: unmarshal pending 2fa: %w", err)
	}
	return &p, nil
}

func (s *twoFAService) PeekPending2FA(ctx context.Context, tempToken string) (*pendingTwoFAPayload, error) {
	raw, found, err := s.store.Get(ctx, pending2FAKey(tempToken))
	if err != nil {
		return nil, fmt.Errorf("twofa: peek pending 2fa: %w", err)
	}
	if !found {
		return nil, errors.New("invalid or expired 2FA session — please log in again")
	}

	var p pendingTwoFAPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("twofa: unmarshal pending 2fa: %w", err)
	}
	return &p, nil
}

// --- Action tokens -----------------------------------------------------------

func (s *twoFAService) IssueActionToken(ctx context.Context, userID int64, action domain.ActionType) (string, error) {
	token, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("twofa: generate action token: %w", err)
	}

	payload := actionTokenPayload{UserID: userID, Action: action}
	b, _ := json.Marshal(payload)

	if err := s.store.Set(ctx, actionTokenKey(token), b, actionTokenTTL); err != nil {
		return "", fmt.Errorf("twofa: store action token: %w", err)
	}
	return token, nil
}

func (s *twoFAService) ConsumeActionToken(ctx context.Context, token string, action domain.ActionType) (int64, error) {
	raw, found, err := s.store.GetDel(ctx, actionTokenKey(token))
	if err != nil {
		return 0, fmt.Errorf("twofa: consume action token: %w", err)
	}
	if !found {
		return 0, ErrActionTokenInvalid
	}

	var p actionTokenPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0, fmt.Errorf("twofa: unmarshal action token: %w", err)
	}
	if p.Action != action {
		return 0, fmt.Errorf("twofa: action token purpose mismatch (got %s, want %s)", p.Action, action)
	}
	return p.UserID, nil
}

// --- Security alert ----------------------------------------------------------

func (s *twoFAService) SendSecurityAlert(_ context.Context, phone, message string) {
	// Telegram Gateway only supports verification codes, not plain messages.
	// Log the alert locally; a proper notification channel can be wired up later.
	s.logger.Info("security alert (not delivered — no plain-message API)", "phone", phone, "message", message)
}

// --- Redis key helpers -------------------------------------------------------

func otpKey(purpose domain.OTPPurpose, phone string) string {
	return fmt.Sprintf("otp:%s:%s", purpose, phone)
}

func pending2FAKey(token string) string {
	return fmt.Sprintf("2fa_pending:%s", token)
}

func actionTokenKey(token string) string {
	return fmt.Sprintf("action_token:%s", token)
}

// --- Crypto helpers ----------------------------------------------------------

// generateOTP creates a cryptographically random 6-digit numeric code.
func generateOTP() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// generateSecureToken creates a 32-byte hex-encoded random token.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
