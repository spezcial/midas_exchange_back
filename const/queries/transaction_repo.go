package queries

const (
	TransactionCreateQuery = `
		INSERT INTO transactions (user_id, wallet_id, type, amount, fee, status, tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
`

	TransactionGetByIDQuery = `SELECT * FROM transactions WHERE id = $1`

	TransactionGetUserTransactionsQuery = `
		SELECT * FROM transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
`

	TransactionUpdateQuery = `
		UPDATE transactions
		SET status = $1, tx_hash = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
`

	TransactionGetPendingQuery = `
		SELECT * FROM transactions
		WHERE status = 'pending'
		ORDER BY created_at ASC
`

	TransactionGetAllQuery = `
		SELECT * FROM transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
`
)

