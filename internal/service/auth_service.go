package service

import (
	"context"
	"fmt"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/email"
)

type AuthService struct {
	userRepo     *repository.UserRepository
	walletRepo   *repository.WalletRepository
	jwtManager   *auth.JWTManager
	emailService *email.EmailService
	bcryptCost   int
	logger       *logger.Logger
}

func NewAuthService(
	userRepo *repository.UserRepository,
	walletRepo *repository.WalletRepository,
	jwtManager *auth.JWTManager,
	emailService *email.EmailService,
	bcryptCost int,
	logger *logger.Logger,
) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		walletRepo:   walletRepo,
		jwtManager:   jwtManager,
		emailService: emailService,
		bcryptCost:   bcryptCost,
		logger:       logger,
	}
}

func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
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
				Balance:    0,
				Locked:     0,
			}
			s.walletRepo.Create(ctx, wallet)
		}
	}

	go func() {
		s.emailService.SendWelcomeEmail(user.Email, user.FirstName)
	}()

	return s.generateTokens(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("Get by email err:", err)
		return nil, fmt.Errorf("invalid credentials")
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("account is deactivated")
	}

	return s.generateTokensWithRememberMe(ctx, user, req.RememberMe)
}

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

func (s *AuthService) generateTokens(ctx context.Context, user *domain.User) (*models.AuthResponse, error) {
	return s.generateTokensWithRememberMe(ctx, user, false)
}

func (s *AuthService) generateTokensWithRememberMe(ctx context.Context, user *domain.User, rememberMe bool) (*models.AuthResponse, error) {
	var accessToken string
	var err error

	if rememberMe {
		// Extended access token for 30 days when remember me is enabled
		accessToken, err = s.jwtManager.GenerateAccessTokenWithExpiry(user.ID, user.Email, string(user.Role), 30*24*time.Hour)
	} else {
		// Default access token expiry (configured in JWT manager)
		accessToken, err = s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Extend refresh token expiry when remember me is enabled
	var sessionExpiry time.Duration
	if rememberMe {
		sessionExpiry = 30 * 24 * time.Hour // 30 days
	} else {
		sessionExpiry = 168 * time.Hour // 7 days (default)
	}

	session := &domain.UserSession{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(sessionExpiry),
	}

	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	user.PasswordHash = ""

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}
