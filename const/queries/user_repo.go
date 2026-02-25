package queries

const (
	UserCreateQuery = `
		INSERT INTO users (email, password_hash, first_name, last_name, role, is_active, is_verified, auth_method, google_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
`

	UserGetByIDQuery = `SELECT * FROM users WHERE id = $1`

	UserGetByEmailQuery = `SELECT * FROM users WHERE email = $1 AND role = 'client'`

	UserGetByGoogleIDQuery = `SELECT * FROM users WHERE google_id = $1`

	UserUpdateQuery = `
		UPDATE users
		SET first_name = $1, last_name = $2, is_active = $3, is_verified = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
`

	UserUpdateProfileQuery = `
		UPDATE users
		SET first_name = $1, last_name = $2, middle_name = $3, phone = $4, kyc_level = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING updated_at
`

	UserUpdatePasswordQuery = `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
`

	// Base queries for queryBuilder
	UserListBaseQuery = `SELECT * FROM users`

	UserCountBaseQuery = `SELECT COUNT(*) FROM users`

	UserSessionCreateQuery = `
		INSERT INTO user_sessions (user_id, refresh_token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
`

	UserSessionGetByTokenQuery = `SELECT * FROM user_sessions WHERE refresh_token = $1 AND expires_at > NOW()`

	UserSessionDeleteQuery = `DELETE FROM user_sessions WHERE refresh_token = $1`
)
