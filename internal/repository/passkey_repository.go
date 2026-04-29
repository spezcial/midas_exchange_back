package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
)

// PasskeyRepository is the interface for passkey credential persistence.
// Defined as an interface so that services can depend on it and tests can mock it.
type PasskeyRepository interface {
	Create(ctx context.Context, cred *domain.PasskeyCredential) error
	GetByUserID(ctx context.Context, userID int64) ([]domain.PasskeyCredential, error)
	GetByCredentialID(ctx context.Context, credentialID string) (*domain.PasskeyCredential, error)
	UpdateSignCount(ctx context.Context, credentialID string, signCount uint32) error
	Delete(ctx context.Context, id int64, userID int64) error
	CountByUserID(ctx context.Context, userID int64) (int, error)
}

// passkeyRepository is the concrete PostgreSQL implementation.
type passkeyRepository struct {
	db *database.Postgres
}

// NewPasskeyRepository creates the PostgreSQL-backed PasskeyRepository.
func NewPasskeyRepository(db *database.Postgres) PasskeyRepository {
	return &passkeyRepository{db: db}
}

func (r *passkeyRepository) Create(ctx context.Context, cred *domain.PasskeyCredential) error {
	return r.db.QueryRowContext(ctx, queries.PasskeyCreateQuery,
		cred.UserID, cred.CredentialID, cred.PublicKey, cred.AAGUID, cred.SignCount, cred.BackupEligible, cred.Name,
	).Scan(&cred.ID, &cred.CreatedAt)
}

func (r *passkeyRepository) GetByUserID(ctx context.Context, userID int64) ([]domain.PasskeyCredential, error) {
	var creds []domain.PasskeyCredential
	err := r.db.SelectContext(ctx, &creds, queries.PasskeyGetByUserIDQuery, userID)
	return creds, err
}

func (r *passkeyRepository) GetByCredentialID(ctx context.Context, credentialID string) (*domain.PasskeyCredential, error) {
	var cred domain.PasskeyCredential
	err := r.db.GetContext(ctx, &cred, queries.PasskeyGetByCredentialIDQuery, credentialID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("passkey not found")
	}
	return &cred, err
}

func (r *passkeyRepository) UpdateSignCount(ctx context.Context, credentialID string, signCount uint32) error {
	_, err := r.db.ExecContext(ctx, queries.PasskeyUpdateSignCountQuery, signCount, credentialID)
	return err
}

func (r *passkeyRepository) Delete(ctx context.Context, id int64, userID int64) error {
	res, err := r.db.ExecContext(ctx, queries.PasskeyDeleteQuery, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("passkey not found")
	}
	return nil
}

func (r *passkeyRepository) CountByUserID(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, queries.PasskeyCountByUserIDQuery, userID).Scan(&count)
	return count, err
}
