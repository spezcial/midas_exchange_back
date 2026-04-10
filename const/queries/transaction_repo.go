package queries

const (
	TransactionCreateQuery = `
		INSERT INTO transactions (user_id, wallet_id, type, amount, fee, status, tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
`

	transactionColumns = `id, user_id, wallet_id, type, amount, fee, status, tx_hash, created_at, updated_at`

	TransactionGetByIDQuery = `SELECT ` + transactionColumns + ` FROM transactions WHERE id = $1`

	TransactionGetUserTransactionsQuery = `
		SELECT ` + transactionColumns + ` FROM transactions
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
		SELECT ` + transactionColumns + ` FROM transactions
		WHERE status = 'pending'
		ORDER BY created_at ASC
`

	TransactionGetAllQuery = `
		SELECT ` + transactionColumns + ` FROM transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
`

	TransactionExistsByTxHashQuery = `SELECT EXISTS(SELECT 1 FROM transactions WHERE tx_hash = $1 AND tx_hash != '')`
)
