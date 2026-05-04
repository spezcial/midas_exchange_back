package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/shopspring/decimal"
)

// OTCBroadcaster pushes new chat messages to connected WebSocket clients.
// Implemented by OTCHub in cmd/server; handlers call it after each successful send.
type OTCBroadcaster interface {
	Broadcast(orderUID string, msg interface{})
}

// otcOrderRepo is the subset of OTCRepository used by OTCService.
type otcOrderRepo interface {
	Create(ctx context.Context, order *domain.OTCOrder) error
	GetByUID(ctx context.Context, uid string) (*domain.OTCOrderDetail, error)
	GetConfigs(ctx context.Context) ([]domain.OTCConfig, error)
	GetConfigByID(ctx context.Context, id int64) (*domain.OTCConfig, error)
	GetConfigByPair(ctx context.Context, fromCurrencyID, toCurrencyID int64) (*domain.OTCConfig, error)
	GetActiveConfigsWithCurrencies(ctx context.Context) ([]domain.OTCConfigWithCurrencies, error)
	GetAllConfigsWithCurrencies(ctx context.Context) ([]domain.OTCConfigWithCurrencies, error)
	CreateConfig(ctx context.Context, config *domain.OTCConfig) error
	UpdateConfig(ctx context.Context, config *domain.OTCConfig) error
	DeleteConfig(ctx context.Context, id int64) error
	ListByUser(ctx context.Context, userID int64, limit, offset int, status string) ([]domain.OTCOrder, int64, error)
	ListAll(ctx context.Context, limit, offset int, filter domain.OTCListFilter) ([]domain.OTCOrder, int64, error)
	ListAllForExport(ctx context.Context, filter domain.OTCListFilter) ([]domain.OTCOrderExportRow, error)
	Take(ctx context.Context, orderID, operatorID int64) error
	CreateMessage(ctx context.Context, msg *domain.OTCMessage) error
	GetMessageByID(ctx context.Context, msgID int64) (*domain.OTCMessage, error)
	UpdateOfferStatus(ctx context.Context, msgID int64, status domain.OTCOfferStatus) error
	MarkMessagesRead(ctx context.Context, orderID, readerID int64) error
	Agree(ctx context.Context, orderID int64, rate, fromAmt, toAmt decimal.Decimal, deadline time.Time) error
	Cancel(ctx context.Context, orderID int64, reason, cancelledBy string) error
	SetPaymentReceived(ctx context.Context, orderID int64) error
	CompleteOrderAtomic(ctx context.Context, orderID, fromWalletID, toWalletID, userID int64, fromAmount, agreedFromAmount, toAmount decimal.Decimal, orderUID string) error
	Expire(ctx context.Context, orderID int64) (bool, error)
	CreateAuditLog(ctx context.Context, entry *domain.OTCAuditLog) error
	GetAuditLogs(ctx context.Context, orderID int64) ([]domain.OTCAuditLog, error)
	GetAnalytics(ctx context.Context, fromDate, toDate time.Time, granularity string) (*domain.OTCAnalytics, error)
}

// otcUserRepo is the subset of UserRepository used by OTCService.
type otcUserRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

// otcWalletRepo is the subset of WalletRepository used by OTCService.
type otcWalletRepo interface {
	GetByUserAndCurrency(ctx context.Context, userID int64, currencyID int32) (*domain.Wallet, error)
	LockAmount(ctx context.Context, walletID int64, amount decimal.Decimal) error
	UnlockAmount(ctx context.Context, walletID int64, amount decimal.Decimal) error
	RefreshWalletCache(ctx context.Context, walletID int64) error
}

type OTCService struct {
	otcRepo    otcOrderRepo
	userRepo   otcUserRepo
	walletRepo otcWalletRepo
	notif      NotificationService
}

func NewOTCService(
	otcRepo *repository.OTCRepository,
	userRepo *repository.UserRepository,
	walletRepo *repository.WalletRepository,
	notif NotificationService,
) *OTCService {
	return &OTCService{
		otcRepo:    otcRepo,
		userRepo:   userRepo,
		walletRepo: walletRepo,
		notif:      notif,
	}
}

func (s *OTCService) GetActiveConfigs(ctx context.Context) ([]domain.OTCConfigWithCurrencies, error) {
	return s.otcRepo.GetActiveConfigsWithCurrencies(ctx)
}

func (s *OTCService) GetAllConfigsWithCurrencies(ctx context.Context) ([]domain.OTCConfigWithCurrencies, error) {
	return s.otcRepo.GetAllConfigsWithCurrencies(ctx)
}

func (s *OTCService) CreateOrder(ctx context.Context, userID int64, fromCurrencyID, toCurrencyID int64, fromAmount, proposedRate decimal.Decimal, comment *string) (*domain.OTCOrder, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.KycLevel < 2 {
		return nil, fmt.Errorf("KYC level 2 or higher required to create an OTC order")
	}

	// Validate currency pair against otc_config
	cfg, err := s.otcRepo.GetConfigByPair(ctx, fromCurrencyID, toCurrencyID)
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.IsActive {
		return nil, fmt.Errorf("this currency pair is not available for OTC trading")
	}
	if fromAmount.LessThan(cfg.MinFromAmount) {
		return nil, fmt.Errorf("minimum order amount for this pair is %s", cfg.MinFromAmount.StringFixed(2))
	}

	// Verify from-wallet exists and has sufficient balance before locking
	fromWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, userID, int32(fromCurrencyID))
	if err != nil {
		return nil, fmt.Errorf("from-wallet not found: %w", err)
	}
	if fromWallet.Balance.LessThan(fromAmount) {
		return nil, fmt.Errorf("insufficient balance to place OTC order")
	}

	// Lock funds first; if order creation fails we unlock
	if err := s.walletRepo.LockAmount(ctx, fromWallet.ID, fromAmount); err != nil {
		return nil, fmt.Errorf("failed to lock funds: %w", err)
	}

	order := &domain.OTCOrder{
		UserID:         userID,
		FromCurrencyID: fromCurrencyID,
		ToCurrencyID:   toCurrencyID,
		FromAmount:     fromAmount,
		ProposedRate:   proposedRate,
		Comment:        comment,
	}

	if err := s.otcRepo.Create(ctx, order); err != nil {
		_ = s.walletRepo.UnlockAmount(ctx, fromWallet.ID, fromAmount)
		return nil, err
	}

	go func() {
		s.notif.NotifyNewOTCOrder(context.Background(), OTCNotificationPayload{
			OrderUID:     order.UID,
			ClientEmail:  user.Email,
			ClientName:   user.FirstName + " " + user.LastName,
			FromAmount:   fromAmount,
			ProposedRate: proposedRate,
			Status:       string(domain.OTCStatusAwaitingReview),
		})
	}()

	return order, nil
}

func (s *OTCService) GetConfigs(ctx context.Context) ([]domain.OTCConfig, error) {
	return s.otcRepo.GetConfigs(ctx)
}

// GetOrderOwner returns the userID that owns the order without triggering any
// side-effects (no lazy expiry, no mark-as-read). Used by client handlers to
// verify ownership before calling GetOrder.
func (s *OTCService) GetOrderOwner(ctx context.Context, uid string) (int64, error) {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return 0, err
	}
	return order.UserID, nil
}

// GetOrder loads the order by UID, applies lazy expiry, computes unread message
// count for callerID, and marks those messages as read.
func (s *OTCService) GetOrder(ctx context.Context, uid string, callerID int64) (*domain.OTCOrderDetail, error) {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}

	// Lazy expiry: if deadline passed and still in awaiting_payment, expire now.
	// The Expire query is idempotent (AND status='awaiting_payment'), so only the
	// first caller will get wasExpired=true and perform the wallet unlock.
	if order.Status == domain.OTCStatusAwaitingPayment &&
		order.PaymentDeadline != nil &&
		order.PaymentDeadline.Before(time.Now()) {

		wasExpired, _ := s.otcRepo.Expire(ctx, order.ID)
		order.Status = domain.OTCStatusExpired

		if wasExpired {
			fromWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, order.UserID, int32(order.FromCurrencyID))
			if err == nil {
				_ = s.walletRepo.UnlockAmount(ctx, fromWallet.ID, order.FromAmount)
			}
		}
	}

	// Count unread messages (from the other party) before marking them read.
	var unread int
	for _, msg := range order.Messages {
		if msg.SenderID != callerID && !msg.IsRead {
			unread++
		}
	}
	order.UnreadCount = unread

	// Mark messages from the other party as read.
	_ = s.otcRepo.MarkMessagesRead(ctx, order.ID, callerID)

	return order, nil
}

func (s *OTCService) ListByUser(ctx context.Context, userID int64, limit, offset int, status string) ([]domain.OTCOrder, int64, error) {
	return s.otcRepo.ListByUser(ctx, userID, limit, offset, status)
}

func (s *OTCService) ListAll(ctx context.Context, limit, offset int, filter domain.OTCListFilter) ([]domain.OTCOrder, int64, error) {
	return s.otcRepo.ListAll(ctx, limit, offset, filter)
}

func (s *OTCService) GetAuditLogs(ctx context.Context, uid string) ([]domain.OTCAuditLog, error) {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	return s.otcRepo.GetAuditLogs(ctx, order.ID)
}

func (s *OTCService) GetAnalytics(ctx context.Context, fromDate, toDate time.Time, granularity string) (*domain.OTCAnalytics, error) {
	return s.otcRepo.GetAnalytics(ctx, fromDate, toDate, granularity)
}

func (s *OTCService) ExportOrdersCSV(ctx context.Context, filter domain.OTCListFilter, w io.Writer) error {
	rows, err := s.otcRepo.ListAllForExport(ctx, filter)
	if err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	headers := []string{
		"UID", "Status", "Client Email", "Operator Email",
		"From Currency", "To Currency",
		"From Amount", "Proposed Rate", "Agreed Rate", "Agreed From Amount", "To Amount",
		"Cancel Reason", "Created At", "Updated At",
	}
	if err := cw.Write(headers); err != nil {
		return err
	}

	for _, r := range rows {
		operatorEmail := ""
		if r.OperatorEmail != nil {
			operatorEmail = *r.OperatorEmail
		}
		agreedRate := ""
		if r.AgreedRate != nil {
			agreedRate = r.AgreedRate.StringFixed(8)
		}
		agreedFromAmount := ""
		if r.AgreedFromAmount != nil {
			agreedFromAmount = r.AgreedFromAmount.StringFixed(8)
		}
		toAmount := ""
		if r.ToAmount != nil {
			toAmount = r.ToAmount.StringFixed(8)
		}
		cancelReason := ""
		if r.CancelReason != nil {
			cancelReason = *r.CancelReason
		}
		record := []string{
			r.UID,
			r.Status,
			r.ClientEmail,
			operatorEmail,
			r.FromCurrencyCode,
			r.ToCurrencyCode,
			r.FromAmount.StringFixed(8),
			r.ProposedRate.StringFixed(8),
			agreedRate,
			agreedFromAmount,
			toAmount,
			cancelReason,
			r.CreatedAt.UTC().Format(time.RFC3339),
			r.UpdatedAt.UTC().Format(time.RFC3339),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (s *OTCService) TakeOrder(ctx context.Context, uid string, operatorID int64) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusAwaitingReview {
		return fmt.Errorf("order must be in awaiting_review status to take")
	}
	if err := s.otcRepo.Take(ctx, order.ID, operatorID); err != nil {
		return err
	}
	user, _ := s.userRepo.GetByID(ctx, operatorID)
	role := "operator"
	if user != nil {
		role = string(user.Role)
	}
	s.logAudit(order.ID, operatorID, role, domain.OTCAuditActionTookOrder, nil)
	return nil
}

func (s *OTCService) AcceptAsProposed(ctx context.Context, uid string, operatorID int64, operatorRole string) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusNegotiating {
		return fmt.Errorf("order must be negotiating to accept the proposed rate")
	}
	if order.OperatorID == nil || *order.OperatorID != operatorID {
		return fmt.Errorf("only the assigned operator can accept the proposed rate")
	}

	toAmount := order.FromAmount.Mul(order.ProposedRate)
	deadline := time.Now().Add(30 * time.Minute)
	if err := s.otcRepo.Agree(ctx, order.ID, order.ProposedRate, order.FromAmount, toAmount, deadline); err != nil {
		return err
	}
	s.logAudit(order.ID, operatorID, operatorRole, domain.OTCAuditActionAcceptedOffer,
		map[string]interface{}{"source": "proposed_rate", "rate": order.ProposedRate})
	return nil
}

func (s *OTCService) SendMessage(ctx context.Context, uid string, senderID int64, senderRole, content string) (*domain.OTCMessage, error) {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.OTCStatusNegotiating && order.Status != domain.OTCStatusAwaitingPayment {
		return nil, fmt.Errorf("messages can only be sent while order is negotiating or awaiting payment")
	}
	if !s.isOrderParticipant(order, senderID) {
		return nil, fmt.Errorf("not authorized to message on this order")
	}

	msg := &domain.OTCMessage{
		OrderID:     order.ID,
		SenderID:    senderID,
		SenderRole:  senderRole,
		MessageType: domain.OTCMessageTypeText,
		Content:     &content,
	}
	if err := s.otcRepo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *OTCService) SendOffer(ctx context.Context, uid string, senderID int64, senderRole string, offerRate, offerFromAmount decimal.Decimal) (*domain.OTCMessage, error) {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.OTCStatusNegotiating {
		return nil, fmt.Errorf("offers can only be sent while order is negotiating")
	}
	if !s.isOrderParticipant(order, senderID) {
		return nil, fmt.Errorf("not authorized to send offers on this order")
	}

	offerToAmount := offerFromAmount.Mul(offerRate)
	offerStatus := domain.OTCOfferStatusPending

	msg := &domain.OTCMessage{
		OrderID:         order.ID,
		SenderID:        senderID,
		SenderRole:      senderRole,
		MessageType:     domain.OTCMessageTypeOffer,
		OfferRate:       &offerRate,
		OfferFromAmount: &offerFromAmount,
		OfferToAmount:   &offerToAmount,
		OfferStatus:     &offerStatus,
	}
	if err := s.otcRepo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}
	s.logAudit(order.ID, senderID, senderRole, domain.OTCAuditActionSentOffer,
		map[string]interface{}{"rate": offerRate, "from_amount": offerFromAmount, "to_amount": offerToAmount})
	return msg, nil
}

func (s *OTCService) AcceptOffer(ctx context.Context, uid string, messageID, acceptorID int64, acceptorRole string) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusNegotiating {
		return fmt.Errorf("order must be negotiating to accept an offer")
	}

	msg, err := s.otcRepo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.OrderID != order.ID {
		return fmt.Errorf("message does not belong to this order")
	}
	if msg.MessageType != domain.OTCMessageTypeOffer {
		return fmt.Errorf("message is not an offer")
	}
	if msg.OfferStatus == nil || *msg.OfferStatus != domain.OTCOfferStatusPending {
		return fmt.Errorf("offer is no longer pending")
	}
	if msg.SenderID == acceptorID {
		return fmt.Errorf("cannot accept your own offer")
	}

	if err := s.otcRepo.UpdateOfferStatus(ctx, messageID, domain.OTCOfferStatusAccepted); err != nil {
		return err
	}

	deadline := time.Now().Add(30 * time.Minute)
	if err := s.otcRepo.Agree(ctx, order.ID, *msg.OfferRate, *msg.OfferFromAmount, *msg.OfferToAmount, deadline); err != nil {
		return err
	}
	s.logAudit(order.ID, acceptorID, acceptorRole, domain.OTCAuditActionAcceptedOffer,
		map[string]interface{}{"message_id": messageID})
	return nil
}

func (s *OTCService) RejectOffer(ctx context.Context, uid string, messageID, rejectorID int64, rejectorRole string) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusNegotiating {
		return fmt.Errorf("order must be negotiating to reject an offer")
	}

	msg, err := s.otcRepo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.OrderID != order.ID {
		return fmt.Errorf("message does not belong to this order")
	}
	if msg.MessageType != domain.OTCMessageTypeOffer {
		return fmt.Errorf("message is not an offer")
	}
	if msg.OfferStatus == nil || *msg.OfferStatus != domain.OTCOfferStatusPending {
		return fmt.Errorf("offer is no longer pending")
	}
	if msg.SenderID == rejectorID {
		return fmt.Errorf("cannot reject your own offer")
	}

	if err := s.otcRepo.UpdateOfferStatus(ctx, messageID, domain.OTCOfferStatusRejected); err != nil {
		return err
	}
	s.logAudit(order.ID, rejectorID, rejectorRole, domain.OTCAuditActionRejectedOffer,
		map[string]interface{}{"message_id": messageID})
	return nil
}

func (s *OTCService) CancelOrder(ctx context.Context, uid string, callerID int64, callerRole, reason string) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}

	isClient := callerRole == string(domain.UserRoleClient)
	isOperatorOrAdmin := !isClient

	if isClient {
		if order.UserID != callerID {
			return fmt.Errorf("not authorized to cancel this order")
		}
		if order.Status != domain.OTCStatusAwaitingReview && order.Status != domain.OTCStatusNegotiating {
			return fmt.Errorf("order can only be cancelled from awaiting_review or negotiating status")
		}
	} else {
		// Operator/admin can cancel from any non-terminal status
		if isOperatorOrAdmin {
			terminal := order.Status == domain.OTCStatusCompleted ||
				order.Status == domain.OTCStatusCancelled ||
				order.Status == domain.OTCStatusExpired
			if terminal {
				return fmt.Errorf("order is already in a terminal status")
			}
		}
	}

	if err := s.otcRepo.Cancel(ctx, order.ID, reason, callerRole); err != nil {
		return err
	}

	// Best-effort unlock: release locked funds back to balance.
	// If this fails, funds remain locked but the cancel is still committed.
	fromWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, order.UserID, int32(order.FromCurrencyID))
	if err == nil {
		_ = s.walletRepo.UnlockAmount(ctx, fromWallet.ID, order.FromAmount)
	}
	s.logAudit(order.ID, callerID, callerRole, domain.OTCAuditActionCancelled,
		map[string]interface{}{"reason": reason})
	return nil
}

func (s *OTCService) ConfirmPaymentReceived(ctx context.Context, uid string, operatorID int64) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusAwaitingPayment {
		return fmt.Errorf("order must be in awaiting_payment status to confirm payment")
	}
	if order.OperatorID == nil || *order.OperatorID != operatorID {
		return fmt.Errorf("only the assigned operator can confirm payment")
	}
	if err := s.otcRepo.SetPaymentReceived(ctx, order.ID); err != nil {
		return err
	}
	s.logAudit(order.ID, operatorID, "operator", domain.OTCAuditActionConfirmedPayment, nil)
	return nil
}

func (s *OTCService) CompleteOrder(ctx context.Context, uid string, operatorID int64) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusPaymentReceived {
		return fmt.Errorf("order must be in payment_received status to complete")
	}
	if order.OperatorID == nil || *order.OperatorID != operatorID {
		return fmt.Errorf("only the assigned operator can complete the order")
	}
	if order.AgreedFromAmount == nil || order.ToAmount == nil {
		return fmt.Errorf("order has no agreed amounts, cannot adjust wallets")
	}

	fromWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, order.UserID, int32(order.FromCurrencyID))
	if err != nil {
		return fmt.Errorf("client from-wallet not found: %w", err)
	}
	toWallet, err := s.walletRepo.GetByUserAndCurrency(ctx, order.UserID, int32(order.ToCurrencyID))
	if err != nil {
		return fmt.Errorf("client to-wallet not found: %w", err)
	}

	// Single atomic DB transaction: finalize from-wallet lock, credit to-wallet,
	// insert both tx records, mark order completed. Rolls back automatically on any failure.
	if err := s.otcRepo.CompleteOrderAtomic(
		ctx,
		order.ID, fromWallet.ID, toWallet.ID, order.UserID,
		order.FromAmount, *order.AgreedFromAmount, *order.ToAmount,
		order.UID,
	); err != nil {
		return err
	}

	// Refresh wallet cache so subsequent reads reflect the committed state.
	_ = s.walletRepo.RefreshWalletCache(ctx, fromWallet.ID)
	_ = s.walletRepo.RefreshWalletCache(ctx, toWallet.ID)

	s.logAudit(order.ID, operatorID, "operator", domain.OTCAuditActionCompleted, nil)
	return nil
}

// --- Config ---

func (s *OTCService) CreateConfig(ctx context.Context, fromCurrencyID, toCurrencyID int64, minFromAmount decimal.Decimal, paymentTimeoutMin int, isActive bool) (*domain.OTCConfig, error) {
	if fromCurrencyID == toCurrencyID {
		return nil, fmt.Errorf("from_currency_id and to_currency_id must differ")
	}
	if minFromAmount.IsNegative() {
		return nil, fmt.Errorf("min_from_amount must be non-negative")
	}
	if paymentTimeoutMin <= 0 {
		return nil, fmt.Errorf("payment_timeout_min must be positive")
	}
	config := &domain.OTCConfig{
		FromCurrencyID:    fromCurrencyID,
		ToCurrencyID:      toCurrencyID,
		MinFromAmount:     minFromAmount,
		PaymentTimeoutMin: paymentTimeoutMin,
		IsActive:          isActive,
	}
	if err := s.otcRepo.CreateConfig(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *OTCService) UpdateConfig(ctx context.Context, id int64, minFromAmount decimal.Decimal, paymentTimeoutMin int, isActive bool) (*domain.OTCConfig, error) {
	config, err := s.otcRepo.GetConfigByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if minFromAmount.IsNegative() {
		return nil, fmt.Errorf("min_from_amount must be non-negative")
	}
	if paymentTimeoutMin <= 0 {
		return nil, fmt.Errorf("payment_timeout_min must be positive")
	}
	config.MinFromAmount = minFromAmount
	config.PaymentTimeoutMin = paymentTimeoutMin
	config.IsActive = isActive
	if err := s.otcRepo.UpdateConfig(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *OTCService) DeleteConfig(ctx context.Context, id int64) error {
	return s.otcRepo.DeleteConfig(ctx, id)
}

// logAudit writes an audit entry in the background. Failures are silently dropped
// so that a logging error never rolls back the primary operation.
func (s *OTCService) logAudit(orderID, actorID int64, actorRole, action string, details interface{}) {
	var detailsStr *string
	if details != nil {
		b, err := json.Marshal(details)
		if err == nil {
			s := string(b)
			detailsStr = &s
		}
	}
	entry := &domain.OTCAuditLog{
		OrderID:   orderID,
		ActorID:   actorID,
		ActorRole: actorRole,
		Action:    action,
		Details:   detailsStr,
	}
	go func() {
		_ = s.otcRepo.CreateAuditLog(context.Background(), entry)
	}()
}

func (s *OTCService) isOrderParticipant(order *domain.OTCOrderDetail, userID int64) bool {
	if order.UserID == userID {
		return true
	}
	if order.OperatorID != nil && *order.OperatorID == userID {
		return true
	}
	return false
}
