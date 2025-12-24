package models

type DepositRequest struct {
	CurrencyCode string  `json:"currency_code" validate:"required"`
	Amount       float64 `json:"amount" validate:"required,gt=0"`
	TxHash       string  `json:"tx_hash"`
}

type WithdrawRequest struct {
	CurrencyCode string  `json:"currency_code" validate:"required"`
	Amount       float64 `json:"amount" validate:"required,gt=0"`
}

type AdminDepositRequest struct {
	UserID       int64   `json:"user_id" validate:"required,gt=0"`
	CurrencyCode string  `json:"currency_code" validate:"required"`
	Amount       float64 `json:"amount" validate:"required,gt=0"`
	TxHash       string  `json:"tx_hash"`
	Description  string  `json:"description"`
}

