package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
	"github.com/go-chi/chi/v5"
)

type ExchangeRatesHandler struct {
	ratesService *service.ExchangeRatesService
}

func NewExchangeRatesHandler(ratesService *service.ExchangeRatesService) *ExchangeRatesHandler {
	return &ExchangeRatesHandler{
		ratesService: ratesService,
	}
}

// GetAllRates returns all exchange Rates (admin only)
func (h *ExchangeRatesHandler) GetAllRates(w http.ResponseWriter, r *http.Request) {
	rates, err := h.ratesService.GetAllRates(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rates)
}

// GetRate returns a single exchange Rate by ID
func (h *ExchangeRatesHandler) GetRate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rate ID")
		return
	}

	rate, err := h.ratesService.GetRateByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rate)
}

// CreateRate creates a new exchange rate
func (h *ExchangeRatesHandler) CreateRate(w http.ResponseWriter, r *http.Request) {
	var req models.CreateExchangeRatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	rate, err := h.ratesService.CreateRate(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, rate)
}

// UpdateRate updates an existing exchange rate
func (h *ExchangeRatesHandler) UpdateRate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rate ID")
		return
	}

	var req models.UpdateExchangeRatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rate, err := h.ratesService.UpdateRate(r.Context(), id, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rate)
}

// DeleteRate deletes an exchange rate
func (h *ExchangeRatesHandler) DeleteRate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rate ID")
		return
	}

	if err := h.ratesService.DeleteRate(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Exchange rate deleted successfully"})
}
