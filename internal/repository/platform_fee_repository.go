package repository

import (
	"context"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
	"github.com/shopspring/decimal"
)

type PlatformFeeRepository struct {
	db *database.Postgres
}

func NewPlatformFeeRepository(db *database.Postgres) *PlatformFeeRepository {
	return &PlatformFeeRepository{db: db}
}

func (r *PlatformFeeRepository) Create(ctx context.Context, fee *domain.PlatformFee) error {
	return r.db.QueryRowContext(ctx, queries.PlatformFeeCreateQuery,
		fee.UserID, fee.Operation, fee.CurrencyID, fee.GrossAmount, fee.Fee,
	).Scan(&fee.ID, &fee.CreatedAt)
}

func (r *PlatformFeeRepository) List(ctx context.Context, limit, offset int) ([]domain.PlatformFeeWithDetails, error) {
	var fees []domain.PlatformFeeWithDetails
	err := r.db.SelectContext(ctx, &fees, queries.PlatformFeeListQuery, limit, offset)
	return fees, err
}

func (r *PlatformFeeRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, queries.PlatformFeeCountQuery).Scan(&count)
	return count, err
}

// Totals returns exchange and withdrawal fee sums in a single DB query.
func (r *PlatformFeeRepository) Totals(ctx context.Context) (exchangeTotal, withdrawalTotal decimal.Decimal, err error) {
	err = r.db.QueryRowContext(ctx, queries.PlatformFeeTotalsQuery).Scan(&exchangeTotal, &withdrawalTotal)
	return
}
