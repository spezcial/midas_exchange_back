package client

import (
	"net/http"

	"github.com/caspianex/exchange-backend/internal/service"
)

type ExchangeRatesHandler struct {
	ratesService *service.ExchangeRatesService
}

func NewExchangePairHandler(ratesService *service.ExchangeRatesService) *ExchangeRatesHandler {
	return &ExchangeRatesHandler{
		ratesService: ratesService,
	}
}

// GetActivePairs returns all active exchange pairs (public endpoint)
func (h *ExchangeRatesHandler) GetActiveRates(w http.ResponseWriter, r *http.Request) {
	pairs, err := h.ratesService.GetActiveRates(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, pairs)
}
