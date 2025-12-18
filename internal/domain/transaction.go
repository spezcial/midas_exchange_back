package domain

import (
	"time"
)

type TransactionType string
type TransactionStatus string

const (
	TransactionTypeDeposit    TransactionType = "deposit"
	TransactionTypeWithdrawal TransactionType = "withdrawal"

	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusCanceled  TransactionStatus = "canceled"
)

type Transaction struct {
	ID        int64             `db:"id" json:"id"`
	UserID    int64             `db:"user_id" json:"user_id"`
	WalletID  int64             `db:"wallet_id" json:"wallet_id"`
	Type      TransactionType   `db:"type" json:"type"`
	Amount    float64           `db:"amount" json:"amount"`
	Fee       float64           `db:"fee" json:"fee"`
	Status    TransactionStatus `db:"status" json:"status"`
	TxHash    string            `db:"tx_hash" json:"tx_hash,omitempty"`
	CreatedAt time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt time.Time         `db:"updated_at" json:"updated_at"`
}
