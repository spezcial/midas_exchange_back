package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type DepositAddressRepository struct {
	db *database.Postgres
}

func NewDepositAddressRepository(db *database.Postgres) *DepositAddressRepository {
	return &DepositAddressRepository{db: db}
}

// ErrAddressConflict is returned by Create when another address already exists for
// this user+currency (ON CONFLICT DO NOTHING returned no row). The caller should
// re-read the existing record with GetByUserAndCurrency.
var ErrAddressConflict = errors.New("deposit address already exists for this user and currency")

func (r *DepositAddressRepository) Create(ctx context.Context, a *domain.DepositAddress) error {
	err := r.db.QueryRowContext(ctx, queries.DepositAddressCreateQuery,
		a.UserID, a.CurrencyID, a.Chain, a.Address,
	).Scan(&a.ID, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// ON CONFLICT DO NOTHING matched — a concurrent request already inserted.
		return ErrAddressConflict
	}
	return err
}

// GetByUserAndCurrency returns the deposit address for a user+currency, or nil if not yet created.
func (r *DepositAddressRepository) GetByUserAndCurrency(ctx context.Context, userID, currencyID int64) (*domain.DepositAddress, error) {
	var a domain.DepositAddress
	err := r.db.GetContext(ctx, &a, queries.DepositAddressGetByUserAndCurrencyQuery, userID, currencyID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &a, err
}

// GetAll returns every deposit address we have stored across all users.
func (r *DepositAddressRepository) GetAll(ctx context.Context) ([]domain.DepositAddress, error) {
	var addrs []domain.DepositAddress
	err := r.db.SelectContext(ctx, &addrs, queries.DepositAddressGetAllQuery)
	return addrs, err
}

// GetByAddress resolves a blockchain address to the owning user and currency.
// Used in the deposit webhook handler.
func (r *DepositAddressRepository) GetByAddress(ctx context.Context, address string) (*domain.DepositAddress, error) {
	var a domain.DepositAddress
	err := r.db.GetContext(ctx, &a, queries.DepositAddressGetByAddressQuery, address)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &a, err
}
