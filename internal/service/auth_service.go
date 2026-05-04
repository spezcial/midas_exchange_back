package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/email"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

// TwoFAMethod describes which 2FA methods are available for a user at login.
type TwoFAMethod struct {
	Telegram bool
	Passkey  bool
}

// AuthService handles registration, login, token management, and password lifecycle.
type AuthService struct {
	userRepo        repository.UserRepo
	walletRepo      repository.WalletRepo
	passkeyRepo     repository.PasskeyRepository
	fingerprintRepo repository.FingerprintRepository
	jwtManager      *auth.JWTManager
	emailService    *email.EmailService
	twofaService    TwoFactorService
	cgService       *CryptoGateService // optional; nil if not configured
	bcryptCost      int
	logger          *logger.Logger
}

func NewAuthService(
	userRepo repository.UserRepo,
	walletRepo repository.WalletRepo,
	passkeyRepo repository.PasskeyRepository,
	fingerprintRepo repository.FingerprintRepository,
	jwtManager *auth.JWTManager,
	emailService *email.EmailService,
	twofaService TwoFactorService,
	bcryptCost int,
	log *logger.Logger,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		walletRepo:      walletRepo,
		passkeyRepo:     passkeyRepo,
		fingerprintRepo: fingerprintRepo,
		jwtManager:      jwtManager,
		emailService:    emailService,
		twofaService:    twofaService,
		bcryptCost:      bcryptCost,
		logger:          log,
	}
}

func (s *AuthService) SetCryptoGateService(cg *CryptoGateService) {
	s.cgService = cg
}

// --- Registration ------------------------------------------------------------

func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("email already registered")
	}

	hashedPassword, err := auth.HashPassword(req.Password, s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         domain.UserRoleClient,
		IsActive:     true,
		IsVerified:   false,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	currencies, err := s.walletRepo.GetAllCurrencies(ctx)
	if err == nil {
		for _, currency := range currencies {
			wallet := &domain.Wallet{
				UserID:     user.ID,
				CurrencyID: currency.ID,
			}
			if err := s.walletRepo.Create(ctx, wallet); err != nil {
				s.logger.Error("failed to create wallet during registration", "user_id", user.ID, "currency_id", currency.ID, "error", err)
			}
		}
	}

	if s.cgService != nil {
		userID := user.ID
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			for code, chain := range domain.CryptoGateCurrencies {
				currency, err := s.walletRepo.GetCurrencyByCode(bgCtx, code)
				if err != nil {
					continue
				}
				if _, err := s.cgService.GetOrCreateDepositAddress(bgCtx, userID, code, int64(currency.ID)); err != nil {
					s.logger.Warn("failed to create deposit address at registration",
						"user_id", userID, "currency", code, "chain", chain.Chain, "error", err)
				}
			}
		}()
	}

	go func() {
		s.emailService.SendWelcomeEmail(user.Email, user.FirstName)
	}()

	return s.generateTokens(ctx, user)
}

// --- Login -------------------------------------------------------------------

// LoginResult is returned by Login. If TwoFARequired is true, the caller should
// present the TempToken to one of the 2FA completion endpoints.
type LoginResult struct {
	TwoFARequired    bool
	TempToken        string               // set when TwoFARequired == true
	Methods          []string             // available 2FA methods when TwoFARequired == true
	Auth             *models.AuthResponse // set when TwoFARequired == false
	TwoFactorEnabled bool                 // true if the user has a verified phone
	PasskeyEnabled   bool                 // true if the user has at least one passkey
}

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest, clientIP, userAgent string) (*LoginResult, error) {
	user, err := s.userRepo.GetByEmailAnyRole(ctx, req.Email)
	if err != nil {
		s.logger.Error("Get by email err", "error", err)
		return nil, fmt.Errorf("invalid credentials")
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("account is deactivated")
	}

	methods := s.available2FAMethods(ctx, user)

	// 2FA is required when the user has at least one method configured.
	if methods.Telegram || methods.Passkey {
		fingerprint := computeFingerprint(clientIP, userAgent)
		tempToken, err := s.twofaService.StorePending2FA(ctx, user.ID, fingerprint, req.RememberMe)
		if err != nil {
			return nil, fmt.Errorf("failed to initiate 2FA: %w", err)
		}

		return &LoginResult{
			TwoFARequired:    true,
			TempToken:        tempToken,
			Methods:          availableMethodList(methods),
			TwoFactorEnabled: methods.Telegram,
			PasskeyEnabled:   methods.Passkey,
		}, nil
	}

	// No 2FA configured — issue tokens directly.
	authResp, err := s.generateTokensWithRememberMe(ctx, user, req.RememberMe)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Auth:             authResp,
		TwoFactorEnabled: methods.Telegram,
		PasskeyEnabled:   methods.Passkey,
	}, nil
}

// CompleteTwoFALogin is called after the user successfully verifies a Telegram OTP
// during the login flow. It validates the pending session, records the fingerprint,
// sends a new-device alert if needed, and issues JWT tokens.
func (s *AuthService) CompleteTwoFALogin(ctx context.Context, tempToken string) (*LoginResult, error) {
	pending, err := s.twofaService.ConsumePending2FA(ctx, tempToken)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, pending.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("account is deactivated")
	}

	s.handleFingerprint(ctx, user, pending.Fingerprint)

	authResp, err := s.generateTokensWithRememberMe(ctx, user, pending.RememberMe)
	if err != nil {
		return nil, err
	}

	methods := s.available2FAMethods(ctx, user)
	return &LoginResult{
		Auth:             authResp,
		TwoFactorEnabled: methods.Telegram,
		PasskeyEnabled:   methods.Passkey,
	}, nil
}

// --- Token refresh & logout --------------------------------------------------

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	session, err := s.userRepo.GetSessionByToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if err := s.userRepo.DeleteSession(ctx, refreshToken); err != nil {
		return nil, err
	}

	return s.generateTokens(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.userRepo.DeleteSession(ctx, refreshToken)
}

// --- Password management -----------------------------------------------------

func (s *AuthService) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword, actionToken string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if user.AuthMethod != domain.AuthMethodRegular {
		return fmt.Errorf("password change is not available for OAuth accounts")
	}

	// Ensure the user has at least one 2FA method — password change without 2FA
	// would remove the last security factor.
	methods := s.available2FAMethods(ctx, user)
	if !methods.Telegram && !methods.Passkey {
		return ErrNoTwoFAMethod
	}

	// Validate the action token (consumed here — single use).
	ownerID, err := s.twofaService.ConsumeActionToken(ctx, actionToken, domain.ActionChangePassword)
	if err != nil {
		return fmt.Errorf("invalid action token: %w", err)
	}
	if ownerID != userID {
		return fmt.Errorf("action token does not belong to this user")
	}

	if !auth.CheckPassword(currentPassword, user.PasswordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	newHash, err := auth.HashPassword(newPassword, s.bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, newHash); err != nil {
		return err
	}

	// Invalidate all existing sessions — stolen sessions should not survive a
	// password change.
	if err := s.userRepo.DeleteAllSessionsByUserID(ctx, userID); err != nil {
		s.logger.Error("failed to purge sessions after password change", "user_id", userID, "error", err)
	}

	return nil
}

// --- Forgot-password flow ----------------------------------------------------

// ForgotPasswordSend looks up the user by email and sends an OTP to their
// registered phone. To prevent user enumeration, the caller should ALWAYS return
// the same generic response to the client regardless of whether this method
// returns an error.
func (s *AuthService) ForgotPasswordSend(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		// Swallow — do not reveal whether the email exists.
		return nil
	}

	if !user.PhoneVerified || user.Phone == nil {
		// No verified phone — can't send OTP. Swallow silently.
		return nil
	}

	// Best-effort; errors are logged but not surfaced to the caller.
	if err := s.twofaService.SendOTP(ctx, user.ID, *user.Phone, domain.OTPPurposeResetPassword); err != nil {
		s.logger.Warn("failed to send password reset OTP", "user_id", user.ID, "error", err)
	}

	return nil
}

// ForgotPasswordVerify checks the OTP and, if valid, issues a single-use
// password-reset action token.
func (s *AuthService) ForgotPasswordVerify(ctx context.Context, phone, code string) (string, error) {
	if err := s.twofaService.VerifyOTP(ctx, phone, code, domain.OTPPurposeResetPassword); err != nil {
		return "", err
	}

	// Resolve user from phone.
	user, err := s.userRepo.GetByPhone(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("account not found")
	}

	token, err := s.twofaService.IssueActionToken(ctx, user.ID, domain.ActionResetPassword)
	if err != nil {
		return "", fmt.Errorf("failed to issue reset token: %w", err)
	}
	return token, nil
}

// ForgotPasswordReset validates the reset token and sets a new password.
func (s *AuthService) ForgotPasswordReset(ctx context.Context, resetToken, newPassword string) error {
	userID, err := s.twofaService.ConsumeActionToken(ctx, resetToken, domain.ActionResetPassword)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	newHash, err := auth.HashPassword(newPassword, s.bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, newHash); err != nil {
		return err
	}

	// Purge all sessions so the account is clean after a password reset.
	if err := s.userRepo.DeleteAllSessionsByUserID(ctx, userID); err != nil {
		s.logger.Error("failed to purge sessions after password reset", "user_id", userID, "error", err)
	}

	return nil
}

// --- Phone management --------------------------------------------------------

// SetPhone validates uniqueness, sends an OTP, but does NOT save the phone yet.
// The phone is saved only after VerifyPhone is called successfully.
func (s *AuthService) SetPhoneSendOTP(ctx context.Context, userID int64, phone string) error {
	phone = normalizePhone(phone)

	// Check uniqueness — another account must not have this phone.
	existing, err := s.userRepo.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check phone: %w", err)
	}
	if existing != nil && existing.ID != userID {
		return fmt.Errorf("phone number is already in use")
	}

	return s.twofaService.SendOTP(ctx, userID, phone, domain.OTPPurposePhoneVerify)
}

// VerifyAndSavePhone verifies the OTP and saves the phone number as verified.
func (s *AuthService) VerifyAndSavePhone(ctx context.Context, userID int64, phone, code string) error {
	phone = normalizePhone(phone)

	if err := s.twofaService.VerifyOTP(ctx, phone, code, domain.OTPPurposePhoneVerify); err != nil {
		return err
	}

	if err := s.userRepo.UpdatePhone(ctx, userID, &phone, true); err != nil {
		return fmt.Errorf("failed to save phone: %w", err)
	}
	return nil
}

// GetUserByID returns the user with the given ID.
func (s *AuthService) GetUserByID(ctx context.Context, userID int64) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

// MeResult is the response for GET /auth/me.
type MeResult struct {
	User             *domain.User
	TwoFactorEnabled bool
	PasskeyEnabled   bool
}

// GetMe returns the authenticated user plus their current 2FA/passkey status.
func (s *AuthService) GetMe(ctx context.Context, userID int64) (*MeResult, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	methods := s.available2FAMethods(ctx, user)
	return &MeResult{
		User:             user,
		TwoFactorEnabled: methods.Telegram,
		PasskeyEnabled:   methods.Passkey,
	}, nil
}

// GetVerifiedPhone returns the user's verified phone number.
// Returns an error if no phone is set or it is not yet verified.
func (s *AuthService) GetVerifiedPhone(ctx context.Context, userID int64) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}
	if user.Phone == nil || !user.PhoneVerified {
		return "", fmt.Errorf("no verified phone number on file")
	}
	return *user.Phone, nil
}

// SendLoginTelegramOTP sends a Telegram OTP for an active pending-2FA session.
// Called when the user explicitly chooses the Telegram method on the frontend.
func (s *AuthService) SendLoginTelegramOTP(ctx context.Context, tempToken string) error {
	pending, err := s.twofaService.PeekPending2FA(ctx, tempToken)
	if err != nil {
		return err
	}
	user, err := s.userRepo.GetByID(ctx, pending.UserID)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if user.Phone == nil || !user.PhoneVerified {
		return fmt.Errorf("no verified phone number on file")
	}
	return s.twofaService.SendOTP(ctx, user.ID, *user.Phone, domain.OTPPurposeLogin)
}

// PeekPending2FAPhone resolves the user's phone from a pending 2FA session
// without consuming the temp token, so that the OTP can be verified before
// the session is committed.
func (s *AuthService) PeekPending2FAPhone(ctx context.Context, tempToken string) (int64, string, error) {
	pending, err := s.twofaService.PeekPending2FA(ctx, tempToken)
	if err != nil {
		return 0, "", err
	}

	user, err := s.userRepo.GetByID(ctx, pending.UserID)
	if err != nil {
		return 0, "", fmt.Errorf("user not found")
	}
	if user.Phone == nil || !user.PhoneVerified {
		return 0, "", fmt.Errorf("no verified phone number on file")
	}
	return pending.UserID, *user.Phone, nil
}

// RemovePhone deletes the phone from the user's account.
// Only allowed if the user has at least one passkey registered.
func (s *AuthService) RemovePhone(ctx context.Context, userID int64) error {
	count, err := s.passkeyRepo.CountByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check passkeys: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("cannot remove phone: you must have at least one passkey registered as a backup")
	}

	return s.userRepo.UpdatePhone(ctx, userID, nil, false)
}

// --- Internal helpers --------------------------------------------------------

func (s *AuthService) generateTokens(ctx context.Context, user *domain.User) (*models.AuthResponse, error) {
	return s.generateTokensWithRememberMe(ctx, user, false)
}

func (s *AuthService) generateTokensWithRememberMe(ctx context.Context, user *domain.User, rememberMe bool) (*models.AuthResponse, error) {
	var accessToken string
	var err error

	if rememberMe {
		accessToken, err = s.jwtManager.GenerateAccessTokenWithExpiry(user.ID, user.Email, string(user.Role), 30*24*time.Hour)
	} else {
		accessToken, err = s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	var sessionExpiry time.Duration
	if rememberMe {
		sessionExpiry = 30 * 24 * time.Hour
	} else {
		sessionExpiry = 168 * time.Hour
	}

	session := &domain.UserSession{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(sessionExpiry),
	}
	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	userResponse := *user
	userResponse.PasswordHash = ""

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &userResponse,
	}, nil
}

// available2FAMethods determines which 2FA methods are active for the user.
func (s *AuthService) available2FAMethods(ctx context.Context, user *domain.User) TwoFAMethod {
	var m TwoFAMethod

	if user.PhoneVerified && user.Phone != nil {
		m.Telegram = true
	}

	count, err := s.passkeyRepo.CountByUserID(ctx, user.ID)
	if err != nil {
		// Log but treat as "no passkey" — this is fail-open for UX (user can still
		// use Telegram if available), but means a passkey-only user would skip 2FA
		// on a transient DB error. Acceptable given the low probability; we log so
		// ops can detect a persistent failure.
		s.logger.Error("failed to count passkeys for 2FA check", "user_id", user.ID, "error", err)
	} else if count > 0 {
		m.Passkey = true
	}

	return m
}

// handleFingerprint checks whether the login fingerprint is new for this user.
// If it is, a security alert is sent via Telegram (fire-and-forget).
// The fingerprint is always upserted so it is recorded for future logins.
func (s *AuthService) handleFingerprint(ctx context.Context, user *domain.User, fingerprint string) {
	if fingerprint == "" {
		return
	}

	// Count existing fingerprints before upsert to decide whether to notify.
	count, err := s.fingerprintRepo.CountByUserID(ctx, user.ID)
	if err != nil {
		s.logger.Warn("failed to count fingerprints", "user_id", user.ID, "error", err)
		// Continue — don't block the login over a non-critical failure.
	}

	exists, err := s.fingerprintRepo.Exists(ctx, user.ID, fingerprint)
	if err != nil {
		s.logger.Warn("failed to check fingerprint", "user_id", user.ID, "error", err)
	}

	// Upsert regardless of result.
	if err := s.fingerprintRepo.Upsert(ctx, user.ID, fingerprint); err != nil {
		s.logger.Warn("failed to upsert fingerprint", "user_id", user.ID, "error", err)
	}

	// Notify only if: this fingerprint is new AND the user already had at least
	// one prior fingerprint (first ever login should not trigger an alert).
	if !exists && count > 0 && user.Phone != nil && user.PhoneVerified {
		s.twofaService.SendSecurityAlert(ctx, *user.Phone,
			"⚠️ New login detected on your CaspianEx account from a new device or browser. "+
				"If this was not you, please change your password immediately.")
	}
}

// computeFingerprint derives a stable, privacy-preserving fingerprint from the
// client IP and User-Agent string.
func computeFingerprint(clientIP, userAgent string) string {
	raw := strings.Join([]string{clientIP, userAgent}, "|")
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// normalizePhone strips whitespace from a phone number.
// Full E.164 normalisation (e.g. via libphonenumber) should be added if
// international numbers need to be supported.
func normalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

func availableMethodList(m TwoFAMethod) []string {
	var out []string
	if m.Telegram {
		out = append(out, "telegram")
	}
	if m.Passkey {
		out = append(out, "passkey")
	}
	return out
}
