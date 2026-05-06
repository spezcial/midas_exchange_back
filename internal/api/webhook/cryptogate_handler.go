package webhook

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

type CryptoGateHandler struct {
	cgService     *service.CryptoGateService
	webhookSecret string
	log           *logger.Logger
}

func NewCryptoGateHandler(cgService *service.CryptoGateService, webhookSecret string, log *logger.Logger) *CryptoGateHandler {
	return &CryptoGateHandler{
		cgService:     cgService,
		webhookSecret: webhookSecret,
		log:           log,
	}
}

func (h *CryptoGateHandler) verifySecret(r *http.Request) bool {
	if h.webhookSecret == "" {
		return false // fail-closed: empty secret means rejecting all webhooks
	}
	got := []byte(r.Header.Get("X-TOKEN"))
	want := []byte(h.webhookSecret)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// HandleDeposit handles POST /cg/deposit from crypto-gate.
func (h *CryptoGateHandler) HandleDeposit(w http.ResponseWriter, r *http.Request) {
	if !h.verifySecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload domain.DepositWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.log.Warn("deposit webhook: invalid JSON", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	h.log.Info("deposit webhook received", "address", payload.Address, "amount", payload.Amount, "asset", payload.Asset, "hash", payload.Hash)

	if err := h.cgService.HandleDepositWebhook(r.Context(), payload); err != nil {
		h.log.Error("deposit webhook: processing error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleWithdraw handles POST /cg/withdraw from crypto-gate.
func (h *CryptoGateHandler) HandleWithdraw(w http.ResponseWriter, r *http.Request) {
	if !h.verifySecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload domain.WithdrawWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.log.Warn("withdraw webhook: invalid JSON", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	h.log.Info("withdraw webhook received", "uuid", payload.UUID, "status", payload.Status, "hash", payload.Hash)

	if err := h.cgService.HandleWithdrawWebhook(r.Context(), payload); err != nil {
		h.log.Error("withdraw webhook: processing error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
