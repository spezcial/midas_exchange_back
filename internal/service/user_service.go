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

type UserProfile struct {
	ID         int64   `json:"id"`
	Email      string  `json:"email"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	MiddleName *string `json:"middle_name"`
	Phone      *string `json:"phone"`
	KycLevel   int     `json:"kyc_level"`
}

func (s *UserService) GetProfile(ctx context.Context, userID int64) (*UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &UserProfile{
		ID:         user.ID,
		Email:      user.Email,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		MiddleName: user.MiddleName,
		Phone:      user.Phone,
		KycLevel:   user.KycLevel,
	}, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID int64, firstName, lastName string, middleName, phone *string, kycLevel int) (*UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.FirstName = firstName
	user.LastName = lastName
	user.MiddleName = middleName
	user.Phone = phone
	user.KycLevel = kycLevel

	if err := s.userRepo.UpdateProfile(ctx, user); err != nil {
		return nil, err
	}

	return &UserProfile{
		ID:         user.ID,
		Email:      user.Email,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		MiddleName: user.MiddleName,
		Phone:      user.Phone,
		KycLevel:   user.KycLevel,
	}, nil
}
