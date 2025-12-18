package admin

import (
	"net/http"
	"strconv"

	admindto "github.com/caspianex/exchange-backend/internal/dto/admin"
	"github.com/caspianex/exchange-backend/internal/service"
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

func (h *ExchangeHandler) ListExchanges(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	status := r.URL.Query().Get("status")
	email := r.URL.Query().Get("email")

	exchanges, err := h.exchangeService.GetAllExchanges(r.Context(), status, email, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := h.exchangeService.GetAllExchangesCount(r.Context(), status, email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, admindto.ListExchangesResponse{
		Exchanges: admindto.ToExchangeDTOList(exchanges),
		Total:     total,
	})
}

func (h *ExchangeHandler) GetExchange(w http.ResponseWriter, r *http.Request) {
	exchangeIDStr := chi.URLParam(r, "id")
	exchangeID, err := strconv.ParseInt(exchangeIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid exchange ID")
		return
	}

	exchange, err := h.exchangeService.GetExchangeByID(r.Context(), exchangeID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, admindto.ToExchangeDTO(exchange))
	return
}
