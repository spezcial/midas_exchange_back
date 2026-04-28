package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	// Set default auth_method if not specified
	if user.AuthMethod == "" {
		user.AuthMethod = domain.AuthMethodRegular
	}

	if err := r.db.QueryRowContext(
		ctx, queries.UserCreateQuery,
		user.Email, user.PasswordHash, user.FirstName, user.LastName,
		user.Role, user.IsActive, user.IsVerified, user.AuthMethod, user.GoogleID,
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
	// Bypass cache — callers (registration, forgot-password) need a DB-authoritative
	// result and must not receive a cached copy with an empty PasswordHash.
	var user domain.User
	err := r.db.GetContext(ctx, &user, queries.UserGetByEmailQuery, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
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

func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	if _, err := r.db.ExecContext(ctx, queries.UserUpdatePasswordQuery, passwordHash, userID); err != nil {
		return err
	}

	// Invalidate both cache keys so next login fetches fresh data with new hash.
	// Try to resolve email from cache before evicting the ID key.
	if user, found := r.cacheService.GetUser(userID); found {
		r.cacheService.InvalidateUserEmail(user.Email)
	}
	r.cacheService.InvalidateUser(userID)

	return nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, user *domain.User) error {
	if err := r.db.QueryRowContext(
		ctx, queries.UserUpdateProfileQuery,
		user.FirstName, user.LastName, user.MiddleName, user.KycLevel, user.ID,
	).Scan(&user.UpdatedAt); err != nil {
		return err
	}

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

	// Cache with remaining TTL, not the full session duration.
	// Using the full duration would keep an expired session alive in Redis
	// long after the DB record has been deleted.
	remainingTTL := time.Until(session.ExpiresAt)
	if remainingTTL <= 0 {
		return nil, fmt.Errorf("session not found or expired")
	}
	r.cacheService.SetSession(&session, remainingTTL)

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

func (r *UserRepository) GetByEmailAnyRole(ctx context.Context, email string) (*domain.User, error) {
	// Bypass cache — this is the login path and PasswordHash must come from the DB.
	// The User struct has json:"-" on PasswordHash, so cached copies have an empty hash.
	var user domain.User
	err := r.db.GetContext(ctx, &user, queries.UserGetByEmailAnyRoleQuery, email)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListStaff(ctx context.Context, limit, offset int, email string) ([]domain.User, error) {
	var users []domain.User

	qb := newQueryBuilder(queries.UserListBaseQuery)
	qb.AddWhere(fmt.Sprintf("role != $%d", qb.paramCounter), "client")

	if email != "" {
		emailPattern := "%" + email + "%"
		qb.AddWhere(fmt.Sprintf("email ILIKE $%d", qb.paramCounter), emailPattern)
	}

	query, args := qb.Build("ORDER BY created_at DESC", fmt.Sprintf("LIMIT $%d OFFSET $%d", qb.paramCounter, qb.paramCounter+1))
	args = append(args, limit, offset)

	err := r.db.SelectContext(ctx, &users, query, args...)
	return users, err
}

func (r *UserRepository) CountStaff(ctx context.Context, email string) (int64, error) {
	var count int64

	qb := newQueryBuilder(queries.UserCountBaseQuery)
	qb.AddWhere(fmt.Sprintf("role != $%d", qb.paramCounter), "client")
	if email != "" {
		emailPattern := "%" + email + "%"
		qb.AddWhere(fmt.Sprintf("email ILIKE $%d", qb.paramCounter), emailPattern)
	}

	query, args := qb.Build("", "")

	row := r.db.QueryRowContext(ctx, query, args...)
	err := row.Scan(&count)

	return count, err
}

func (r *UserRepository) UpdateStaff(ctx context.Context, user *domain.User) error {
	if err := r.db.QueryRowContext(
		ctx, queries.UserUpdateStaffQuery,
		user.FirstName, user.LastName, user.Role, user.IsActive, user.ID,
	).Scan(&user.UpdatedAt); err != nil {
		return err
	}

	r.cacheService.SetUser(user)

	return nil
}

func (r *UserRepository) GetByGoogleID(ctx context.Context, googleID string) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user, queries.UserGetByGoogleIDQuery, googleID)
	if err == sql.ErrNoRows {
		return nil, nil // Return nil, nil to distinguish from errors
	}
	if err != nil {
		return nil, err
	}

	// Update cache
	r.cacheService.SetUser(&user)

	return &user, nil
}

// GetByPhone looks up a user by their phone number. Returns sql.ErrNoRows if not found.
func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user, queries.UserGetByPhoneQuery, phone)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdatePhone sets the phone number and phone_verified flag for a user.
func (r *UserRepository) UpdatePhone(ctx context.Context, userID int64, phone *string, verified bool) error {
	if err := r.db.QueryRowContext(ctx, queries.UserUpdatePhoneQuery, phone, verified, userID).Scan(new(time.Time)); err != nil {
		return err
	}
	// Invalidate cache so the updated phone/verified fields are fetched fresh.
	if user, found := r.cacheService.GetUser(userID); found {
		r.cacheService.InvalidateUserEmail(user.Email)
	}
	r.cacheService.InvalidateUser(userID)
	return nil
}

// DeleteAllSessionsByUserID removes every session for a user.
// Called after password change or password reset to invalidate all existing logins.
func (r *UserRepository) DeleteAllSessionsByUserID(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, queries.UserSessionDeleteAllByUserIDQuery, userID)
	return err
}
