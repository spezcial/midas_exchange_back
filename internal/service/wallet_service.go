package service

import (
	"context"
	"fmt"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/shopspring/decimal"
)

type WalletService struct {
	walletRepo   *repository.WalletRepository
	txRepo       *repository.TransactionRepository
	feeService   *PlatformFeeService
	twofaService TwoFactorService   // validates action tokens before withdrawals
	cgService    *CryptoGateService // optional; nil if crypto-gate is not configured
}

func NewWalletService(
	walletRepo *repository.WalletRepository,
	txRepo *repository.TransactionRepository,
	feeService *PlatformFeeService,
	twofaService TwoFactorService,
) *WalletService {
	return &WalletService{
		walletRepo:   walletRepo,
		txRepo:       txRepo,
		feeService:   feeService,
		twofaService: twofaService,
	}
}

func (s *WalletService) SetCryptoGateService(cg *CryptoGateService) {
	s.cgService = cg
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
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	// RecordDeposit wraps the transaction insert and balance credit in a single DB
	// transaction — identical to the webhook deposit path.
	return s.walletRepo.RecordDeposit(ctx, userID, wallet.ID, req.Amount, req.TxHash)
}

func (s *WalletService) Withdraw(ctx context.Context, userID int64, req *models.WithdrawRequest) (*domain.Transaction, error) {
	// Validate the action token before touching any balances.
	// An empty token means the user has no 2FA set up and cannot withdraw.
	if req.ActionToken == "" {
		return nil, ErrNoTwoFAMethod
	}
	ownerID, err := s.twofaService.ConsumeActionToken(ctx, req.ActionToken, domain.ActionWithdraw)
	if err != nil {
		return nil, fmt.Errorf("withdraw authorisation failed: %w", err)
	}
	if ownerID != userID {
		return nil, fmt.Errorf("action token does not belong to this user")
	}

	currency, err := s.walletRepo.GetCurrencyByCode(ctx, req.CurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("currency not found: %w", err)
	}

	wallet, err := s.walletRepo.GetByUserAndCurrency(ctx, userID, currency.ID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	_, isCrypto := domain.CurrencyChainFor(req.CurrencyCode)
	isCryptoOnChain := isCrypto && s.cgService != nil && req.ToAddress != ""

	var fee decimal.Decimal
	if !isCryptoOnChain {
		fee = req.Amount.Mul(decimal.RequireFromString("0.001"))
	}

	// AtomicDeduct checks balance and deducts in a single SQL UPDATE … WHERE balance >= $1.
	// No separate read-then-check needed — eliminates the double-spend race condition.
	if err := s.walletRepo.AtomicDeduct(ctx, wallet.ID, req.Amount.Add(fee)); err != nil {
		return nil, err // "insufficient balance" returned by the repo on failure
	}

	status := domain.TransactionStatusCompleted
	if isCryptoOnChain {
		status = domain.TransactionStatusPending
	}

	tx := &domain.Transaction{
		UserID:   userID,
		WalletID: wallet.ID,
		Type:     domain.TransactionTypeWithdrawal,
		Amount:   req.Amount,
		Fee:      fee,
		Status:   status,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		// Balance already deducted — refund before surfacing the error.
		if cErr := s.walletRepo.AtomicCredit(ctx, wallet.ID, req.Amount.Add(fee)); cErr != nil {
			return nil, fmt.Errorf("failed to create transaction (%w) and refund also failed (%v) — MANUAL INTERVENTION REQUIRED", err, cErr)
		}
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if isCryptoOnChain {
		if err := s.cgService.InitiateWithdrawal(ctx, tx, req.ToAddress, req.CurrencyCode); err != nil {
			return nil, err
		}
		return tx, nil
	}

	tx.Status = domain.TransactionStatusCompleted
	if err := s.txRepo.Update(ctx, tx); err != nil {
		return nil, err
	}

	// Record withdrawal fee in platform ledger (fire-and-forget).
	// Crypto on-chain withdrawals carry no platform fee (fee == 0), so this is fiat-only in practice.
	if fee.IsPositive() {
		s.feeService.Record(userID, domain.FeeOperationWithdrawal, currency.ID, req.Amount, fee)
	}

	return tx, nil
}

func (s *WalletService) GetTransactionHistory(ctx context.Context, userID int64, limit, offset int) ([]domain.Transaction, error) {
	return s.txRepo.GetUserTransactions(ctx, userID, limit, offset)
}

func (s *WalletService) GetAllCurrencies(ctx context.Context) ([]domain.Currency, error) {
	return s.walletRepo.GetAllCurrencies(ctx)
}
