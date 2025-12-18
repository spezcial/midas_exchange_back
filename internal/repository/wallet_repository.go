package repository

import (
	"context"
	"database/sql"
	"fmt"

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

	// Update cache immediately (no DB write - already done above)
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
