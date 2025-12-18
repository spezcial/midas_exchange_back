package admin

import "github.com/caspianex/exchange-backend/internal/domain"

// ToExchangeDTO converts domain.CurrencyExchangeWithCurrencies to ExchangeDTO
func ToExchangeDTO(exchange *domain.CurrencyExchangeWithCurrencies) ExchangeDTO {
	if exchange == nil {
		return ExchangeDTO{}
	}

	return ExchangeDTO{
		ID:              exchange.ID,
		UID:             exchange.UID,
		UserID:          exchange.UserID,
		Email:           exchange.Email,
		FromCurrencyID:  exchange.FromCurrencyID,
		ToCurrencyID:    exchange.ToCurrencyID,
		FromCurrency:    ToCurrencyDTO(exchange.FromCurrency),
		ToCurrency:      ToCurrencyDTO(exchange.ToCurrency),
		FromAmount:      exchange.FromAmount,
		ToAmount:        exchange.ToAmount,
		ToAmountWithFee: exchange.ToAmountWithFee,
		ExchangeRate:    exchange.ExchangeRate,
		Fee:             exchange.Fee,
		Status:          exchange.Status,
		CreatedAt:       exchange.CreatedAt,
		UpdatedAt:       exchange.UpdatedAt,
	}
}

// ToExchangeDTOList converts a slice of domain.CurrencyExchangeWithCurrencies to []ExchangeDTO
func ToExchangeDTOList(exchanges []domain.CurrencyExchangeWithCurrencies) []ExchangeDTO {
	dtos := make([]ExchangeDTO, len(exchanges))
	for i, exchange := range exchanges {
		dtos[i] = ToExchangeDTO(&exchange)
	}
	return dtos
}

// ToCurrencyDTO converts domain.Currency to CurrencyDTO
func ToCurrencyDTO(currency domain.Currency) CurrencyDTO {
	return CurrencyDTO{
		ID:       currency.ID,
		Code:     currency.Code,
		Name:     currency.Name,
		Symbol:   currency.Symbol,
		IsActive: currency.IsActive,
		IsCrypto: currency.IsCrypto,
	}
}
