package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/database"
	"github.com/shopspring/decimal"
)

type WalletRepository struct {
	db           *database.Postgres
	cacheService *cache.CacheService
}

func NewWalletRepository(db *database.Postgres, cacheService *cache.CacheService) *WalletRepository {
	return &WalletRepository{
		db:           db,
		cacheService: cacheService,
	}
}

func (r *WalletRepository) Create(ctx context.Context, wallet *domain.Wallet) error {
	if err := r.db.QueryRowContext(
		ctx, queries.WalletCreateQuery,
		wallet.UserID, wallet.CurrencyID, wallet.Balance, wallet.Locked,
	).Scan(&wallet.ID, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		return err
	}

	// Update cache immediately (no DB write - already done above)
	r.cacheService.SetWallet(wallet)

	return nil
}

func (r *WalletRepository) GetByID(ctx context.Context, id int64) (*domain.Wallet, error) {
	// Not cached by ID - not a common query pattern
	var wallet domain.Wallet
	err := r.db.GetContext(ctx, &wallet, queries.WalletGetByIDQuery, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("wallet not found")
	}
	return &wallet, err
}

func (r *WalletRepository) GetByUserAndCurrency(ctx context.Context, userID int64, currencyID int32) (*domain.Wallet, error) {
	// Check cache first
	if wallet, found := r.cacheService.GetWallet(userID, currencyID); found {
		return wallet, nil
	}

	// Cache miss - fetch from DB
	var wallet domain.Wallet
	err := r.db.GetContext(ctx, &wallet, queries.WalletGetByUserAndCurrencyQuery, userID, currencyID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("wallet not found")
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetWallet(&wallet)

	return &wallet, nil
}

func (r *WalletRepository) GetUserWallets(ctx context.Context, userID int64) ([]domain.WalletWithCurrency, error) {
	// Check cache first
	if wallets, found := r.cacheService.GetUserWallets(userID); found {
		return wallets, nil
	}

	// Cache miss - fetch from DB
	var wallets []domain.WalletWithCurrency
	err := r.db.SelectContext(ctx, &wallets, queries.WalletGetUserWalletsQuery, userID)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetUserWallets(userID, wallets)

	return wallets, nil
}

// AtomicDeduct decrements balance by amount only if current balance >= amount.
// Returns ErrInsufficientBalance if the balance check fails (prevents concurrent double-spend).
func (r *WalletRepository) AtomicDeduct(ctx context.Context, walletID int64, amount decimal.Decimal) error {
	var updatedAt time.Time
	var newBalance decimal.Decimal
	err := r.db.QueryRowContext(ctx, queries.WalletDeductBalanceQuery, amount, walletID).Scan(&updatedAt, &newBalance)
	if err == sql.ErrNoRows {
		return fmt.Errorf("insufficient balance")
	}
	if err != nil {
		return err
	}

	// Refresh cache entry with the new balance
	var wallet domain.Wallet
	if err := r.db.GetContext(context.Background(), &wallet, queries.WalletGetByIDQuery, walletID); err == nil {
		r.cacheService.SetWallet(&wallet)
	}
	return nil
}

// LockAmount moves amount from balance to locked atomically.
// Returns "insufficient balance" if balance < amount.
func (r *WalletRepository) LockAmount(ctx context.Context, walletID int64, amount decimal.Decimal) error {
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.WalletLockAmountQuery, amount, walletID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("insufficient balance")
	}
	if err != nil {
		return err
	}
	_ = r.RefreshWalletCache(ctx, walletID)
	return nil
}

// UnlockAmount moves amount from locked back to balance atomically.
// Silently succeeds even if locked < amount (no-op via ErrNoRows) — callers treat it as best-effort.
func (r *WalletRepository) UnlockAmount(ctx context.Context, walletID int64, amount decimal.Decimal) error {
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.WalletUnlockAmountQuery, amount, walletID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return nil // locked < amount, treat as already unlocked
	}
	if err != nil {
		return err
	}
	_ = r.RefreshWalletCache(ctx, walletID)
	return nil
}

// AtomicCredit increments wallet balance by amount using a single SQL UPDATE.
// Never use UpdateBalance for deposits — it reads first, then writes (race condition).
func (r *WalletRepository) AtomicCredit(ctx context.Context, walletID int64, amount decimal.Decimal) error {
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.WalletAddBalanceQuery, amount, walletID).Scan(&updatedAt)
	if err != nil {
		return err
	}
	_ = r.RefreshWalletCache(ctx, walletID)
	return nil
}

// RecordDeposit atomically creates a completed deposit transaction record and credits
// the wallet in a single DB transaction. A crash between the two writes is impossible.
func (r *WalletRepository) RecordDeposit(ctx context.Context, userID, walletID int64, amount decimal.Decimal, txHash *string) (*domain.Transaction, error) {
	dbTx, err := r.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer dbTx.Rollback()

	tx := &domain.Transaction{
		UserID:   userID,
		WalletID: walletID,
		Type:     domain.TransactionTypeDeposit,
		Amount:   amount,
		Fee:      decimal.Zero,
		Status:   domain.TransactionStatusCompleted,
		TxHash:   txHash,
	}
	err = dbTx.QueryRowContext(ctx, queries.TransactionCreateQuery,
		tx.UserID, tx.WalletID, tx.Type, tx.Amount, tx.Fee, tx.Status, tx.TxHash,
	).Scan(&tx.ID, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create deposit transaction: %w", err)
	}

	var updatedAt time.Time
	if err = dbTx.QueryRowContext(ctx, queries.WalletAddBalanceQuery, amount, walletID).Scan(&updatedAt); err != nil {
		return nil, fmt.Errorf("credit wallet: %w", err)
	}

	if err = dbTx.Commit(); err != nil {
		return nil, fmt.Errorf("commit deposit: %w", err)
	}

	_ = r.RefreshWalletCache(ctx, walletID)
	return tx, nil
}

// RefreshWalletCache reads the wallet from DB and updates the in-memory cache.
// Call this after operations that write to the wallet outside the normal repo methods
// (e.g., after CompleteOrderAtomic).
func (r *WalletRepository) RefreshWalletCache(ctx context.Context, walletID int64) error {
	var wallet domain.Wallet
	if err := r.db.GetContext(ctx, &wallet, queries.WalletGetByIDQuery, walletID); err != nil {
		return err
	}
	r.cacheService.SetWallet(&wallet)
	return nil
}

func (r *WalletRepository) GetCurrencyByCode(ctx context.Context, code string) (*domain.Currency, error) {
	// Check cache first - currencies are heavily cached
	if currency, found := r.cacheService.GetCurrency(code); found {
		return currency, nil
	}

	// Cache miss - fetch from DB
	var currency domain.Currency
	err := r.db.GetContext(ctx, &currency, queries.CurrencyGetByCodeQuery, code)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("currency not found")
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetCurrency(&currency)

	return &currency, nil
}

func (r *WalletRepository) GetAllCurrencies(ctx context.Context) ([]domain.Currency, error) {
	// Check cache first
	if currencies, found := r.cacheService.GetAllCurrencies(); found {
		return currencies, nil
	}

	// Cache miss - fetch from DB
	var currencies []domain.Currency
	err := r.db.SelectContext(ctx, &currencies, queries.CurrencyGetAllQuery)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetAllCurrencies(currencies)

	return currencies, nil
}
