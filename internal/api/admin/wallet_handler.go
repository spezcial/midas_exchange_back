package admin

import (
	"encoding/json"
	"net/http"

	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
)

type WalletHandler struct {
	walletService *service.WalletService
}

func NewWalletHandler(walletService *service.WalletService) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
	}
}

func (h *WalletHandler) ManualDeposit(w http.ResponseWriter, r *http.Request) {
	var req models.AdminDepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	depositReq := &models.DepositRequest{
		CurrencyCode: req.CurrencyCode,
		Amount:       req.Amount,
		TxHash:       req.TxHash,
	}

	tx, err := h.walletService.Deposit(r.Context(), req.UserID, depositReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, tx)
}
