package domain

import "time"

type FeeOperation string

const (
	FeeOperationExchange   FeeOperation = "exchange"
	FeeOperationWithdrawal FeeOperation = "withdrawal"
)

type PlatformFee struct {
	ID          int64        `db:"id"           json:"id"`
	UserID      int64        `db:"user_id"      json:"user_id"`
	Operation   FeeOperation `db:"operation"    json:"operation"`
	CurrencyID  int32        `db:"currency_id"  json:"currency_id"`
	GrossAmount float64      `db:"gross_amount" json:"gross_amount"` // full transaction amount before fee, in fee currency
	Fee         float64      `db:"fee"          json:"fee"`
	CreatedAt   time.Time    `db:"created_at"   json:"created_at"`
}

type PlatformFeeWithDetails struct {
	PlatformFee
	UserEmail      string `db:"user_email"      json:"user_email"`
	CurrencyCode   string `db:"currency_code"   json:"currency_code"`
	CurrencySymbol string `db:"currency_symbol" json:"currency_symbol"`
}
