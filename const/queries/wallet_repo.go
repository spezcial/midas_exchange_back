package queries

const (
	WalletCreateQuery = `
		INSERT INTO wallets (user_id, currency_id, balance, locked)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
`

	WalletGetByIDQuery = `SELECT * FROM wallets WHERE id = $1`

	WalletGetByUserAndCurrencyQuery = `SELECT * FROM wallets WHERE user_id = $1 AND currency_id = $2`

	WalletGetUserWalletsQuery = `
		SELECT
			w.id, w.user_id, w.currency_id, w.balance, w.locked, w.created_at, w.updated_at,
			c.id as "currency.id", c.code as "currency.code", c.name as "currency.name",
			c.symbol as "currency.symbol", c.is_active as "currency.is_active",
			c.is_crypto as "currency.is_crypto", c.created_at as "currency.created_at",
			c.updated_at as "currency.updated_at"
		FROM wallets w
		JOIN currencies c ON w.currency_id = c.id
		WHERE w.user_id = $1
		ORDER BY c.code
`

	WalletGetForUpdateQuery = `SELECT * FROM wallets WHERE id = $1`

	WalletUpdateBalanceQuery = `
		UPDATE wallets
		SET balance = $1, locked = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
`

	CurrencyGetByCodeQuery = `SELECT * FROM currencies WHERE code = $1 AND is_active = true`

	CurrencyGetAllQuery = `SELECT * FROM currencies WHERE is_active = true ORDER BY code`
)

