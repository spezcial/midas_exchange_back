package service

import (
	"context"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
)

type UserService struct {
	userRepo   *repository.UserRepository
	walletRepo *repository.WalletRepository
}

func NewUserService(userRepo *repository.UserRepository, walletRepo *repository.WalletRepository) *UserService {
	return &UserService{
		userRepo:   userRepo,
		walletRepo: walletRepo,
	}
}

type UserWithWallets struct {
	User    *domain.User                `json:"user"`
	Wallets []domain.WalletWithCurrency `json:"wallets"`
}

func (s *UserService) GetUser(ctx context.Context, userID int64) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *UserService) GetUserWithWallets(ctx context.Context, userID int64) (*UserWithWallets, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""

	wallets, err := s.walletRepo.GetUserWallets(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &UserWithWallets{
		User:    user,
		Wallets: wallets,
	}, nil
}

func (s *UserService) ListUsers(ctx context.Context, limit, offset int, email string) ([]domain.User, int64, error) {
	users, err := s.userRepo.List(ctx, limit, offset, email)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.userRepo.Count(ctx, email)
	if err != nil {
		return nil, 0, err
	}

	for i := range users {
		users[i].PasswordHash = ""
	}

	return users, total, nil
}

func (s *UserService) UpdateUser(ctx context.Context, user *domain.User) error {
	return s.userRepo.Update(ctx, user)
}
