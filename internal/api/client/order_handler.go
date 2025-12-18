package client

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	clientdto "github.com/caspianex/exchange-backend/internal/dto/client"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
	"github.com/go-chi/chi/v5"
)

type ExchangeHandler struct {
	exchangeService *service.CurrencyExchangeService
}

func NewExchangeHandler(exchangeService *service.CurrencyExchangeService) *ExchangeHandler {
	return &ExchangeHandler{
		exchangeService: exchangeService,
	}
}

func (h *ExchangeHandler) CreateExchange(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.CreateExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	exchange, err := h.exchangeService.CreateExchange(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, exchange)
}

func (h *ExchangeHandler) GetExchanges(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	exchanges, err := h.exchangeService.GetUserExchanges(r.Context(), userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := h.exchangeService.GetUserExchangesCount(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, clientdto.ListExchangesResponse{
		Exchanges: clientdto.ToExchangeDTOList(exchanges),
		Total:     count,
	})
}

func (h *ExchangeHandler) GetExchange(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	exchangeIDStr := chi.URLParam(r, "id")
	exchangeID, err := strconv.ParseInt(exchangeIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid exchange ID")
		return
	}

	exchange, err := h.exchangeService.GetUserExchangeByID(r.Context(), userID, exchangeID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, clientdto.ToExchangeDTO(*exchange))
}

func (h *ExchangeHandler) CancelExchange(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	exchangeIDStr := chi.URLParam(r, "id")
	exchangeID, err := strconv.ParseInt(exchangeIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid exchange ID")
		return
	}

	if err := h.exchangeService.CancelExchange(r.Context(), userID, exchangeID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Exchange canceled successfully"})
}
