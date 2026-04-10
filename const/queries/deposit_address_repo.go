package queries

const (
	// ON CONFLICT DO NOTHING handles the race where two concurrent requests both
	// miss the GetByUserAndCurrency check and try to insert for the same user+currency.
	// The caller must re-read the existing row when no row is returned.
	DepositAddressCreateQuery = `
		INSERT INTO crypto_deposit_addresses (user_id, currency_id, chain, address)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, currency_id) DO NOTHING
		RETURNING id, created_at
`

	DepositAddressGetByUserAndCurrencyQuery = `
		SELECT id, user_id, currency_id, chain, address, created_at
		FROM crypto_deposit_addresses
		WHERE user_id = $1 AND currency_id = $2
`

	DepositAddressGetByAddressQuery = `
		SELECT id, user_id, currency_id, chain, address, created_at
		FROM crypto_deposit_addresses
		WHERE address = $1
`

	DepositAddressGetAllQuery = `
		SELECT id, user_id, currency_id, chain, address, created_at
		FROM crypto_deposit_addresses
		ORDER BY created_at DESC
`
)
