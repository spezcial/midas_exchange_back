package queries

const (
	UserCreateQuery = `
		INSERT INTO users (email, password_hash, first_name, last_name, role, is_active, is_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
`

	UserGetByIDQuery = `SELECT * FROM users WHERE id = $1`

	UserGetByEmailQuery = `SELECT * FROM users WHERE email = $1`

	UserUpdateQuery = `
		UPDATE users
		SET first_name = $1, last_name = $2, is_active = $3, is_verified = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
`

	UserListQuery = `SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	UserSessionCreateQuery = `
		INSERT INTO user_sessions (user_id, refresh_token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
`

	UserSessionGetByTokenQuery = `SELECT * FROM user_sessions WHERE refresh_token = $1 AND expires_at > NOW()`

	UserSessionDeleteQuery = `DELETE FROM user_sessions WHERE refresh_token = $1`
)

