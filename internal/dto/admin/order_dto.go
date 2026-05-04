package admin

import (
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// ExchangeDTO represents a currency exchange with all details for admin view
type ExchangeDTO struct {
	ID              int64                         `json:"id"`
	UID             string                        `json:"uid"`
	UserID          int64                         `json:"user_id"`
	Email           string                        `json:"email,omitempty"`
	FromCurrencyID  int32                         `json:"from_currency_id"`
	ToCurrencyID    int32                         `json:"to_currency_id"`
	FromCurrency    CurrencyDTO                   `json:"from_currency"`
	ToCurrency      CurrencyDTO                   `json:"to_currency"`
	FromAmount      decimal.Decimal               `json:"from_amount"`
	ToAmount        decimal.Decimal               `json:"to_amount"`
	ToAmountWithFee decimal.Decimal               `json:"to_amount_with_fee"`
	ExchangeRate    decimal.Decimal               `json:"exchange_rate"`
	Fee             decimal.Decimal               `json:"fee"`
	Status          domain.CurrencyExchangeStatus `json:"status"`
	CreatedAt       time.Time                     `json:"created_at"`
	UpdatedAt       time.Time                     `json:"updated_at"`
}

// CurrencyDTO represents currency information
type CurrencyDTO struct {
	ID       int32  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	IsActive bool   `json:"is_active"`
	IsCrypto bool   `json:"is_crypto"`
}

// ListExchangesResponse represents the response for listing exchanges
type ListExchangesResponse struct {
	Exchanges []ExchangeDTO `json:"exchanges"`
	Total     int64         `json:"total"`
}
