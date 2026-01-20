package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/config"
	"github.com/caspianex/exchange-backend/pkg/email"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type OAuthService struct {
	oauthRepo    *repository.OAuthRepository
	userRepo     *repository.UserRepository
	walletRepo   *repository.WalletRepository
	jwtManager   *auth.JWTManager
	emailService *email.EmailService
	googleConfig *oauth2.Config
	stateExpiry  time.Duration
	logger       *logger.Logger
}

func NewOAuthService(
	oauthRepo *repository.OAuthRepository,
	userRepo *repository.UserRepository,
	walletRepo *repository.WalletRepository,
	jwtManager *auth.JWTManager,
	emailService *email.EmailService,
	cfg *config.OAuthConfig,
	logger *logger.Logger,
) *OAuthService {
	googleConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &OAuthService{
		oauthRepo:    oauthRepo,
		userRepo:     userRepo,
		walletRepo:   walletRepo,
		jwtManager:   jwtManager,
		emailService: emailService,
		googleConfig: googleConfig,
		stateExpiry:  cfg.StateExpiry,
		logger:       logger,
	}
}

func (s *OAuthService) GetGoogleAuthURL(ctx context.Context) (*models.GoogleAuthURLResponse, error) {
	// Generate a random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Store state in database
	oauthState := &domain.OAuthState{
		State:     state,
		ExpiresAt: time.Now().Add(s.stateExpiry),
	}

	if err := s.oauthRepo.CreateOAuthState(ctx, oauthState); err != nil {
		return nil, fmt.Errorf("failed to store state: %w", err)
	}
	// Generate Google OAuth URL
	authURL := s.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	return &models.GoogleAuthURLResponse{
		AuthURL: authURL,
		State:   state,
	}, nil
}

func (s *OAuthService) HandleGoogleCallback(ctx context.Context, req *models.GoogleCallbackRequest) (*models.AuthResponse, error) {
	// Validate state
	_, err := s.oauthRepo.GetAndDeleteOAuthState(ctx, req.State)
	if err != nil {
		s.logger.Error("Invalid OAuth state", "error", err)
		return nil, fmt.Errorf("invalid or expired state")
	}

	// Exchange code for token
	token, err := s.googleConfig.Exchange(ctx, req.Code)
	if err != nil {
		s.logger.Error("Failed to exchange code", "error", err)
		return nil, fmt.Errorf("failed to exchange authorization code")
	}

	// Get user info from Google
	userInfo, err := s.getGoogleUserInfo(ctx, token.AccessToken)
	if err != nil {
		s.logger.Error("Failed to get Google user info", "error", err)
		return nil, fmt.Errorf("failed to get user information from Google")
	}

	// Check if email is verified
	if !userInfo.VerifiedEmail {
		return nil, fmt.Errorf("Google email is not verified")
	}

	// Try to find user by Google ID
	user, err := s.userRepo.GetByGoogleID(ctx, userInfo.ID)
	if err != nil {
		s.logger.Error("Failed to get user by Google ID", "error", err)
		return nil, fmt.Errorf("database error")
	}

	if user != nil {
		// Existing user - log them in
		if !user.IsActive {
			return nil, fmt.Errorf("account is deactivated")
		}
		return s.generateTokens(ctx, user)
	}

	// Check if email already exists with regular auth
	existingUser, _ := s.userRepo.GetByEmail(ctx, userInfo.Email)
	if existingUser != nil {
		return nil, fmt.Errorf("email already registered. Please login with password")
	}

	// Create new user
	googleID := userInfo.ID
	user = &domain.User{
		Email:      userInfo.Email,
		FirstName:  userInfo.GivenName,
		LastName:   userInfo.FamilyName,
		Role:       domain.UserRoleClient,
		IsActive:   true,
		IsVerified: true, // Google verified the email
		AuthMethod: domain.AuthMethodGoogle,
		GoogleID:   &googleID,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create Google user", "error", err)
		return nil, fmt.Errorf("failed to create user")
	}

	// Create wallets for the new user
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

	// Send welcome email asynchronously
	go func() {
		s.emailService.SendWelcomeEmail(user.Email, user.FirstName)
	}()

	return s.generateTokens(ctx, user)
}

func (s *OAuthService) getGoogleUserInfo(ctx context.Context, accessToken string) (*domain.GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google API error: %s", string(body))
	}

	var userInfo domain.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func (s *OAuthService) generateTokens(ctx context.Context, user *domain.User) (*models.AuthResponse, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	session := &domain.UserSession{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(168 * time.Hour), // 7 days
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

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *OAuthService) CleanupExpiredStates(ctx context.Context) (int64, error) {
	return s.oauthRepo.CleanupExpiredStates(ctx)
}
