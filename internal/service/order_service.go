package service

import (
	"context"
	"fmt"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/email"
	"github.com/google/uuid"
)

type CurrencyExchangeService struct {
	exchangeRepo *repository.CurrencyExchangeRepository
	walletRepo   *repository.WalletRepository
	userRepo     *repository.UserRepository
	emailService *email.EmailService
}

func NewCurrencyExchangeService(
	exchangeRepo *repository.CurrencyExchangeRepository,
	walletRepo *repository.WalletRepository,
	userRepo *repository.UserRepository,
	emailService *email.EmailService,
) *CurrencyExchangeService {
	return &CurrencyExchangeService{
		exchangeRepo: exchangeRepo,
		walletRepo:   walletRepo,
		userRepo:     userRepo,
		emailService: emailService,
	}
}

func (s *CurrencyExchangeService) CreateExchange(ctx context.Context, userID int64, req *models.CreateExchangeRequest) (*domain.CurrencyExchange, error) {
	// Get currency information
	fromCurrency, err := s.walletRepo.GetCurrencyByCode(ctx, req.FromCurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("from currency not found: %w", err)
	}

	toCurrency, err := s.walletRepo.GetCurrencyByCode(ctx, req.ToCurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("to currency not found: %w", err)
	}

	// Get exchange rate
	rate, err := s.exchangeRepo.GetExchangeRate(ctx, fromCurrency.ID, toCurrency.ID)
	if err != nil {
		return nil, fmt.Errorf("exchange rate not available for this pair: %w", err)
	}

	// Calculate amounts
	toAmount := req.FromAmount * rate.Rate
	toAmountWithFee := toAmount * (100 - rate.Fee) / 100

	// Get user's wallets
	fromWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, userID, fromCurrency.ID)
	if err != nil {
		return nil, fmt.Errorf("from wallet not found: %w", err)
	}

	toWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, userID, toCurrency.ID)
	if err != nil {
		return nil, fmt.Errorf("to wallet not found: %w", err)
	}

	// Check sufficient balance
	if fromWallet.Balance < req.FromAmount {
		return nil, fmt.Errorf("insufficient balance")
	}

	// Perform wallet swap: deduct from fromWallet, credit to toWallet
	newFromBalance := fromWallet.Balance - req.FromAmount
	if err := s.walletRepo.UpdateBalance(ctx, fromWallet.ID, newFromBalance, fromWallet.Locked); err != nil {
		return nil, fmt.Errorf("failed to deduct from wallet: %w", err)
	}

	newToBalance := toWallet.Balance + toAmountWithFee
	if err := s.walletRepo.UpdateBalance(ctx, toWallet.ID, newToBalance, toWallet.Locked); err != nil {
		// Rollback the first update
		s.walletRepo.UpdateBalance(ctx, fromWallet.ID, fromWallet.Balance, fromWallet.Locked)
		return nil, fmt.Errorf("failed to credit to wallet: %w", err)
	}

	// Create exchange record
	exchange := &domain.CurrencyExchange{
		UID:             uuid.New().String(),
		UserID:          userID,
		FromCurrencyID:  fromCurrency.ID,
		ToCurrencyID:    toCurrency.ID,
		FromAmount:      req.FromAmount,
		ToAmount:        toAmount,
		ToAmountWithFee: toAmountWithFee,
		Fee:             rate.Fee,
		ExchangeRate:    rate.Rate,
		Status:          domain.CurrencyExchangeStatusCompleted,
	}

	if err := s.exchangeRepo.Create(ctx, exchange); err != nil {
		// Rollback wallet updates
		s.walletRepo.UpdateBalance(ctx, fromWallet.ID, fromWallet.Balance, fromWallet.Locked)
		s.walletRepo.UpdateBalance(ctx, toWallet.ID, toWallet.Balance, toWallet.Locked)
		return nil, fmt.Errorf("failed to create exchange: %w", err)
	}

	// Send notification email
	user, _ := s.userRepo.GetByID(ctx, userID)
	go s.emailService.SendOrderCreatedEmail(user.Email, user.FirstName, exchange)

	return exchange, nil
}

func (s *CurrencyExchangeService) GetUserExchanges(ctx context.Context, userID int64, limit, offset int) ([]domain.CurrencyExchangeWithCurrencies, error) {
	return s.exchangeRepo.GetUserExchanges(ctx, userID, limit, offset)
}

func (s *CurrencyExchangeService) GetUserExchangesCount(ctx context.Context, userID int64) (int64, error) {
	return s.exchangeRepo.GetUserExchangesCount(ctx, userID)
}

func (s *CurrencyExchangeService) GetUserExchangeByID(ctx context.Context, userID, exchangeID int64) (*domain.CurrencyExchangeWithCurrencies, error) {
	exchange, err := s.exchangeRepo.GetByID(ctx, exchangeID)
	if err != nil {
		return nil, err
	}

	if exchange.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	return exchange, nil
}

func (s *CurrencyExchangeService) GetExchangeByID(ctx context.Context, exchangeID int64) (*domain.CurrencyExchangeWithCurrencies, error) {
	exchange, err := s.exchangeRepo.GetByID(ctx, exchangeID)
	if err != nil {
		return nil, err
	}

	return exchange, nil
}

func (s *CurrencyExchangeService) CancelExchange(ctx context.Context, userID, exchangeID int64) error {
	exchange, err := s.exchangeRepo.GetByID(ctx, exchangeID)
	if err != nil {
		return err
	}

	if exchange.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	if exchange.Status != domain.CurrencyExchangeStatusPending {
		return fmt.Errorf("can only cancel pending exchanges")
	}

	exchange.Status = domain.CurrencyExchangeStatusCanceled
	return s.exchangeRepo.Update(ctx, exchange.ToCurrencyExchange())
}

func (s *CurrencyExchangeService) GetAllExchanges(ctx context.Context, status, email string, limit, offset int) ([]domain.CurrencyExchangeWithCurrencies, error) {
	return s.exchangeRepo.GetAllExchanges(ctx, status, email, limit, offset)
}

func (s *CurrencyExchangeService) GetAllExchangesCount(ctx context.Context, status, email string) (int64, error) {
	return s.exchangeRepo.GetAllExchangesCount(ctx, status, email)
}

func (s *CurrencyExchangeService) GetExchangeRate(ctx context.Context, fromCurrencyId int32, toCurrencyId int32) (*domain.ExchangeRate, error) {
	rate, err := s.exchangeRepo.GetExchangeRate(ctx, fromCurrencyId, toCurrencyId)
	if err != nil {
		return &domain.ExchangeRate{}, err
	}

	return rate, nil
}
