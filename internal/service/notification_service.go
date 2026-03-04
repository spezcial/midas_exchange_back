package service

import "context"

type OTCNotificationPayload struct {
	OrderUID     string
	ClientEmail  string
	ClientName   string
	FromCurrency string
	ToCurrency   string
	FromAmount   float64
	ProposedRate float64
	Status       string
}

type NotificationService interface {
	NotifyNewOTCOrder(ctx context.Context, p OTCNotificationPayload) error
	NotifyOTCStatusChanged(ctx context.Context, p OTCNotificationPayload) error
}

type NoOpNotificationService struct{}

func (n *NoOpNotificationService) NotifyNewOTCOrder(_ context.Context, _ OTCNotificationPayload) error {
	return nil
}

func (n *NoOpNotificationService) NotifyOTCStatusChanged(_ context.Context, _ OTCNotificationPayload) error {
	return nil
}
