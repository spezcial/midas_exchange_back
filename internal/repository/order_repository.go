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

type CurrencyExchangeRepository struct {
	db           *database.Postgres
	cacheService *cache.CacheService
}

func NewCurrencyExchangeRepository(db *database.Postgres, cacheService *cache.CacheService) *CurrencyExchangeRepository {
	return &CurrencyExchangeRepository{
		db:           db,
		cacheService: cacheService,
	}
}

func (r *CurrencyExchangeRepository) Create(ctx context.Context, exchange *domain.CurrencyExchange) error {
	return r.db.QueryRowContext(
		ctx, queries.CurrencyExchangeCreateQuery,
		exchange.UID, exchange.UserID, exchange.FromCurrencyID, exchange.ToCurrencyID,
		exchange.FromAmount, exchange.ToAmount, exchange.ToAmountWithFee,
		exchange.ExchangeRate, exchange.Fee, exchange.Status,
	).Scan(&exchange.ID, &exchange.CreatedAt, &exchange.UpdatedAt)
}

func (r *CurrencyExchangeRepository) GetByID(ctx context.Context, id int64) (*domain.CurrencyExchangeWithCurrencies, error) {
	var exchange domain.CurrencyExchangeWithCurrencies
	err := r.db.GetContext(ctx, &exchange, queries.CurrencyExchangeGetByIDQuery, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("currency exchange not found")
	}
	return &exchange, err
}

func (r *CurrencyExchangeRepository) GetByUID(ctx context.Context, uid string) (*domain.CurrencyExchangeWithCurrencies, error) {
	var exchange domain.CurrencyExchangeWithCurrencies
	err := r.db.GetContext(ctx, &exchange, queries.CurrencyExchangeGetByUIDQuery, uid)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("currency exchange not found")
	}
	return &exchange, err
}

func (r *CurrencyExchangeRepository) GetUserExchanges(ctx context.Context, userID int64, limit, offset int) ([]domain.CurrencyExchangeWithCurrencies, error) {
	var exchanges []domain.CurrencyExchangeWithCurrencies
	err := r.db.SelectContext(ctx, &exchanges, queries.CurrencyExchangeGetUserExchangesQuery, userID, limit, offset)
	return exchanges, err
}

func (r *CurrencyExchangeRepository) GetUserExchangesCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	var err error
	row := r.db.QueryRowContext(ctx, queries.CurrencyExchangeGetUserExchangesCountQuery, userID)
	if row != nil {
		err = row.Scan(&count)
	}
	return count, err
}

func (r *CurrencyExchangeRepository) GetAllExchanges(ctx context.Context, status, email string, limit, offset int) ([]domain.CurrencyExchangeWithCurrencies, error) {
	var exchanges []domain.CurrencyExchangeWithCurrencies

	qb := newQueryBuilder(queries.CurrencyExchangeGetAllBaseQuery)

	if status != "" {
		qb.AddWhere(fmt.Sprintf("c.status = $%d", qb.paramCounter), status)
	}

	if email != "" {
		qb.AddWhere(fmt.Sprintf("u.email = $%d", qb.paramCounter), email)
	}

	query, args := qb.Build("ORDER BY c.created_at DESC", fmt.Sprintf("LIMIT $%d OFFSET $%d", qb.paramCounter, qb.paramCounter+1))
	args = append(args, limit, offset)

	err := r.db.SelectContext(ctx, &exchanges, query, args...)
	return exchanges, err
}

func (r *CurrencyExchangeRepository) GetAllExchangesCount(ctx context.Context, status, email string) (int64, error) {
	var count int64

	qb := newQueryBuilder(queries.CurrencyExchangeCountBaseQuery)

	if status != "" {
		qb.AddWhere(fmt.Sprintf("c.status = $%d", qb.paramCounter), status)
	}

	if email != "" {
		qb.AddWhere(fmt.Sprintf("u.email = $%d", qb.paramCounter), email)
	}

	query, args := qb.Build("", "")

	row := r.db.QueryRowContext(ctx, query, args...)
	err := row.Scan(&count)

	return count, err
}

func (r *CurrencyExchangeRepository) Update(ctx context.Context, exchange *domain.CurrencyExchange) error {
	return r.db.QueryRowContext(
		ctx, queries.CurrencyExchangeUpdateQuery,
		exchange.Status, exchange.ID,
	).Scan(&exchange.UpdatedAt)
}

func (r *CurrencyExchangeRepository) GetExchangeRate(ctx context.Context, fromCurrencyID, toCurrencyID int32) (*domain.ExchangeRate, error) {
	if rate, found := r.cacheService.GetExchangeRate(fromCurrencyID, toCurrencyID); found {
		return rate, nil
	}

	// Cache miss - fetch from DB
	var rate domain.ExchangeRate
	err := r.db.GetContext(ctx, &rate, queries.CurrencyExchangeGetExchangeRateQuery, fromCurrencyID, toCurrencyID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("exchange rate not found")
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetExchangeRate(&rate)

	return &rate, nil
}

func (r *CurrencyExchangeRepository) CountExchangesByStatus(ctx context.Context, status domain.CurrencyExchangeStatus) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, queries.CurrencyExchangeCountByStatusQuery, status)
	return count, err
}
