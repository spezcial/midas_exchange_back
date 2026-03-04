package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/auth"
)

type UserService struct {
	userRepo   *repository.UserRepository
	walletRepo *repository.WalletRepository
	bcryptCost int
}

func NewUserService(userRepo *repository.UserRepository, walletRepo *repository.WalletRepository, bcryptCost int) *UserService {
	return &UserService{
		userRepo:   userRepo,
		walletRepo: walletRepo,
		bcryptCost: bcryptCost,
	}
}

type CreateStaffInput struct {
	Email     string          `json:"email"`
	FirstName string          `json:"first_name"`
	LastName  string          `json:"last_name"`
	Role      domain.UserRole `json:"role"`
}

type UpdateStaffInput struct {
	FirstName string          `json:"first_name"`
	LastName  string          `json:"last_name"`
	Role      domain.UserRole `json:"role"`
	IsActive  bool            `json:"is_active"`
}

type StaffUser struct {
	ID        int64           `json:"id"`
	Email     string          `json:"email"`
	FirstName string          `json:"first_name"`
	LastName  string          `json:"last_name"`
	Role      domain.UserRole `json:"role"`
	IsActive  bool            `json:"is_active"`
}

func staffFromUser(u *domain.User) *StaffUser {
	return &StaffUser{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.Role,
		IsActive:  u.IsActive,
	}
}

func generateTempPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, length)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}
	return string(password), nil
}

func (s *UserService) CreateStaff(ctx context.Context, callerID int64, input CreateStaffInput) (*StaffUser, string, error) {
	if !input.Role.IsStaffRole() || !input.Role.IsValidRole() {
		return nil, "", fmt.Errorf("invalid staff role: %s", input.Role)
	}

	tempPassword, err := generateTempPassword(12)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate password: %w", err)
	}

	hashedPassword, err := auth.HashPassword(tempPassword, s.bcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		Email:        input.Email,
		PasswordHash: hashedPassword,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Role:         input.Role,
		IsActive:     true,
		IsVerified:   true,
		AuthMethod:   domain.AuthMethodRegular,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, "", fmt.Errorf("failed to create staff: %w", err)
	}

	return staffFromUser(user), tempPassword, nil
}

func (s *UserService) ListStaff(ctx context.Context, limit, offset int, email string) ([]StaffUser, int64, error) {
	users, err := s.userRepo.ListStaff(ctx, limit, offset, email)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.userRepo.CountStaff(ctx, email)
	if err != nil {
		return nil, 0, err
	}

	staff := make([]StaffUser, len(users))
	for i := range users {
		staff[i] = *staffFromUser(&users[i])
	}

	return staff, total, nil
}

func (s *UserService) UpdateStaff(ctx context.Context, callerID, staffID int64, input UpdateStaffInput) (*StaffUser, error) {
	if !input.Role.IsStaffRole() || !input.Role.IsValidRole() {
		return nil, fmt.Errorf("invalid staff role: %s", input.Role)
	}

	user, err := s.userRepo.GetByID(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("staff not found")
	}

	if !domain.UserRole(user.Role).IsStaffRole() {
		return nil, fmt.Errorf("user is not a staff member")
	}

	user.FirstName = input.FirstName
	user.LastName = input.LastName
	user.Role = input.Role
	user.IsActive = input.IsActive

	if err := s.userRepo.UpdateStaff(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update staff: %w", err)
	}

	return staffFromUser(user), nil
}

func (s *UserService) DeactivateStaff(ctx context.Context, callerID, staffID int64) error {
	if callerID == staffID {
		return fmt.Errorf("cannot deactivate your own account")
	}

	user, err := s.userRepo.GetByID(ctx, staffID)
	if err != nil {
		return fmt.Errorf("staff not found")
	}

	if !domain.UserRole(user.Role).IsStaffRole() {
		return fmt.Errorf("user is not a staff member")
	}

	user.IsActive = false

	return s.userRepo.UpdateStaff(ctx, user)
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
