package client

import (
	"errors"
	"net/http"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

type DepositAddressHandler struct {
	cgService *service.CryptoGateService
	log       *logger.Logger
}

func NewDepositAddressHandler(cgService *service.CryptoGateService, log *logger.Logger) *DepositAddressHandler {
	return &DepositAddressHandler{cgService: cgService, log: log}
}

// GetDepositAddress returns the user's deposit address for a given currency.
// Query param: currency (e.g. BTC, ETH, USDT)
func (h *DepositAddressHandler) GetDepositAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	currencyCode := r.URL.Query().Get("currency")
	if currencyCode == "" {
		respondError(w, http.StatusBadRequest, "currency query param required")
		return
	}

	addr, err := h.cgService.GetOrCreateDepositAddressByCode(r.Context(), userID, currencyCode)
	if err != nil {
		h.log.Error("get deposit address", "error", err, "user_id", userID, "currency", currencyCode)
		// Return 400 only for known user-facing errors (unsupported currency).
		// Everything else is an internal failure; don't leak details to the client.
		if errors.Is(err, service.ErrUnsupportedCurrency) {
			respondError(w, http.StatusBadRequest, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, "failed to get deposit address")
		}
		return
	}

	respondJSON(w, http.StatusOK, addr)
}
