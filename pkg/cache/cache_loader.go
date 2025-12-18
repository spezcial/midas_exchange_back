package cache

import (
	"context"
	"github.com/caspianex/exchange-backend/const/queries"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

type CacheLoader struct {
	db           *database.Postgres
	cacheService *CacheService
	logger       *logger.Logger
}

func NewCacheLoader(db *database.Postgres, cacheService *CacheService, logger *logger.Logger) *CacheLoader {
	return &CacheLoader{
		db:           db,
		cacheService: cacheService,
		logger:       logger,
	}
}

func (cl *CacheLoader) WarmUpCache(ctx context.Context) error {
	cl.logger.Info("Starting cache warm-up")
	start := time.Now()

	if err := cl.loadCurrencies(ctx); err != nil {
		return err
	}

	if err := cl.loadExchangeRates(ctx); err != nil {
		return err
	}

	if err := cl.loadUsers(ctx); err != nil {
		return err
	}

	cl.logger.Info("Cache warm-up completed", "duration_ms", time.Since(start).Milliseconds())
	return nil
}

func (cl *CacheLoader) loadCurrencies(ctx context.Context) error {
	var currencies []domain.Currency

	if err := cl.db.SelectContext(ctx, &currencies, queries.CurrencyGetAllQuery); err != nil {
		cl.logger.Error("Failed to load currencies", "error", err)
		return err
	}

	cl.cacheService.SetAllCurrencies(currencies)
	cl.logger.Info("Loaded currencies into cache", "count", len(currencies))

	return nil
}

func (cl *CacheLoader) loadExchangeRates(ctx context.Context) error {
	var rates []domain.ExchangeRate
	query := `SELECT * FROM exchange_rates WHERE is_active = true`

	if err := cl.db.SelectContext(ctx, &rates, query); err != nil {
		cl.logger.Error("Failed to load exchange rates", "error", err)
		return err
	}

	// Load individual rates into cache (both by pair and by ID)
	for i := range rates {
		cl.cacheService.UpdateExchangeRateCacheOnly(&rates[i])
	}

	// Load aggregate caches
	var ratesWithCurrencies []domain.ExchangeRateWithCurrencies
	err := cl.db.SelectContext(ctx, &ratesWithCurrencies, queries.ExchangeRateWithCurrenciesQuery)
	if err != nil {
		return nil
	}

	// Set aggregate caches
	cl.cacheService.SetAllExchangeRates(ratesWithCurrencies)

	// Load active rates
	var activeRates []domain.ExchangeRateWithCurrencies
	err = cl.db.SelectContext(ctx, &activeRates, queries.ExchangeRateGetActiveQuery)
	if err == nil {
		cl.cacheService.SetActiveExchangeRates(activeRates)
	}

	cl.logger.Info("Loaded exchange rates into cache", "count", len(rates))
	return nil
}

func (cl *CacheLoader) loadUsers(ctx context.Context) error {
	var users []domain.User
	query := `SELECT * FROM users`

	if err := cl.db.SelectContext(ctx, &users, query); err != nil {
		cl.logger.Error("Failed to load recent users", "error", err)
		return err
	}

	for i := range users {
		cl.cacheService.SetUser(&users[i])
	}

	cl.logger.Info("Loaded users into cache", "count", len(users))
	return nil
}

// Optional: Load recently active users
func (cl *CacheLoader) LoadRecentUsers(ctx context.Context, limit int) error {
	var users []domain.User
	query := `
		SELECT * FROM users
		WHERE updated_at > NOW() - INTERVAL '7 days'
		ORDER BY updated_at DESC
		LIMIT $1
	`

	if err := cl.db.SelectContext(ctx, &users, query, limit); err != nil {
		cl.logger.Error("Failed to load recent users", "error", err)
		return err
	}

	for i := range users {
		cl.cacheService.SetUser(&users[i])
	}

	cl.logger.Info("Loaded recent users into cache", "count", len(users))
	return nil
}

// Optional: Load user wallets for active sessions
func (cl *CacheLoader) LoadUserWallets(ctx context.Context, userID int64) error {
	var wallets []domain.WalletWithCurrency

	if err := cl.db.SelectContext(ctx, &wallets, queries.WalletGetUserWalletsQuery, userID); err != nil {
		return err
	}

	cl.cacheService.SetUserWallets(userID, wallets)
	return nil
}
