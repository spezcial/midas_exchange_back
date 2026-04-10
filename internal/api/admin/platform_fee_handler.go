package admin

import (
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/service"
)

type PlatformFeeHandler struct {
	feeService *service.PlatformFeeService
}

func NewPlatformFeeHandler(feeService *service.PlatformFeeService) *PlatformFeeHandler {
	return &PlatformFeeHandler{feeService: feeService}
}

func (h *PlatformFeeHandler) ListFees(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	fees, err := h.feeService.List(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := h.feeService.Count(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	totals, err := h.feeService.Totals(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"fees":   fees,
		"total":  total,
		"totals": totals,
	})
}
