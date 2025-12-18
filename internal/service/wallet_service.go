package service

import (
	"context"
	"fmt"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
)

type WalletService struct {
	walletRepo *repository.WalletRepository
	txRepo     *repository.TransactionRepository
}

func NewWalletService(
	walletRepo *repository.WalletRepository,
	txRepo *repository.TransactionRepository,
) *WalletService {
	return &WalletService{
		walletRepo: walletRepo,
		txRepo:     txRepo,
	}
}

func (s *WalletService) GetUserWallets(ctx context.Context, userID int64) ([]domain.WalletWithCurrency, error) {
	return s.walletRepo.GetUserWallets(ctx, userID)
}

func (s *WalletService) Deposit(ctx context.Context, userID int64, req *models.DepositRequest) (*domain.Transaction, error) {
	currency, err := s.walletRepo.GetCurrencyByCode(ctx, req.CurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("currency not found: %w", err)
	}

	wallet, err := s.walletRepo.GetByUserAndCurrency(ctx, userID, currency.ID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	tx := &domain.Transaction{
		UserID:   userID,
		WalletID: wallet.ID,
		Type:     domain.TransactionTypeDeposit,
		Amount:   req.Amount,
		Fee:      0,
		Status:   domain.TransactionStatusPending,
		TxHash:   req.TxHash,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	newBalance := wallet.Balance + req.Amount
	if err := s.walletRepo.UpdateBalance(ctx, wallet.ID, newBalance, wallet.Locked); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	tx.Status = domain.TransactionStatusCompleted
	if err := s.txRepo.Update(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (s *WalletService) Withdraw(ctx context.Context, userID int64, req *models.WithdrawRequest) (*domain.Transaction, error) {
	currency, err := s.walletRepo.GetCurrencyByCode(ctx, req.CurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("currency not found: %w", err)
	}

	wallet, err := s.walletRepo.GetByUserAndCurrency(ctx, userID, currency.ID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	if wallet.Balance < req.Amount {
		return nil, fmt.Errorf("insufficient balance")
	}

	tx := &domain.Transaction{
		UserID:   userID,
		WalletID: wallet.ID,
		Type:     domain.TransactionTypeWithdrawal,
		Amount:   req.Amount,
		Fee:      req.Amount * 0.001,
		Status:   domain.TransactionStatusPending,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	newBalance := wallet.Balance - req.Amount - tx.Fee
	if err := s.walletRepo.UpdateBalance(ctx, wallet.ID, newBalance, wallet.Locked); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	tx.Status = domain.TransactionStatusCompleted
	if err := s.txRepo.Update(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (s *WalletService) GetTransactionHistory(ctx context.Context, userID int64, limit, offset int) ([]domain.Transaction, error) {
	return s.txRepo.GetUserTransactions(ctx, userID, limit, offset)
}

func (s *WalletService) GetAllCurrencies(ctx context.Context) ([]domain.Currency, error) {
	return s.walletRepo.GetAllCurrencies(ctx)
}
