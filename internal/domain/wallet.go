package domain

import (
	"time"
)

type Currency struct {
	ID        int32     `db:"id" json:"id"`
	Code      string    `db:"code" json:"code"`
	Name      string    `db:"name" json:"name"`
	Symbol    string    `db:"symbol" json:"symbol"`
	IsActive  bool      `db:"is_active" json:"is_active"`
	IsCrypto  bool      `db:"is_crypto" json:"is_crypto"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Wallet struct {
	ID         int64     `db:"id" json:"id"`
	UserID     int64     `db:"user_id" json:"user_id"`
	CurrencyID int32     `db:"currency_id" json:"currency_id"`
	Balance    float64   `db:"balance" json:"balance"`
	Locked     float64   `db:"locked" json:"locked"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type WalletWithCurrency struct {
	Wallet
	Currency Currency `json:"currency"`
}
