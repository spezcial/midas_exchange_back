package queries

const (
	FingerprintExistsQuery = `
		SELECT EXISTS(
			SELECT 1 FROM user_fingerprints
			WHERE user_id = $1 AND fingerprint = $2
		)
`

	FingerprintCountByUserIDQuery = `
		SELECT COUNT(*) FROM user_fingerprints WHERE user_id = $1
`

	FingerprintUpsertQuery = `
		INSERT INTO user_fingerprints (user_id, fingerprint)
		VALUES ($1, $2)
		ON CONFLICT (user_id, fingerprint)
		DO UPDATE SET last_seen_at = NOW()
`
)
