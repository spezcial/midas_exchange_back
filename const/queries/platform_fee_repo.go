package queries

const (
	PlatformFeeCreateQuery = `
		INSERT INTO platform_fees (user_id, operation, currency_id, gross_amount, fee)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
`

	PlatformFeeListQuery = `
		SELECT
			pf.id, pf.user_id, pf.operation, pf.currency_id, pf.gross_amount, pf.fee, pf.created_at,
			u.email  AS user_email,
			c.code   AS currency_code,
			c.symbol AS currency_symbol
		FROM platform_fees pf
		JOIN users      u ON u.id = pf.user_id
		JOIN currencies c ON c.id = pf.currency_id
		ORDER BY pf.created_at DESC
		LIMIT $1 OFFSET $2
`

	PlatformFeeCountQuery = `SELECT COUNT(*) FROM platform_fees`

	// PlatformFeeTotalsQuery returns exchange and withdrawal fee totals in a single scan.
	PlatformFeeTotalsQuery = `
		SELECT
			COALESCE(SUM(fee) FILTER (WHERE operation = 'exchange'),   0) AS exchange_total,
			COALESCE(SUM(fee) FILTER (WHERE operation = 'withdrawal'), 0) AS withdrawal_total
		FROM platform_fees
`
)
