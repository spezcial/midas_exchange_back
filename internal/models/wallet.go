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

