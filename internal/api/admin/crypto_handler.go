package admin

import (
	"net/http"

	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

type CryptoHandler struct {
	cgService *service.CryptoGateService
	logger    *logger.Logger
}

func NewCryptoHandler(cgService *service.CryptoGateService, logger *logger.Logger) *CryptoHandler {
	return &CryptoHandler{cgService: cgService, logger: logger}
}

// AuditAddresses compares our local deposit address table against the gateway.
// GET /admin/crypto/audit
func (h *CryptoHandler) AuditAddresses(w http.ResponseWriter, r *http.Request) {
	result, err := h.cgService.AuditAddresses(r.Context())
	if err != nil {
		h.logger.Error("crypto address audit failed", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
