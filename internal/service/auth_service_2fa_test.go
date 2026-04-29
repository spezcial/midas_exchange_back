package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	pkgauth "github.com/caspianex/exchange-backend/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

// ─── In-memory fakes ─────────────────────────────────────────────────────────

type fakeUserRepo struct {
	users    map[int64]*domain.User
	byEmail  map[string]*domain.User
	byPhone  map[string]*domain.User
	sessions map[string]*domain.UserSession
}

func newFakeUserRepo(users ...*domain.User) *fakeUserRepo {
	r := &fakeUserRepo{
		users:    make(map[int64]*domain.User),
		byEmail:  make(map[string]*domain.User),
		byPhone:  make(map[string]*domain.User),
		sessions: make(map[string]*domain.UserSession),
	}
	for _, u := range users {
		u2 := *u
		r.users[u.ID] = &u2
		r.byEmail[u.Email] = &u2
		if u.Phone != nil {
			r.byPhone[*u.Phone] = &u2
		}
	}
	return r
}

func (f *fakeUserRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	if u, ok := f.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, errors.New("user not found")
}
func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if u, ok := f.byEmail[email]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}
func (f *fakeUserRepo) GetByEmailAnyRole(ctx context.Context, email string) (*domain.User, error) {
	return f.GetByEmail(ctx, email)
}
func (f *fakeUserRepo) GetByPhone(_ context.Context, phone string) (*domain.User, error) {
	if u, ok := f.byPhone[phone]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}
func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	u.ID = int64(len(f.users) + 1)
	cp := *u
	f.users[u.ID] = &cp
	f.byEmail[u.Email] = &cp
	return nil
}
func (f *fakeUserRepo) UpdatePhone(_ context.Context, userID int64, phone *string, verified bool) error {
	u, ok := f.users[userID]
	if !ok {
		return errors.New("not found")
	}
	u.Phone = phone
	u.PhoneVerified = verified
	if phone != nil {
		f.byPhone[*phone] = u
	}
	return nil
}
func (f *fakeUserRepo) UpdatePassword(_ context.Context, _ int64, _ string) error { return nil }
func (f *fakeUserRepo) DeleteSession(_ context.Context, token string) error {
	delete(f.sessions, token)
	return nil
}
func (f *fakeUserRepo) DeleteAllSessionsByUserID(_ context.Context, _ int64) error { return nil }
func (f *fakeUserRepo) CreateSession(_ context.Context, s *domain.UserSession) error {
	s.ID = int64(len(f.sessions) + 1)
	s.CreatedAt = time.Now()
	f.sessions[s.RefreshToken] = s
	return nil
}
func (f *fakeUserRepo) GetSessionByToken(_ context.Context, token string) (*domain.UserSession, error) {
	if s, ok := f.sessions[token]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

type fakePasskeyRepo struct {
	counts map[int64]int
}

func newFakePasskeyRepo() *fakePasskeyRepo {
	return &fakePasskeyRepo{counts: make(map[int64]int)}
}

func (f *fakePasskeyRepo) Create(_ context.Context, _ *domain.PasskeyCredential) error { return nil }
func (f *fakePasskeyRepo) GetByUserID(_ context.Context, _ int64) ([]domain.PasskeyCredential, error) {
	return nil, nil
}
func (f *fakePasskeyRepo) GetByCredentialID(_ context.Context, _ string) (*domain.PasskeyCredential, error) {
	return nil, errors.New("not found")
}
func (f *fakePasskeyRepo) UpdateSignCount(_ context.Context, _ string, _ uint32, _ bool) error {
	return nil
}
func (f *fakePasskeyRepo) Delete(_ context.Context, _ int64, _ int64) error            { return nil }
func (f *fakePasskeyRepo) CountByUserID(_ context.Context, id int64) (int, error) {
	return f.counts[id], nil
}

type fakeFingerprintRepo struct {
	prints map[string]bool
	counts map[int64]int
}

func newFakeFingerprintRepo() *fakeFingerprintRepo {
	return &fakeFingerprintRepo{
		prints: make(map[string]bool),
		counts: make(map[int64]int),
	}
}

func (f *fakeFingerprintRepo) Exists(_ context.Context, userID int64, fp string) (bool, error) {
	return f.prints[fmt.Sprintf("%d|%s", userID, fp)], nil
}
func (f *fakeFingerprintRepo) CountByUserID(_ context.Context, id int64) (int, error) {
	return f.counts[id], nil
}
func (f *fakeFingerprintRepo) Upsert(_ context.Context, userID int64, fp string) error {
	k := fmt.Sprintf("%d|%s", userID, fp)
	if !f.prints[k] {
		f.prints[k] = true
		f.counts[userID]++
	}
	return nil
}

// ─── Auth service builder ────────────────────────────────────────────────────

func buildTestAuthService(userRepo *fakeUserRepo, passkeyRepo *fakePasskeyRepo, twofa TwoFactorService) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		passkeyRepo:     passkeyRepo,
		fingerprintRepo: newFakeFingerprintRepo(),
		jwtManager:      newTestJWT(),
		twofaService:    twofa,
		bcryptCost:      4,
	}
}

func hashPasswordFast(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	return string(h), err
}

func newTestJWT() *pkgauth.JWTManager {
	return pkgauth.NewJWTManager("test-secret-key-for-unit-tests-only", 15*time.Minute, 168*time.Hour)
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestLogin_NoTwoFA_IssuesTokens(t *testing.T) {
	hash, _ := hashPasswordFast("password")
	user := &domain.User{ID: 1, Email: "a@b.com", PasswordHash: hash, IsActive: true}

	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), newTwoFAService())

	result, err := svc.Login(context.Background(), &models.LoginRequest{Email: "a@b.com", Password: "password"}, "1.2.3.4", "TestUA")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if result.TwoFARequired {
		t.Fatal("expected no 2FA required")
	}
	if result.Auth == nil || result.Auth.AccessToken == "" {
		t.Fatal("expected access token")
	}
}

func TestLogin_WithVerifiedPhone_Requires2FA(t *testing.T) {
	hash, _ := hashPasswordFast("password")
	phone := "+77001234500"
	user := &domain.User{
		ID: 2, Email: "b@b.com", PasswordHash: hash, IsActive: true,
		Phone: &phone, PhoneVerified: true,
	}

	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), newTwoFAService())

	result, err := svc.Login(context.Background(), &models.LoginRequest{Email: "b@b.com", Password: "password"}, "1.2.3.4", "TestUA")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !result.TwoFARequired {
		t.Fatal("expected 2FA required")
	}
	if result.TempToken == "" {
		t.Fatal("expected temp token")
	}
	found := false
	for _, m := range result.Methods {
		if m == "telegram" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'telegram' in methods, got %v", result.Methods)
	}
}

func TestLogin_PasskeyOnly_Requires2FA(t *testing.T) {
	hash, _ := hashPasswordFast("password")
	user := &domain.User{ID: 3, Email: "c@b.com", PasswordHash: hash, IsActive: true}

	passkeyRepo := newFakePasskeyRepo()
	passkeyRepo.counts[3] = 1 // has a passkey but no phone

	svc := buildTestAuthService(newFakeUserRepo(user), passkeyRepo, newTwoFAService())

	result, err := svc.Login(context.Background(), &models.LoginRequest{Email: "c@b.com", Password: "password"}, "1.2.3.4", "TestUA")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !result.TwoFARequired {
		t.Fatal("expected 2FA required for passkey-only user")
	}
	found := false
	for _, m := range result.Methods {
		if m == "passkey" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'passkey' in methods, got %v", result.Methods)
	}
}

func TestLogin_BadPassword(t *testing.T) {
	hash, _ := hashPasswordFast("correct")
	user := &domain.User{ID: 4, Email: "d@b.com", PasswordHash: hash, IsActive: true}
	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), newTwoFAService())

	_, err := svc.Login(context.Background(), &models.LoginRequest{Email: "d@b.com", Password: "wrong"}, "1.2.3.4", "UA")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLogin_DeactivatedUser(t *testing.T) {
	hash, _ := hashPasswordFast("password")
	user := &domain.User{ID: 5, Email: "e@b.com", PasswordHash: hash, IsActive: false}
	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), newTwoFAService())

	_, err := svc.Login(context.Background(), &models.LoginRequest{Email: "e@b.com", Password: "password"}, "1.2.3.4", "UA")
	if err == nil {
		t.Fatal("expected error for deactivated user")
	}
}

func TestChangePassword_NoTwoFA_Blocked(t *testing.T) {
	hash, _ := hashPasswordFast("old")
	user := &domain.User{ID: 6, Email: "f@b.com", PasswordHash: hash, IsActive: true, AuthMethod: domain.AuthMethodRegular}

	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), newTwoFAService())

	err := svc.ChangePassword(context.Background(), 6, "old", "newpass123", "some_token")
	if err != ErrNoTwoFAMethod {
		t.Fatalf("expected ErrNoTwoFAMethod, got: %v", err)
	}
}

func TestChangePassword_WithValidActionToken(t *testing.T) {
	hash, _ := hashPasswordFast("old")
	phone := "+77001234510"
	user := &domain.User{
		ID: 7, Email: "g@b.com", PasswordHash: hash, IsActive: true,
		AuthMethod: domain.AuthMethodRegular,
		Phone:      &phone, PhoneVerified: true,
	}

	twofa := newTwoFAService()
	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), twofa)

	// Issue a valid action token.
	token, err := twofa.IssueActionToken(context.Background(), 7, domain.ActionChangePassword)
	if err != nil {
		t.Fatalf("IssueActionToken failed: %v", err)
	}

	err = svc.ChangePassword(context.Background(), 7, "old", "newpass123", token)
	if err != nil {
		t.Fatalf("ChangePassword with valid token failed: %v", err)
	}
}

func TestRemovePhone_NoPasskey_Blocked(t *testing.T) {
	phone := "+77001230001"
	user := &domain.User{ID: 8, Email: "h@b.com", Phone: &phone, PhoneVerified: true}

	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), newTwoFAService())

	err := svc.RemovePhone(context.Background(), 8)
	if err == nil {
		t.Fatal("expected error when removing phone without passkey")
	}
}

func TestRemovePhone_WithPasskey_OK(t *testing.T) {
	phone := "+77001230002"
	user := &domain.User{ID: 9, Email: "i@b.com", Phone: &phone, PhoneVerified: true}

	passkeyRepo := newFakePasskeyRepo()
	passkeyRepo.counts[9] = 1

	svc := buildTestAuthService(newFakeUserRepo(user), passkeyRepo, newTwoFAService())

	if err := svc.RemovePhone(context.Background(), 9); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestForgotPasswordSend_AlwaysReturnsNil(t *testing.T) {
	// Even for unknown emails, ForgotPasswordSend must return nil (no enumeration).
	svc := buildTestAuthService(newFakeUserRepo(), newFakePasskeyRepo(), newTwoFAService())

	err := svc.ForgotPasswordSend(context.Background(), "unknown@example.com")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestCompleteTwoFALogin_Success(t *testing.T) {
	hash, _ := hashPasswordFast("password")
	user := &domain.User{ID: 10, Email: "j@b.com", PasswordHash: hash, IsActive: true}

	twofa := newTwoFAService()
	svc := buildTestAuthService(newFakeUserRepo(user), newFakePasskeyRepo(), twofa)

	// Store a pending session.
	token, err := twofa.StorePending2FA(context.Background(), 10, "fp123", false)
	if err != nil {
		t.Fatalf("StorePending2FA failed: %v", err)
	}

	authResp, err := svc.CompleteTwoFALogin(context.Background(), token)
	if err != nil {
		t.Fatalf("CompleteTwoFALogin failed: %v", err)
	}
	if authResp.Auth.AccessToken == "" {
		t.Fatal("expected access token")
	}
}
