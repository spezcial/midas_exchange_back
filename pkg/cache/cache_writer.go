package cache

import (
	"context"
	"fmt"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type CacheWriter struct {
	db *database.Postgres
}

func NewCacheWriter(db *database.Postgres) *CacheWriter {
	return &CacheWriter{db: db}
}

func (cw *CacheWriter) ProcessWrite(ctx context.Context, op WriteOperation) error {
	switch op.Type {
	case "user":
		return cw.writeUser(ctx, op)
	case "wallet":
		return cw.writeWallet(ctx, op)
	case "session":
		return cw.writeSession(ctx, op)
	case "exchange_rate":
		return cw.writeExchangeRate(ctx, op)
	default:
		return fmt.Errorf("unknown write operation type: %s", op.Type)
	}
}

func (cw *CacheWriter) writeUser(ctx context.Context, op WriteOperation) error {
	user, ok := op.Value.(*domain.User)
	if !ok {
		return fmt.Errorf("invalid user data")
	}

	switch op.Action {
	case "insert":
		query := `
			INSERT INTO users (id, email, password_hash, first_name, last_name, role, is_active, is_verified, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO NOTHING
		`
		_, err := cw.db.ExecContext(ctx, query,
			user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName,
			user.Role, user.IsActive, user.IsVerified, user.CreatedAt, user.UpdatedAt)
		return err

	case "update":
		query := `
			UPDATE users
			SET first_name = $1, last_name = $2, is_active = $3, is_verified = $4, updated_at = $5
			WHERE id = $6
		`
		_, err := cw.db.ExecContext(ctx, query,
			user.FirstName, user.LastName, user.IsActive, user.IsVerified, user.UpdatedAt, user.ID)
		return err

	default:
		return fmt.Errorf("unknown action for user: %s", op.Action)
	}
}

func (cw *CacheWriter) writeWallet(ctx context.Context, op WriteOperation) error {
	wallet, ok := op.Value.(*domain.Wallet)
	if !ok {
		return fmt.Errorf("invalid wallet data")
	}

	switch op.Action {
	case "insert":
		query := `
			INSERT INTO wallets (id, user_id, currency_id, balance, locked, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (user_id, currency_id) DO NOTHING
		`
		_, err := cw.db.ExecContext(ctx, query,
			wallet.ID, wallet.UserID, wallet.CurrencyID, wallet.Balance, wallet.Locked,
			wallet.CreatedAt, wallet.UpdatedAt)
		return err

	case "update":
		query := `
			UPDATE wallets
			SET balance = $1, locked = $2, updated_at = $3
			WHERE id = $4
		`
		_, err := cw.db.ExecContext(ctx, query,
			wallet.Balance, wallet.Locked, wallet.UpdatedAt, wallet.ID)
		return err

	default:
		return fmt.Errorf("unknown action for wallet: %s", op.Action)
	}
}

func (cw *CacheWriter) writeSession(ctx context.Context, op WriteOperation) error {
	session, ok := op.Value.(*domain.UserSession)
	if !ok {
		return fmt.Errorf("invalid session data")
	}

	switch op.Action {
	case "insert":
		query := `
			INSERT INTO user_sessions (id, user_id, refresh_token, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (refresh_token) DO NOTHING
		`
		_, err := cw.db.ExecContext(ctx, query,
			session.ID, session.UserID, session.RefreshToken, session.ExpiresAt, session.CreatedAt)
		return err

	case "delete":
		query := `DELETE FROM user_sessions WHERE refresh_token = $1`
		_, err := cw.db.ExecContext(ctx, query, session.RefreshToken)
		return err

	default:
		return fmt.Errorf("unknown action for session: %s", op.Action)
	}
}

func (cw *CacheWriter) writeExchangeRate(ctx context.Context, op WriteOperation) error {
	rate, ok := op.Value.(*domain.ExchangeRate)
	if !ok {
		return fmt.Errorf("invalid exchange rate data")
	}

	switch op.Action {
	case "insert":
		query := `
			INSERT INTO exchange_rates (id, from_currency_id, to_currency_id, rate, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (from_currency_id, to_currency_id) DO UPDATE SET rate = $4, updated_at = $7
		`
		_, err := cw.db.ExecContext(ctx, query,
			rate.ID, rate.FromCurrencyID, rate.ToCurrencyID, rate.Rate, rate.IsActive,
			rate.CreatedAt, rate.UpdatedAt)
		return err

	case "update":
		query := `
			UPDATE exchange_rates
			SET rate = $1, is_active = $2, updated_at = $3
			WHERE id = $4
		`
		_, err := cw.db.ExecContext(ctx, query,
			rate.Rate, rate.IsActive, rate.UpdatedAt, rate.ID)
		return err

	default:
		return fmt.Errorf("unknown action for exchange_rate: %s", op.Action)
	}
}
