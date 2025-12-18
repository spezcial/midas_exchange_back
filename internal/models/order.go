package models

import "github.com/caspianex/exchange-backend/internal/domain"

type CreateExchangeRequest struct {
	FromCurrencyCode string  `json:"from_currency_code" validate:"required"`
	ToCurrencyCode   string  `json:"to_currency_code" validate:"required"`
	FromAmount       float64 `json:"from_amount" validate:"required,gt=0"`
}

type GetExchangeResponse struct {
	Exchanges []domain.CurrencyExchange `json:"exchanges"`
	Total     int64                     `json:"total"`
}

type ListAllExchangesResponse struct {
	Exchanges []domain.CurrencyExchange `json:"exchanges"`
	Total     int64                     `json:"total"`
}
