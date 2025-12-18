package models

type CreateExchangeRatesRequest struct {
	FromCurrencyID int32   `json:"from_currency_id" validate:"required,gt=0"`
	ToCurrencyID   int32   `json:"to_currency_id" validate:"required,gt=0"`
	Fee            float64 `json:"fee" validate:"gte=0"`
	Rate           float64 `json:"rate" validate:"gte=0"`
	IsActive       bool    `json:"is_active"`
}

type UpdateExchangeRatesRequest struct {
	Fee      float64 `json:"fee" validate:"gte=0"`
	IsActive bool    `json:"is_active"`
}
