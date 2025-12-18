package queries

const (
	ExchangeRateWithCurrenciesQuery = `
		SELECT
			er.id, er.from_currency_id, er.to_currency_id, er.rate, er.fee, er.is_active,
			er.created_at, er.updated_at,
			bc.id as "from_currency.id", bc.code as "from_currency.code",
			bc.name as "from_currency.name", bc.symbol as "from_currency.symbol",
			bc.is_active as "from_currency.is_active", bc.is_crypto as "from_currency.is_crypto",
			bc.created_at as "from_currency.created_at", bc.updated_at as "from_currency.updated_at",
			qc.id as "to_currency.id", qc.code as "to_currency.code",
			qc.name as "to_currency.name", qc.symbol as "to_currency.symbol",
			qc.is_active as "to_currency.is_active", qc.is_crypto as "to_currency.is_crypto",
			qc.created_at as "to_currency.created_at", qc.updated_at as "to_currency.updated_at"
		FROM exchange_rates er
		JOIN currencies bc ON er.from_currency_id = bc.id
		JOIN currencies qc ON er.to_currency_id = qc.id
		ORDER BY bc.code, qc.code
`

	ExchangeRateCreateQuery = `
		INSERT INTO exchange_rates (from_currency_id, to_currency_id, rate, fee, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
`

	ExchangeRateGetByPairQuery = `SELECT * FROM exchange_rates WHERE from_currency_id = $1 AND to_currency_id = $2`

	ExchangeRateGetByIDQuery = `SELECT * FROM exchange_rates WHERE id = $1`

	ExchangeRateGetActiveQuery = `
		SELECT
			ep.id, ep.from_currency_id, ep.to_currency_id, ep.rate, ep.fee, ep.is_active,
			ep.created_at, ep.updated_at,
			bc.id as "from_currency.id", bc.code as "from_currency.code",
			bc.name as "from_currency.name", bc.symbol as "from_currency.symbol",
			bc.is_active as "from_currency.is_active", bc.is_crypto as "from_currency.is_crypto",
			bc.created_at as "from_currency.created_at", bc.updated_at as "from_currency.updated_at",
			qc.id as "to_currency.id", qc.code as "to_currency.code",
			qc.name as "to_currency.name", qc.symbol as "to_currency.symbol",
			qc.is_active as "to_currency.is_active", qc.is_crypto as "to_currency.is_crypto",
			qc.created_at as "to_currency.created_at", qc.updated_at as "to_currency.updated_at"
		FROM exchange_rates ep
		JOIN currencies bc ON ep.from_currency_id = bc.id
		JOIN currencies qc ON ep.to_currency_id = qc.id
		WHERE ep.is_active = true AND bc.is_active = true AND qc.is_active = true
		ORDER BY bc.code, qc.code
`

	ExchangeRateUpdateQuery = `
		UPDATE exchange_rates
		SET fee = $1, is_active = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
`

	ExchangeRateDeleteQuery = `DELETE FROM exchange_rates WHERE id = $1 RETURNING from_currency_id, to_currency_id`
)
