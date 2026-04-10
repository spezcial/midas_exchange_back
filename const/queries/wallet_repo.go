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
			c.updated_at as "currency.updated_at",
			da.address as deposit_address
		FROM wallets w
		JOIN currencies c ON w.currency_id = c.id
		LEFT JOIN crypto_deposit_addresses da ON da.user_id = w.user_id AND da.currency_id = w.currency_id
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

	// WalletDeductBalanceQuery atomically deducts amount only if current balance >= amount.
	// Returns updated_at on success; no row if balance is insufficient (prevents double-spend).
	WalletDeductBalanceQuery = `
		UPDATE wallets
		SET balance = balance - $1, updated_at = NOW()
		WHERE id = $2 AND balance >= $1
		RETURNING updated_at, balance
`

	// WalletLockAmountQuery atomically moves `amount` from balance to locked.
	// Fails (no row returned) if balance < amount.
	WalletLockAmountQuery = `
    UPDATE wallets
    SET balance = balance - $1, locked = locked + $1, updated_at = NOW()
    WHERE id = $2 AND balance >= $1
    RETURNING updated_at
`

	// WalletUnlockAmountQuery atomically moves `amount` from locked back to balance.
	// Fails (no row returned) if locked < amount.
	WalletUnlockAmountQuery = `
    UPDATE wallets
    SET locked = locked - $1, balance = balance + $1, updated_at = NOW()
    WHERE id = $2 AND locked >= $1
    RETURNING updated_at
`

	// WalletFinalizeLockedQuery is used at OTC completion.
	// $1 = from_amount (the original locked amount to fully release from locked)
	// $2 = agreed_from_amount (the actual amount consumed; may differ from from_amount)
	// Effect: locked -= from_amount, balance += (from_amount - agreed_from_amount)
	// Net: client pays agreed_from_amount; any excess lock is returned to balance.
	// Fails if locked < from_amount OR if resulting balance would go negative.
	WalletFinalizeLockedQuery = `
    UPDATE wallets
    SET locked = locked - $1, balance = balance + ($1 - $2), updated_at = NOW()
    WHERE id = $3
      AND locked >= $1
      AND balance + ($1 - $2) >= 0
    RETURNING updated_at
`

	// WalletAddBalanceQuery atomically increments balance by amount.
	WalletAddBalanceQuery = `
    UPDATE wallets
    SET balance = balance + $1, updated_at = NOW()
    WHERE id = $2
    RETURNING updated_at
`

	CurrencyGetByCodeQuery = `SELECT * FROM currencies WHERE code = $1 AND is_active = true`

	CurrencyGetAllQuery = `SELECT * FROM currencies WHERE is_active = true ORDER BY code`
)
