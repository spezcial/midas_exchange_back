package service

import (
	"context"
	"fmt"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
)

type OTCService struct {
	otcRepo  *repository.OTCRepository
	userRepo *repository.UserRepository
	notif    NotificationService
}

func NewOTCService(otcRepo *repository.OTCRepository, userRepo *repository.UserRepository, notif NotificationService) *OTCService {
	return &OTCService{otcRepo: otcRepo, userRepo: userRepo, notif: notif}
}

func (s *OTCService) CreateOrder(ctx context.Context, userID int64, fromCurrencyID, toCurrencyID int64, fromAmount, proposedRate float64, comment *string) (*domain.OTCOrder, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.KycLevel < 2 {
		return nil, fmt.Errorf("KYC level 2 or higher required to create an OTC order")
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

func (s *OTCService) GetOrder(ctx context.Context, uid string) (*domain.OTCOrderDetail, error) {
	return s.otcRepo.GetByUID(ctx, uid)
}

func (s *OTCService) ListByUser(ctx context.Context, userID int64, limit, offset int, status string) ([]domain.OTCOrder, int64, error) {
	return s.otcRepo.ListByUser(ctx, userID, limit, offset, status)
}

func (s *OTCService) ListAll(ctx context.Context, limit, offset int, status, email string) ([]domain.OTCOrder, int64, error) {
	return s.otcRepo.ListAll(ctx, limit, offset, status, email)
}

func (s *OTCService) TakeOrder(ctx context.Context, uid string, operatorID int64) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusAwaitingReview {
		return fmt.Errorf("order must be in awaiting_review status to take")
	}
	return s.otcRepo.Take(ctx, order.ID, operatorID)
}

func (s *OTCService) SendMessage(ctx context.Context, uid string, senderID int64, senderRole, content string) (*domain.OTCMessage, error) {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.OTCStatusNegotiating {
		return nil, fmt.Errorf("messages can only be sent while order is negotiating")
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

func (s *OTCService) SendOffer(ctx context.Context, uid string, senderID int64, senderRole string, offerRate, offerFromAmount float64) (*domain.OTCMessage, error) {
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

	offerToAmount := offerFromAmount * offerRate
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
	return msg, nil
}

func (s *OTCService) AcceptOffer(ctx context.Context, uid string, messageID, acceptorID int64) error {
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
	return s.otcRepo.Agree(ctx, order.ID, *msg.OfferRate, *msg.OfferFromAmount, *msg.OfferToAmount, deadline)
}

func (s *OTCService) RejectOffer(ctx context.Context, uid string, messageID, rejectorID int64) error {
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

	return s.otcRepo.UpdateOfferStatus(ctx, messageID, domain.OTCOfferStatusRejected)
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

	return s.otcRepo.Cancel(ctx, order.ID, reason, callerRole)
}

func (s *OTCService) ConfirmPaymentReceived(ctx context.Context, uid string, operatorID int64) error {
	order, err := s.otcRepo.GetByUID(ctx, uid)
	if err != nil {
		return err
	}
	if order.Status != domain.OTCStatusAwaitingPayment && order.Status != domain.OTCStatusExpired {
		return fmt.Errorf("order must be in awaiting_payment or expired status")
	}
	if order.OperatorID == nil || *order.OperatorID != operatorID {
		return fmt.Errorf("only the assigned operator can confirm payment")
	}
	return s.otcRepo.SetPaymentReceived(ctx, order.ID)
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
	return s.otcRepo.Complete(ctx, order.ID)
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
