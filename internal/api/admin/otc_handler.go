package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/go-chi/chi/v5"
)

type AdminOTCHandler struct {
	otcService  *service.OTCService
	logger      *logger.Logger
	broadcaster service.OTCBroadcaster
}

func NewOTCHandler(otcService *service.OTCService, logger *logger.Logger, broadcaster service.OTCBroadcaster) *AdminOTCHandler {
	return &AdminOTCHandler{otcService: otcService, logger: logger, broadcaster: broadcaster}
}

func (h *AdminOTCHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	configs, err := h.otcService.GetAllConfigsWithCurrencies(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"configs": configs,
	})
}

type createConfigRequest struct {
	FromCurrencyID    int64   `json:"from_currency_id"`
	ToCurrencyID      int64   `json:"to_currency_id"`
	MinFromAmount     float64 `json:"min_from_amount"`
	PaymentTimeoutMin int     `json:"payment_timeout_min"`
	IsActive          bool    `json:"is_active"`
}

func (h *AdminOTCHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var req createConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FromCurrencyID == 0 || req.ToCurrencyID == 0 {
		respondError(w, http.StatusBadRequest, "from_currency_id and to_currency_id are required")
		return
	}
	if req.PaymentTimeoutMin == 0 {
		req.PaymentTimeoutMin = 30 // default
	}

	config, err := h.otcService.CreateConfig(r.Context(), req.FromCurrencyID, req.ToCurrencyID, req.MinFromAmount, req.PaymentTimeoutMin, req.IsActive)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, config)
}

type updateConfigRequest struct {
	MinFromAmount     float64 `json:"min_from_amount"`
	PaymentTimeoutMin int     `json:"payment_timeout_min"`
	IsActive          bool    `json:"is_active"`
}

func (h *AdminOTCHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid config ID")
		return
	}

	var req updateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.PaymentTimeoutMin == 0 {
		req.PaymentTimeoutMin = 30
	}

	config, err := h.otcService.UpdateConfig(r.Context(), id, req.MinFromAmount, req.PaymentTimeoutMin, req.IsActive)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, config)
}

func (h *AdminOTCHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid config ID")
		return
	}

	if err := h.otcService.DeleteConfig(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "config deleted"})
}

func parseOTCFilter(r *http.Request) domain.OTCListFilter {
	f := domain.OTCListFilter{
		Status: r.URL.Query().Get("status"),
		Email:  r.URL.Query().Get("email"),
	}
	if s := r.URL.Query().Get("from_date"); s != "" {
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			f.FromDate = &t
		}
	}
	if s := r.URL.Query().Get("to_date"); s != "" {
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			t = t.AddDate(0, 0, 1) // exclusive upper bound: include the full day
			f.ToDate = &t
		}
	}
	if s := r.URL.Query().Get("from_currency_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			f.FromCurrencyID = &id
		}
	}
	if s := r.URL.Query().Get("to_currency_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			f.ToCurrencyID = &id
		}
	}
	if s := r.URL.Query().Get("operator_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			f.OperatorID = &id
		}
	}
	return f
}

func (h *AdminOTCHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	orders, total, err := h.otcService.ListAll(r.Context(), limit, offset, parseOTCFilter(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
		"total":  total,
	})
}

func (h *AdminOTCHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.GetUserID(r.Context())

	uid := chi.URLParam(r, "uid")
	order, err := h.otcService.GetOrder(r.Context(), uid, callerID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, order)
}

func (h *AdminOTCHandler) TakeOrder(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	if err := h.otcService.TakeOrder(r.Context(), uid, operatorID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "order taken"})
}

func (h *AdminOTCHandler) AcceptAsProposed(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	operatorRole, _ := middleware.GetUserRole(r.Context())

	uid := chi.URLParam(r, "uid")
	if err := h.otcService.AcceptAsProposed(r.Context(), uid, operatorID, operatorRole); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "proposed rate accepted"})
}

type adminSendMessageRequest struct {
	Content string `json:"content"`
}

func (h *AdminOTCHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	senderID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	senderRole, _ := middleware.GetUserRole(r.Context())

	uid := chi.URLParam(r, "uid")

	var req adminSendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}

	msg, err := h.otcService.SendMessage(r.Context(), uid, senderID, senderRole, req.Content)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, msg)
	}
	respondJSON(w, http.StatusCreated, msg)
}

type adminSendOfferRequest struct {
	OfferRate       float64 `json:"offer_rate"`
	OfferFromAmount float64 `json:"offer_from_amount"`
}

func (h *AdminOTCHandler) SendOffer(w http.ResponseWriter, r *http.Request) {
	senderID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	senderRole, _ := middleware.GetUserRole(r.Context())

	uid := chi.URLParam(r, "uid")

	var req adminSendOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.OfferRate <= 0 || req.OfferFromAmount <= 0 {
		respondError(w, http.StatusBadRequest, "offer_rate and offer_from_amount must be positive")
		return
	}

	msg, err := h.otcService.SendOffer(r.Context(), uid, senderID, senderRole, req.OfferRate, req.OfferFromAmount)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, msg)
	}
	respondJSON(w, http.StatusCreated, msg)
}

func (h *AdminOTCHandler) AcceptOffer(w http.ResponseWriter, r *http.Request) {
	acceptorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	messageIDStr := chi.URLParam(r, "id")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	acceptorRole, _ := middleware.GetUserRole(r.Context())
	if err := h.otcService.AcceptOffer(r.Context(), uid, messageID, acceptorID, acceptorRole); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "offer accepted"})
}

func (h *AdminOTCHandler) RejectOffer(w http.ResponseWriter, r *http.Request) {
	rejectorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	messageIDStr := chi.URLParam(r, "id")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	if err := h.otcService.RejectOffer(r.Context(), uid, messageID, rejectorID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "offer rejected"})
}

func (h *AdminOTCHandler) ConfirmPaymentReceived(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	if err := h.otcService.ConfirmPaymentReceived(r.Context(), uid, operatorID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "payment received confirmed"})
}

func (h *AdminOTCHandler) CompleteOrder(w http.ResponseWriter, r *http.Request) {
	operatorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	uid := chi.URLParam(r, "uid")
	if err := h.otcService.CompleteOrder(r.Context(), uid, operatorID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "order completed"})
}

type adminCancelOrderRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminOTCHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	callerRole, _ := middleware.GetUserRole(r.Context())

	uid := chi.URLParam(r, "uid")

	var req adminCancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.otcService.CancelOrder(r.Context(), uid, callerID, callerRole, req.Reason); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.broadcaster != nil {
		h.broadcaster.Broadcast(uid, map[string]string{"type": "order_update"})
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "order cancelled"})
}

// GetAuditLog returns the operator action log for a specific order.
// GET /admin/otc/orders/{uid}/audit-log
func (h *AdminOTCHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	logs, err := h.otcService.GetAuditLogs(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"audit_log": logs})
}

// GetAnalytics returns aggregate analytics for OTC orders.
// GET /admin/otc/analytics?from=YYYY-MM-DD&to=YYYY-MM-DD&granularity=day|week|month
func (h *AdminOTCHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	fromDate := time.Now().AddDate(0, -1, 0) // default: last 30 days
	toDate := time.Now().AddDate(0, 0, 1)    // exclusive: tomorrow

	if s := q.Get("from"); s != "" {
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			fromDate = t
		}
	}
	if s := q.Get("to"); s != "" {
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			toDate = t.AddDate(0, 0, 1)
		}
	}

	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	analytics, err := h.otcService.GetAnalytics(r.Context(), fromDate, toDate, granularity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, analytics)
}

// ExportOrders streams a CSV file with OTC order history.
// GET /admin/otc/orders/export?status=&email=&from_date=&to_date=&from_currency_id=&to_currency_id=
func (h *AdminOTCHandler) ExportOrders(w http.ResponseWriter, r *http.Request) {
	filter := parseOTCFilter(r)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"otc_orders.csv\"")

	if err := h.otcService.ExportOrdersCSV(r.Context(), filter, w); err != nil {
		h.logger.Error("OTC CSV export failed", "error", err)
	}
}
