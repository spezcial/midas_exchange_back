package repository

import (
	"context"

	"github.com/caspianex/exchange-backend/internal/domain"
)

// UserRepo is the interface that auth and related services depend on.
// Keeping it separate from the concrete UserRepository allows services to be
// unit-tested with in-memory fakes.
type UserRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByEmailAnyRole(ctx context.Context, email string) (*domain.User, error)
	GetByPhone(ctx context.Context, phone string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	UpdatePhone(ctx context.Context, userID int64, phone *string, verified bool) error
	UpdatePassword(ctx context.Context, userID int64, passwordHash string) error
	CreateSession(ctx context.Context, session *domain.UserSession) error
	GetSessionByToken(ctx context.Context, token string) (*domain.UserSession, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteAllSessionsByUserID(ctx context.Context, userID int64) error
}

// WalletRepo is the minimal wallet interface used by AuthService.
type WalletRepo interface {
	GetAllCurrencies(ctx context.Context) ([]domain.Currency, error)
	Create(ctx context.Context, wallet *domain.Wallet) error
	GetCurrencyByCode(ctx context.Context, code string) (*domain.Currency, error)
}
