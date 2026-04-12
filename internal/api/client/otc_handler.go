package client

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type OTCHandler struct {
	otcService  *service.OTCService
	broadcaster service.OTCBroadcaster
}

func NewOTCHandler(otcService *service.OTCService, broadcaster service.OTCBroadcaster) *OTCHandler {
	return &OTCHandler{otcService: otcService, broadcaster: broadcaster}
}

type createOTCOrderRequest struct {
	FromCurrencyID int64   `json:"from_currency_id"`
	ToCurrencyID   int64   `json:"to_currency_id"`
	FromAmount     float64 `json:"from_amount"`
	ProposedRate   float64 `json:"proposed_rate"`
	Comment        *string `json:"comment"`
}

func (h *OTCHandler) GetActiveConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.otcService.GetActiveConfigs(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, configs)
}

func (h *OTCHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createOTCOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FromCurrencyID == 0 || req.ToCurrencyID == 0 || req.FromAmount <= 0 || req.ProposedRate <= 0 {
		respondError(w, http.StatusBadRequest, "from_currency_id, to_currency_id, from_amount, and proposed_rate are required")
		return
	}

	order, err := h.otcService.CreateOrder(r.Context(), userID, req.FromCurrencyID, req.ToCurrencyID, req.FromAmount, req.ProposedRate, req.Comment)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

func (h *OTCHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
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
	status := r.URL.Query().Get("status")

	orders, total, err := h.otcService.ListByUser(r.Context(), userID, limit, offset, status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
		"total":  total,
	})
}

func (h *OTCHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")

	// Peek at the order owner before triggering side-effects (lazy expiry, mark-read).
	// Without this check a client could enumerate UIDs to expire other users' orders.
	order, err := h.otcService.GetOrderOwner(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusNotFound, "order not found")
		return
	}
	if order != userID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	detail, err := h.otcService.GetOrder(r.Context(), uid, userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, detail)
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

func (h *OTCHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}

	msg, err := h.otcService.SendMessage(r.Context(), uid, userID, "client", req.Content)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, msg)
	}
	respondJSON(w, http.StatusCreated, msg)
}

type sendOfferRequest struct {
	OfferRate       float64 `json:"offer_rate"`
	OfferFromAmount float64 `json:"offer_from_amount"`
}

func (h *OTCHandler) SendOffer(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")

	var req sendOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.OfferRate <= 0 || req.OfferFromAmount <= 0 {
		respondError(w, http.StatusBadRequest, "offer_rate and offer_from_amount must be positive")
		return
	}

	msg, err := h.otcService.SendOffer(r.Context(), uid, userID, "client", req.OfferRate, req.OfferFromAmount)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, msg)
	}
	respondJSON(w, http.StatusCreated, msg)
}

func (h *OTCHandler) AcceptOffer(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	messageIDStr := chi.URLParam(r, "messageID")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	if err := h.otcService.AcceptOffer(r.Context(), uid, messageID, userID, "client"); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "offer accepted"})
}

func (h *OTCHandler) RejectOffer(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	messageIDStr := chi.URLParam(r, "messageID")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	if err := h.otcService.RejectOffer(r.Context(), uid, messageID, userID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "offer rejected"})
}

type cancelOrderRequest struct {
	Reason string `json:"reason"`
}

func (h *OTCHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")

	var req cancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.otcService.CancelOrder(r.Context(), uid, userID, "client", req.Reason); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "order cancelled"})
}
