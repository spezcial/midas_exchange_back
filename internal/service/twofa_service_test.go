package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/telegramgw"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTwoFAService() TwoFactorService {
	store := cache.NewMemoryCache(5*time.Minute, 10*time.Minute)
	gw := &telegramgw.NoOpGateway{}
	return NewTwoFactorService(store, gw, nil)
}

func newTwoFAServiceWithStore(store cache.Store) TwoFactorService {
	gw := &telegramgw.NoOpGateway{}
	return NewTwoFactorService(store, gw, nil)
}

// ─── OTP tests ───────────────────────────────────────────────────────────────

func TestSendOTP_Success(t *testing.T) {
	svc := newTwoFAService()
	err := svc.SendOTP(context.Background(), 1, "+77001234567", domain.OTPPurposeLogin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendOTP_RateLimited(t *testing.T) {
	svc := newTwoFAService()
	phone := "+77001234568"

	if err := svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin); err != nil {
		t.Fatalf("first OTP failed: %v", err)
	}

	// Second request within 60 s should be rate-limited.
	if err := svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin); err != ErrOTPRateLimited {
		t.Fatalf("expected ErrOTPRateLimited, got: %v", err)
	}
}

func TestVerifyOTP_Success(t *testing.T) {
	store := cache.NewMemoryCache(5*time.Minute, 10*time.Minute)
	svc := newTwoFAServiceWithStore(store)

	phone := "+77001234569"
	_ = svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin)

	// Peek at the stored code.
	code := extractStoredCode(t, store, domain.OTPPurposeLogin, phone)

	if err := svc.VerifyOTP(context.Background(), phone, code, domain.OTPPurposeLogin); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestVerifyOTP_WrongCode(t *testing.T) {
	store := cache.NewMemoryCache(5*time.Minute, 10*time.Minute)
	svc := newTwoFAServiceWithStore(store)

	phone := "+77001234570"
	_ = svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin)

	err := svc.VerifyOTP(context.Background(), phone, "000000", domain.OTPPurposeLogin)
	if err != ErrOTPInvalid {
		t.Fatalf("expected ErrOTPInvalid, got: %v", err)
	}
}

func TestVerifyOTP_MaxAttempts(t *testing.T) {
	store := cache.NewMemoryCache(5*time.Minute, 10*time.Minute)
	svc := newTwoFAServiceWithStore(store)

	phone := "+77001234571"
	_ = svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin)

	for i := 0; i < otpMaxAttempts; i++ {
		_ = svc.VerifyOTP(context.Background(), phone, "000000", domain.OTPPurposeLogin)
	}

	// After max attempts, key is deleted — any further attempt returns invalid.
	err := svc.VerifyOTP(context.Background(), phone, "000000", domain.OTPPurposeLogin)
	if err != ErrOTPInvalid && err != ErrOTPMaxAttempts {
		t.Fatalf("expected OTP locked out error, got: %v", err)
	}
}

func TestVerifyOTP_SingleUse(t *testing.T) {
	store := cache.NewMemoryCache(5*time.Minute, 10*time.Minute)
	svc := newTwoFAServiceWithStore(store)

	phone := "+77001234572"
	_ = svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin)

	code := extractStoredCode(t, store, domain.OTPPurposeLogin, phone)

	// First use — should succeed.
	if err := svc.VerifyOTP(context.Background(), phone, code, domain.OTPPurposeLogin); err != nil {
		t.Fatalf("first verify failed: %v", err)
	}

	// Second use of the same code — OTP was consumed.
	if err := svc.VerifyOTP(context.Background(), phone, code, domain.OTPPurposeLogin); err == nil {
		t.Fatal("expected error on second use, got nil")
	}
}

func TestVerifyOTP_PurposeMismatch(t *testing.T) {
	store := cache.NewMemoryCache(5*time.Minute, 10*time.Minute)
	svc := newTwoFAServiceWithStore(store)

	phone := "+77001234573"
	_ = svc.SendOTP(context.Background(), 1, phone, domain.OTPPurposeLogin)
	code := extractStoredCode(t, store, domain.OTPPurposeLogin, phone)

	// Verifying with a different purpose should fail (key doesn't exist for that purpose).
	err := svc.VerifyOTP(context.Background(), phone, code, domain.OTPPurposeAction)
	if err != ErrOTPInvalid {
		t.Fatalf("expected ErrOTPInvalid for purpose mismatch, got: %v", err)
	}
}

// ─── Pending 2FA tests ───────────────────────────────────────────────────────

func TestPending2FA_StoreAndConsume(t *testing.T) {
	svc := newTwoFAService()

	token, err := svc.StorePending2FA(context.Background(), 42, "fp_abc123", true)
	if err != nil || token == "" {
		t.Fatalf("StorePending2FA failed: %v", err)
	}

	payload, err := svc.ConsumePending2FA(context.Background(), token)
	if err != nil {
		t.Fatalf("ConsumePending2FA failed: %v", err)
	}

	if payload.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", payload.UserID)
	}
	if payload.Fingerprint != "fp_abc123" {
		t.Errorf("expected fingerprint fp_abc123, got %s", payload.Fingerprint)
	}
	if !payload.RememberMe {
		t.Error("expected remember_me=true")
	}
}

func TestPending2FA_SingleUse(t *testing.T) {
	svc := newTwoFAService()

	token, _ := svc.StorePending2FA(context.Background(), 1, "fp", false)

	_, err1 := svc.ConsumePending2FA(context.Background(), token)
	if err1 != nil {
		t.Fatalf("first consume failed: %v", err1)
	}

	// Second consume must fail — token was deleted.
	_, err2 := svc.ConsumePending2FA(context.Background(), token)
	if err2 == nil {
		t.Fatal("expected error on second consume, got nil")
	}
}

func TestPending2FA_Peek(t *testing.T) {
	svc := newTwoFAService()

	token, _ := svc.StorePending2FA(context.Background(), 7, "fp_peek", false)

	// Peek should NOT delete the token.
	p1, err := svc.PeekPending2FA(context.Background(), token)
	if err != nil || p1.UserID != 7 {
		t.Fatalf("PeekPending2FA failed: %v", err)
	}

	// Consume should still work after peek.
	p2, err := svc.ConsumePending2FA(context.Background(), token)
	if err != nil || p2.UserID != 7 {
		t.Fatalf("ConsumePending2FA after peek failed: %v", err)
	}
}

// ─── Action token tests ──────────────────────────────────────────────────────

func TestActionToken_IssueAndConsume(t *testing.T) {
	svc := newTwoFAService()

	token, err := svc.IssueActionToken(context.Background(), 10, domain.ActionWithdraw)
	if err != nil || token == "" {
		t.Fatalf("IssueActionToken failed: %v", err)
	}

	userID, err := svc.ConsumeActionToken(context.Background(), token, domain.ActionWithdraw)
	if err != nil {
		t.Fatalf("ConsumeActionToken failed: %v", err)
	}
	if userID != 10 {
		t.Errorf("expected user_id 10, got %d", userID)
	}
}

func TestActionToken_WrongPurpose(t *testing.T) {
	svc := newTwoFAService()

	token, _ := svc.IssueActionToken(context.Background(), 10, domain.ActionWithdraw)

	_, err := svc.ConsumeActionToken(context.Background(), token, domain.ActionChangePassword)
	if err == nil {
		t.Fatal("expected error on purpose mismatch, got nil")
	}
}

func TestActionToken_SingleUse(t *testing.T) {
	svc := newTwoFAService()

	token, _ := svc.IssueActionToken(context.Background(), 10, domain.ActionWithdraw)

	_, err1 := svc.ConsumeActionToken(context.Background(), token, domain.ActionWithdraw)
	if err1 != nil {
		t.Fatalf("first consume failed: %v", err1)
	}

	_, err2 := svc.ConsumeActionToken(context.Background(), token, domain.ActionWithdraw)
	if err2 == nil {
		t.Fatal("expected error on second consume, got nil")
	}
}

func TestActionToken_Invalid(t *testing.T) {
	svc := newTwoFAService()

	_, err := svc.ConsumeActionToken(context.Background(), "does-not-exist", domain.ActionWithdraw)
	if err != ErrActionTokenInvalid {
		t.Fatalf("expected ErrActionTokenInvalid, got: %v", err)
	}
}

// ─── internal helper ─────────────────────────────────────────────────────────

// extractStoredCode reads the OTP payload directly from the MemoryCache so tests
// can provide the correct code without going through Telegram.
func extractStoredCode(t *testing.T, store cache.Store, purpose domain.OTPPurpose, phone string) string {
	t.Helper()
	key := otpKey(purpose, phone)
	raw, found, err := store.Get(context.Background(), key)
	if err != nil || !found {
		t.Fatalf("OTP not found in store (err=%v, found=%v)", err, found)
	}
	var p otpPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal OTP payload: %v", err)
	}
	return p.Code
}
