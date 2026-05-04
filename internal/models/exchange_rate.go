package models

import "github.com/shopspring/decimal"

type CreateExchangeRatesRequest struct {
	FromCurrencyID int32           `json:"from_currency_id" validate:"required,gt=0"`
	ToCurrencyID   int32           `json:"to_currency_id" validate:"required,gt=0"`
	Fee            decimal.Decimal `json:"fee" validate:"gte=0"`
	Rate           decimal.Decimal `json:"rate" validate:"gte=0"`
	IsActive       bool            `json:"is_active"`
}

type UpdateExchangeRatesRequest struct {
	Fee      decimal.Decimal `json:"fee" validate:"gte=0"`
	IsActive bool            `json:"is_active"`
}
