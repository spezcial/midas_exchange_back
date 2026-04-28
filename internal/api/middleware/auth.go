package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/cache"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	UserEmailKey contextKey = "user_email"
	UserRoleKey  contextKey = "user_role"
	RawTokenKey  contextKey = "raw_token" // raw Bearer token string, used by logout
)

type AuthMiddleware struct {
	jwtManager     *auth.JWTManager
	tokenBlocklist cache.Store // Redis store for logout blocklist
}

func NewAuthMiddleware(jwtManager *auth.JWTManager, tokenBlocklist cache.Store) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager:     jwtManager,
		tokenBlocklist: tokenBlocklist,
	}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization header format"})
			return
		}

		rawToken := parts[1]
		claims, err := m.jwtManager.ValidateToken(rawToken)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		// Check if this specific token has been blocklisted (e.g. after logout).
		if m.isBlocklisted(r.Context(), claims.JTI) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "token has been revoked"})
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		ctx = context.WithValue(ctx, RawTokenKey, rawToken)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := r.Context().Value(UserRoleKey).(string)
			if !ok {
				respondJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}
			for _, role := range roles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
		})
	}
}

// BlockToken adds the given JTI to the blocklist with a TTL matching the token's
// remaining lifetime. Called by the logout handler.
func (m *AuthMiddleware) BlockToken(ctx context.Context, jti string, expiresAt time.Time) error {
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return nil // already expired — no need to blocklist
	}
	key := blocklistKey(jti)
	_, err := m.tokenBlocklist.SetNX(ctx, key, []byte("1"), remaining)
	return err
}

// BlockRawToken parses the token, validates it, and adds its JTI to the blocklist.
// This is the convenient variant for use in the Logout handler where we have the
// raw token string from context.
func (m *AuthMiddleware) BlockRawToken(ctx context.Context, rawToken string) error {
	claims, err := m.jwtManager.ValidateToken(rawToken)
	if err != nil || claims.JTI == "" {
		return nil // expired or no JTI (legacy token) — nothing to blocklist
	}
	return m.BlockToken(ctx, claims.JTI, claims.ExpiresAt.Time)
}

func (m *AuthMiddleware) isBlocklisted(ctx context.Context, jti string) bool {
	if jti == "" {
		return false // legacy token without JTI — allow through (no way to blocklist)
	}
	_, found, err := m.tokenBlocklist.Get(ctx, blocklistKey(jti))
	if err != nil {
		// Fail closed: if we cannot reach the blocklist store, treat the token as
		// revoked. This prevents logged-out tokens from being reused during Redis
		// outages. The tradeoff is that authenticated requests fail while Redis is
		// down, but that is preferable to a security hole in a financial app.
		return true
	}
	return found
}

func blocklistKey(jti string) string {
	return fmt.Sprintf("token_blocked:%s", jti)
}

func GetUserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	return userID, ok
}

func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}

func GetUserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(UserRoleKey).(string)
	return role, ok
}

func GetRawToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(RawTokenKey).(string)
	return token, ok
}
