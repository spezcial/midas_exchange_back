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

func (r *WalletRepository) UpdateBalance(ctx context.Context, walletID int64, balance, locked float64) error {
	// Get wallet first to know userID and currencyID for cache update
	var wallet domain.Wallet
	if err := r.db.GetContext(ctx, &wallet, queries.WalletGetForUpdateQuery, walletID); err != nil {
		return err
	}
	// Update in DB
	if err := r.db.QueryRowContext(ctx, queries.WalletUpdateBalanceQuery, balance, locked, walletID).Scan(&wallet.UpdatedAt); err != nil {
		return err
	}

	// Update cache values
	wallet.Balance = balance
	wallet.Locked = locked

	r.cacheService.SetWallet(&wallet)

	return nil
}

// AtomicDeduct decrements balance by amount only if current balance >= amount.
// Returns ErrInsufficientBalance if the balance check fails (prevents concurrent double-spend).
func (r *WalletRepository) AtomicDeduct(ctx context.Context, walletID int64, amount float64) error {
	var updatedAt time.Time
	var newBalance float64
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
func (r *WalletRepository) LockAmount(ctx context.Context, walletID int64, amount float64) error {
	var wallet domain.Wallet
	if err := r.db.GetContext(ctx, &wallet, queries.WalletGetForUpdateQuery, walletID); err != nil {
		return err
	}
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.WalletLockAmountQuery, amount, walletID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("insufficient balance")
	}
	if err != nil {
		return err
	}
	wallet.Balance -= amount
	wallet.Locked += amount
	wallet.UpdatedAt = updatedAt
	r.cacheService.SetWallet(&wallet)
	return nil
}

// UnlockAmount moves amount from locked back to balance atomically.
// Silently succeeds even if locked < amount (no-op via ErrNoRows) — callers treat it as best-effort.
func (r *WalletRepository) UnlockAmount(ctx context.Context, walletID int64, amount float64) error {
	var wallet domain.Wallet
	if err := r.db.GetContext(ctx, &wallet, queries.WalletGetForUpdateQuery, walletID); err != nil {
		return err
	}
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.WalletUnlockAmountQuery, amount, walletID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return nil // locked < amount, treat as already unlocked
	}
	if err != nil {
		return err
	}
	wallet.Locked -= amount
	wallet.Balance += amount
	wallet.UpdatedAt = updatedAt
	r.cacheService.SetWallet(&wallet)
	return nil
}

// FinalizeFromLocked is used at OTC order completion.
// fromAmount: the original amount that was locked at order creation.
// agreedFromAmount: the actual amount to consume (may differ from fromAmount).
// Effect: locked -= fromAmount, balance += (fromAmount - agreedFromAmount).
func (r *WalletRepository) FinalizeFromLocked(ctx context.Context, walletID int64, fromAmount, agreedFromAmount float64) error {
	var wallet domain.Wallet
	if err := r.db.GetContext(ctx, &wallet, queries.WalletGetForUpdateQuery, walletID); err != nil {
		return err
	}
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.WalletFinalizeLockedQuery, fromAmount, agreedFromAmount, walletID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("insufficient locked balance or funds cannot cover agreed amount")
	}
	if err != nil {
		return err
	}
	wallet.Locked -= fromAmount
	wallet.Balance += (fromAmount - agreedFromAmount)
	wallet.UpdatedAt = updatedAt
	r.cacheService.SetWallet(&wallet)
	return nil
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
