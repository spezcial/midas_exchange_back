package domain

import (
	"time"
)

type CurrencyExchangeStatus string

const (
	CurrencyExchangeStatusPending   CurrencyExchangeStatus = "pending"
	CurrencyExchangeStatusCompleted CurrencyExchangeStatus = "completed"
	CurrencyExchangeStatusCanceled  CurrencyExchangeStatus = "canceled"
)

type CurrencyExchange struct {
	ID               int64                  `db:"id" json:"id"`
	UID              string                 `db:"uid" json:"uid"`
	UserID           int64                  `db:"user_id" json:"user_id"`
	Email            string                 `db:"email" json:"email,omitempty"`
	FromCurrencyID   int32                  `db:"from_currency_id" json:"from_currency_id"`
	ToCurrencyID     int32                  `db:"to_currency_id" json:"to_currency_id"`
	FromAmount       float64                `db:"from_amount" json:"from_amount"`
	ToAmount         float64                `db:"to_amount" json:"to_amount"`
	ToAmountWithFee  float64                `db:"to_amount_with_fee" json:"to_amount_with_fee"`
	ExchangeRate     float64                `db:"exchange_rate" json:"exchange_rate"`
	Fee              float64                `db:"fee" json:"fee"`
	Status           CurrencyExchangeStatus `db:"status" json:"status"`
	CreatedAt        time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time              `db:"updated_at" json:"updated_at"`
}

type CurrencyExchangeWithDetails struct {
	CurrencyExchange
	FromCurrency Currency `json:"from_currency"`
	ToCurrency   Currency `json:"to_currency"`
	User         User     `json:"user,omitempty"`
}

// CurrencyExchangeWithCurrencies represents a currency exchange with embedded currency information
type CurrencyExchangeWithCurrencies struct {
	ID              int64                  `db:"id"`
	UID             string                 `db:"uid"`
	UserID          int64                  `db:"user_id"`
	Email           string                 `db:"email"`
	FromCurrencyID  int32                  `db:"from_currency_id"`
	ToCurrencyID    int32                  `db:"to_currency_id"`
	FromAmount      float64                `db:"from_amount"`
	ToAmount        float64                `db:"to_amount"`
	ToAmountWithFee float64                `db:"to_amount_with_fee"`
	ExchangeRate    float64                `db:"exchange_rate"`
	Fee             float64                `db:"fee"`
	Status          CurrencyExchangeStatus `db:"status"`
	CreatedAt       time.Time              `db:"created_at"`
	UpdatedAt       time.Time              `db:"updated_at"`
	FromCurrency    Currency               `db:"from_currency"`
	ToCurrency      Currency               `db:"to_currency"`
}

// ToCurrencyExchange converts CurrencyExchangeWithCurrencies to CurrencyExchange for update operations
func (c *CurrencyExchangeWithCurrencies) ToCurrencyExchange() *CurrencyExchange {
	return &CurrencyExchange{
		ID:              c.ID,
		UID:             c.UID,
		UserID:          c.UserID,
		FromCurrencyID:  c.FromCurrencyID,
		ToCurrencyID:    c.ToCurrencyID,
		FromAmount:      c.FromAmount,
		ToAmount:        c.ToAmount,
		ToAmountWithFee: c.ToAmountWithFee,
		ExchangeRate:    c.ExchangeRate,
		Fee:             c.Fee,
		Status:          c.Status,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

type ExchangeRate struct {
	ID             int64     `db:"id" json:"id"`
	FromCurrencyID int32     `db:"from_currency_id" json:"from_currency_id"`
	ToCurrencyID   int32     `db:"to_currency_id" json:"to_currency_id"`
	Rate           float64   `db:"rate" json:"rate"`
	Fee            float64   `db:"fee" json:"fee"`
	IsActive       bool      `db:"is_active" json:"is_active"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

type ExchangeRateWithCurrencies struct {
	ExchangeRate
	FromCurrency Currency `db:"from_currency" json:"from_currency"`
	ToCurrency   Currency `db:"to_currency" json:"to_currency"`
}
