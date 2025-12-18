package queries

const (
	CurrencyExchangeCreateQuery = `
		INSERT INTO currency_exchanges (
			uid, user_id, from_currency_id, to_currency_id, from_amount, to_amount,
			to_amount_with_fee, exchange_rate, fee, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
`

	CurrencyExchangeGetByIDQuery = `
		SELECT c.id, c.uid, c.user_id, '' as email, c.from_currency_id,
		       c.to_currency_id, c.from_amount, c.to_amount, c.to_amount_with_fee,
		       c.exchange_rate, c.fee, c.status, c.created_at, c.updated_at,
		       fc.id as "from_currency.id", fc.code as "from_currency.code",
		       fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
		       fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
		       tc.id as "to_currency.id", tc.code as "to_currency.code",
		       tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
		       tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto"
		FROM currency_exchanges c
		LEFT JOIN currencies fc ON fc.id = c.from_currency_id
		LEFT JOIN currencies tc ON tc.id = c.to_currency_id
		WHERE c.id = $1`

	CurrencyExchangeGetByUIDQuery = `
		SELECT c.id, c.uid, c.user_id, '' as email, c.from_currency_id,
		       c.to_currency_id, c.from_amount, c.to_amount, c.to_amount_with_fee,
		       c.exchange_rate, c.fee, c.status, c.created_at, c.updated_at,
		       fc.id as "from_currency.id", fc.code as "from_currency.code",
		       fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
		       fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
		       tc.id as "to_currency.id", tc.code as "to_currency.code",
		       tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
		       tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto"
		FROM currency_exchanges c
		LEFT JOIN currencies fc ON fc.id = c.from_currency_id
		LEFT JOIN currencies tc ON tc.id = c.to_currency_id
		WHERE c.uid = $1`

	CurrencyExchangeGetUserExchangesQuery = `
		SELECT c.id, c.uid, c.user_id, '' as email, c.from_currency_id,
		       c.to_currency_id, c.from_amount, c.to_amount, c.to_amount_with_fee,
		       c.exchange_rate, c.fee, c.status, c.created_at, c.updated_at,
		       fc.id as "from_currency.id", fc.code as "from_currency.code",
		       fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
		       fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
		       tc.id as "to_currency.id", tc.code as "to_currency.code",
		       tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
		       tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto"
		FROM currency_exchanges c
		LEFT JOIN currencies fc ON fc.id = c.from_currency_id
		LEFT JOIN currencies tc ON tc.id = c.to_currency_id
		WHERE c.user_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3
`

	CurrencyExchangeGetUserExchangesCountQuery = `
		SELECT count(*) as cnt
		FROM currency_exchanges
		WHERE user_id = $1
`

	// Base queries - SELECT and JOIN only
	CurrencyExchangeGetAllBaseQuery = `
		SELECT c.id, c.uid, c.user_id, u.email, c.from_currency_id,
		       c.to_currency_id, c.from_amount, c.to_amount, c.to_amount_with_fee,
		       c.exchange_rate, c.fee, c.status, c.created_at, c.updated_at,
		       fc.id as "from_currency.id", fc.code as "from_currency.code",
		       fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
		       fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
		       tc.id as "to_currency.id", tc.code as "to_currency.code",
		       tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
		       tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto"
		FROM currency_exchanges c
		LEFT JOIN users u ON u.id = c.user_id
		LEFT JOIN currencies fc ON fc.id = c.from_currency_id
		LEFT JOIN currencies tc ON tc.id = c.to_currency_id
`

	CurrencyExchangeCountBaseQuery = `
		SELECT COUNT(*) as cnt
		FROM currency_exchanges c
		LEFT JOIN users u ON u.id = c.user_id
`

	CurrencyExchangeUpdateQuery = `
		UPDATE currency_exchanges
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING updated_at
`

	CurrencyExchangeGetExchangeRateQuery = `
		SELECT * FROM exchange_rates
		WHERE from_currency_id = $1 AND to_currency_id = $2 AND is_active = true
		LIMIT 1
`

	CurrencyExchangeCountByStatusQuery = `SELECT COUNT(*) as cnt FROM currency_exchanges WHERE status = $1`
)
