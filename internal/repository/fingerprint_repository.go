package repository

import (
	"context"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/pkg/database"
)

// FingerprintRepository is the interface for login fingerprint persistence.
type FingerprintRepository interface {
	// Exists reports whether this fingerprint has been seen before for the user.
	Exists(ctx context.Context, userID int64, fingerprint string) (bool, error)
	// CountByUserID returns how many distinct fingerprints the user has registered.
	CountByUserID(ctx context.Context, userID int64) (int, error)
	// Upsert inserts the fingerprint or updates last_seen_at if it already exists.
	Upsert(ctx context.Context, userID int64, fingerprint string) error
}

type fingerprintRepository struct {
	db *database.Postgres
}

func NewFingerprintRepository(db *database.Postgres) FingerprintRepository {
	return &fingerprintRepository{db: db}
}

func (r *fingerprintRepository) Exists(ctx context.Context, userID int64, fingerprint string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, queries.FingerprintExistsQuery, userID, fingerprint).Scan(&exists)
	return exists, err
}

func (r *fingerprintRepository) CountByUserID(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, queries.FingerprintCountByUserIDQuery, userID).Scan(&count)
	return count, err
}

func (r *fingerprintRepository) Upsert(ctx context.Context, userID int64, fingerprint string) error {
	_, err := r.db.ExecContext(ctx, queries.FingerprintUpsertQuery, userID, fingerprint)
	return err
}
