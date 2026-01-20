package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type OAuthRepository struct {
	db *database.Postgres
}

func NewOAuthRepository(db *database.Postgres) *OAuthRepository {
	return &OAuthRepository{db: db}
}

func (r *OAuthRepository) CreateOAuthState(ctx context.Context, state *domain.OAuthState) error {
	return r.db.QueryRowContext(
		ctx, queries.OAuthStateCreateQuery,
		state.State, state.ExpiresAt,
	).Scan(&state.ID, &state.CreatedAt)
}

func (r *OAuthRepository) GetAndDeleteOAuthState(ctx context.Context, state string) (*domain.OAuthState, error) {
	var oauthState domain.OAuthState
	err := r.db.GetContext(ctx, &oauthState, queries.OAuthStateGetByStateQuery, state)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid or expired state")
	}
	if err != nil {
		return nil, err
	}

	// Delete the state after retrieving it (one-time use)
	_, err = r.db.ExecContext(ctx, queries.OAuthStateDeleteQuery, state)
	if err != nil {
		return nil, err
	}

	return &oauthState, nil
}

func (r *OAuthRepository) CleanupExpiredStates(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, queries.OAuthStateCleanupExpiredQuery)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
