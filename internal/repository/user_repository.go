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

type UserRepository struct {
	db           *database.Postgres
	cacheService *cache.CacheService
}

func NewUserRepository(db *database.Postgres, cacheService *cache.CacheService) *UserRepository {
	return &UserRepository{
		db:           db,
		cacheService: cacheService,
	}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if err := r.db.QueryRowContext(
		ctx, queries.UserCreateQuery,
		user.Email, user.PasswordHash, user.FirstName, user.LastName,
		user.Role, user.IsActive, user.IsVerified,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return err
	}

	// Update cache immediately (no DB write - already done above)
	r.cacheService.SetUser(user)

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	// Check cache first
	if user, found := r.cacheService.GetUser(id); found {
		return user, nil
	}

	// Cache miss - fetch from DB
	var user domain.User
	err := r.db.GetContext(ctx, &user, queries.UserGetByIDQuery, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetUser(&user)

	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	// Check cache first
	if user, found := r.cacheService.GetUserByEmail(email); found {
		return user, nil
	}

	// Cache miss - fetch from DB
	var user domain.User
	err := r.db.GetContext(ctx, &user, queries.UserGetByEmailQuery, email)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetUser(&user)

	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	if err := r.db.QueryRowContext(
		ctx, queries.UserUpdateQuery,
		user.FirstName, user.LastName, user.IsActive, user.IsVerified, user.ID,
	).Scan(&user.UpdatedAt); err != nil {
		return err
	}

	// Update cache immediately (no DB write - already done above)
	r.cacheService.SetUser(user)

	return nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int, email string) ([]domain.User, error) {
	// This operation is not cached - admin operation, not frequent
	var users []domain.User

	qb := newQueryBuilder(queries.UserListBaseQuery)

	qb.AddWhere(fmt.Sprintf("role = $%d", qb.paramCounter), "client")

	if email != "" {
		emailPattern := "%" + email + "%"
		qb.AddWhere(fmt.Sprintf("email ILIKE $%d", qb.paramCounter), emailPattern)
	}

	query, args := qb.Build("ORDER BY created_at DESC", fmt.Sprintf("LIMIT $%d OFFSET $%d", qb.paramCounter, qb.paramCounter+1))
	args = append(args, limit, offset)

	err := r.db.SelectContext(ctx, &users, query, args...)
	return users, err
}

func (r *UserRepository) Count(ctx context.Context, email string) (int64, error) {
	var count int64

	qb := newQueryBuilder(queries.UserCountBaseQuery)
	qb.AddWhere(fmt.Sprintf("role = $%d", qb.paramCounter), "client")
	if email != "" {
		emailPattern := "%" + email + "%"
		qb.AddWhere(fmt.Sprintf("email ILIKE $%d", qb.paramCounter), emailPattern)
	}

	query, args := qb.Build("", "")

	row := r.db.QueryRowContext(ctx, query, args...)
	err := row.Scan(&count)

	return count, err
}

func (r *UserRepository) CreateSession(ctx context.Context, session *domain.UserSession) error {
	if err := r.db.QueryRowContext(
		ctx, queries.UserSessionCreateQuery,
		session.UserID, session.RefreshToken, session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt); err != nil {
		return err
	}

	// Update cache immediately (no DB write - already done above)
	r.cacheService.SetSession(session, session.ExpiresAt.Sub(session.CreatedAt))

	return nil
}

func (r *UserRepository) GetSessionByToken(ctx context.Context, token string) (*domain.UserSession, error) {
	// Check cache first
	if session, found := r.cacheService.GetSession(token); found {
		return session, nil
	}

	// Cache miss - fetch from DB
	var session domain.UserSession
	err := r.db.GetContext(ctx, &session, queries.UserSessionGetByTokenQuery, token)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found or expired")
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetSession(&session, session.ExpiresAt.Sub(session.CreatedAt))

	return &session, nil
}

func (r *UserRepository) DeleteSession(ctx context.Context, token string) error {
	// Delete from DB immediately (critical operation)
	if _, err := r.db.ExecContext(ctx, queries.UserSessionDeleteQuery, token); err != nil {
		return err
	}

	// Delete from cache
	r.cacheService.DeleteSession(token)

	return nil
}
