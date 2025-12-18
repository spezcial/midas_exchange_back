package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type ExchangeRateRepository struct {
	db           *database.Postgres
	cacheService *cache.CacheService
}

func NewExchangeRateRepository(db *database.Postgres, cacheService *cache.CacheService) *ExchangeRateRepository {
	return &ExchangeRateRepository{
		db:           db,
		cacheService: cacheService,
	}
}

func (r *ExchangeRateRepository) Create(ctx context.Context, rate *domain.ExchangeRate) error {
	if err := r.db.QueryRowContext(
		ctx, queries.ExchangeRateCreateQuery,
		rate.FromCurrencyID, rate.ToCurrencyID, rate.Rate, rate.Fee, rate.IsActive,
	).Scan(&rate.ID, &rate.CreatedAt, &rate.UpdatedAt); err != nil {
		return err
	}

	// Update all cache keys for this exchange rate
	r.cacheService.UpdateExchangeRateCache(rate)

	return nil
}

func (r *ExchangeRateRepository) GetByPair(ctx context.Context, fromId, toId int32) (*domain.ExchangeRate, error) {
	// Check cache first
	if rate, found := r.cacheService.GetExchangeRate(fromId, toId); found {
		return rate, nil
	}

	// Cache miss - fetch from DB
	var rate domain.ExchangeRate
	err := r.db.GetContext(ctx, &rate, queries.ExchangeRateGetByPairQuery, fromId, toId)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("exchange rate not found")
	}
	if err != nil {
		return nil, err
	}

	// Update all cache keys for this exchange rate
	r.cacheService.UpdateExchangeRateCache(&rate)

	return &rate, nil
}

func (r *ExchangeRateRepository) GetByID(ctx context.Context, id int64) (*domain.ExchangeRate, error) {
	// Check cache first
	if rate, found := r.cacheService.GetExchangeRateById(id); found {
		return rate, nil
	}

	// Cache miss - fetch from DB
	var rate domain.ExchangeRate
	err := r.db.GetContext(ctx, &rate, queries.ExchangeRateGetByIDQuery, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("exchange rate not found")
	}
	if err != nil {
		return nil, err
	}

	// Update all cache keys for this exchange rate
	r.cacheService.UpdateExchangeRateCache(&rate)

	return &rate, nil
}

func (r *ExchangeRateRepository) GetAll(ctx context.Context) ([]domain.ExchangeRateWithCurrencies, error) {
	// Check cache first
	if rates, found := r.cacheService.GetAllExchangeRates(); found {
		return rates, nil
	}

	// Cache miss - fetch from DB
	var rates []domain.ExchangeRateWithCurrencies
	err := r.db.SelectContext(ctx, &rates, queries.ExchangeRateWithCurrenciesQuery)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetAllExchangeRates(rates)

	return rates, nil
}

func (r *ExchangeRateRepository) GetActive(ctx context.Context) ([]domain.ExchangeRateWithCurrencies, error) {
	// Check cache first
	if rates, found := r.cacheService.GetActiveExchangeRates(); found {
		return rates, nil
	}

	// Cache miss - fetch from DB
	var rates []domain.ExchangeRateWithCurrencies
	err := r.db.SelectContext(ctx, &rates, queries.ExchangeRateGetActiveQuery)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetActiveExchangeRates(rates)

	return rates, nil
}

func (r *ExchangeRateRepository) Update(ctx context.Context, rate *domain.ExchangeRate) error {
	if err := r.db.QueryRowContext(
		ctx, queries.ExchangeRateUpdateQuery,
		rate.Fee, rate.IsActive, rate.ID,
	).Scan(&rate.UpdatedAt); err != nil {
		return err
	}

	// Update all cache keys for this exchange rate
	r.cacheService.UpdateExchangeRateCache(rate)

	return nil
}

func (r *ExchangeRateRepository) Delete(ctx context.Context, id int64) error {
	row := r.db.QueryRowContext(ctx, queries.ExchangeRateDeleteQuery, id)
	if row != nil {
		var from, to int32
		err := row.Scan(&from, &to)
		if err != nil {
			return err
		}
		// Invalidate all cache keys for this exchange rate
		r.cacheService.InvalidateExchangeRateCache(from, to, id)
	}

	return nil
}

// RateUpdateData holds data for batch updating rates (worker updates only rate, not fee)
type RateUpdateData struct {
	ID   int64
	Rate float64
}

// BatchUpdate updates multiple exchange rates in a single transaction
// Only updates the rate field - fee is managed by admins
func (r *ExchangeRateRepository) BatchUpdate(ctx context.Context, updates []RateUpdateData) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build batch update query using CASE statements
	var ids []string
	var ratesCases []string
	args := make([]interface{}, 0, len(updates)*2)

	for i, update := range updates {
		ids = append(ids, fmt.Sprintf("$%d", i*2+1))
		ratesCases = append(ratesCases, fmt.Sprintf("WHEN id = $%d THEN $%d::numeric", i*2+1, i*2+2))

		args = append(args, update.ID, update.Rate)
	}

	query := fmt.Sprintf(`
		UPDATE exchange_rates
		SET
			rate = (CASE %s END)::numeric,
			updated_at = NOW()
		WHERE id IN (%s)
	`, strings.Join(ratesCases, " "), strings.Join(ids, ","))

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute batch update: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if int(rowsAffected) != len(updates) {
		return fmt.Errorf("expected to update %d rows, but updated %d", len(updates), rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update cache for all updated rates
	for _, update := range updates {
		rate, err := r.GetByID(ctx, update.ID)
		if err == nil {
			// Update both cache keys (by pair and by ID) without invalidating lists yet
			r.cacheService.UpdateExchangeRateCacheOnly(rate)
		}
	}

	// Invalidate aggregate caches once at the end (more efficient for batch)
	r.cacheService.InvalidateExchangeRateListCaches()

	return nil
}
