package queries

const (
	PasskeyCreateQuery = `
		INSERT INTO passkey_credentials (user_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
`

	PasskeyGetByUserIDQuery = `
		SELECT id, user_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, name, created_at, last_used_at
		FROM passkey_credentials
		WHERE user_id = $1
		ORDER BY created_at ASC
`

	PasskeyGetByCredentialIDQuery = `
		SELECT id, user_id, credential_id, public_key, aaguid, sign_count, backup_eligible, backup_state, name, created_at, last_used_at
		FROM passkey_credentials
		WHERE credential_id = $1
`

	PasskeyUpdateSignCountQuery = `
		UPDATE passkey_credentials
		SET sign_count = $1, backup_state = $2, last_used_at = NOW()
		WHERE credential_id = $3
`

	PasskeyDeleteQuery = `
		DELETE FROM passkey_credentials
		WHERE id = $1 AND user_id = $2
`

	PasskeyCountByUserIDQuery = `
		SELECT COUNT(*) FROM passkey_credentials WHERE user_id = $1
`
)
