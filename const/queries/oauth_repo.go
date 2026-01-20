package queries

const (
	OAuthStateCreateQuery = `
		INSERT INTO oauth_states (state, expires_at)
		VALUES ($1, $2)
		RETURNING id, created_at
`

	OAuthStateGetByStateQuery = `
		SELECT * FROM oauth_states WHERE state = $1 AND expires_at > NOW()
`

	OAuthStateDeleteQuery = `DELETE FROM oauth_states WHERE state = $1`

	OAuthStateCleanupExpiredQuery = `DELETE FROM oauth_states WHERE expires_at <= NOW()`
)
