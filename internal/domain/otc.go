package domain

import "time"

type OTCStatus string

const (
	OTCStatusAwaitingReview  OTCStatus = "awaiting_review"
	OTCStatusNegotiating     OTCStatus = "negotiating"
	OTCStatusAwaitingPayment OTCStatus = "awaiting_payment"
	OTCStatusPaymentReceived OTCStatus = "payment_received"
	OTCStatusCompleted       OTCStatus = "completed"
	OTCStatusCancelled       OTCStatus = "cancelled"
	OTCStatusExpired         OTCStatus = "expired"
)

type OTCMessageType string

const (
	OTCMessageTypeText  OTCMessageType = "text"
	OTCMessageTypeOffer OTCMessageType = "offer"
)

type OTCOfferStatus string

const (
	OTCOfferStatusPending  OTCOfferStatus = "pending"
	OTCOfferStatusAccepted OTCOfferStatus = "accepted"
	OTCOfferStatusRejected OTCOfferStatus = "rejected"
)

type OTCOrder struct {
	ID               int64      `db:"id" json:"id"`
	UID              string     `db:"uid" json:"uid"`
	UserID           int64      `db:"user_id" json:"user_id"`
	OperatorID       *int64     `db:"operator_id" json:"operator_id"`
	FromCurrencyID   int64      `db:"from_currency_id" json:"from_currency_id"`
	ToCurrencyID     int64      `db:"to_currency_id" json:"to_currency_id"`
	FromAmount       float64    `db:"from_amount" json:"from_amount"`
	ProposedRate     float64    `db:"proposed_rate" json:"proposed_rate"`
	AgreedRate       *float64   `db:"agreed_rate" json:"agreed_rate"`
	AgreedFromAmount *float64   `db:"agreed_from_amount" json:"agreed_from_amount"`
	ToAmount         *float64   `db:"to_amount" json:"to_amount"`
	Status           OTCStatus  `db:"status" json:"status"`
	Comment          *string    `db:"comment" json:"comment"`
	CancelReason     *string    `db:"cancel_reason" json:"cancel_reason"`
	CancelledBy      *string    `db:"cancelled_by" json:"cancelled_by"`
	PaymentDeadline  *time.Time `db:"payment_deadline" json:"payment_deadline"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

type OTCMessage struct {
	ID              int64           `db:"id" json:"id"`
	OrderID         int64           `db:"order_id" json:"order_id"`
	SenderID        int64           `db:"sender_id" json:"sender_id"`
	SenderRole      string          `db:"sender_role" json:"sender_role"`
	MessageType     OTCMessageType  `db:"message_type" json:"message_type"`
	Content         *string         `db:"content" json:"content"`
	OfferRate       *float64        `db:"offer_rate" json:"offer_rate"`
	OfferFromAmount *float64        `db:"offer_from_amount" json:"offer_from_amount"`
	OfferToAmount   *float64        `db:"offer_to_amount" json:"offer_to_amount"`
	OfferStatus     *OTCOfferStatus `db:"offer_status" json:"offer_status"`
	IsRead          bool            `db:"is_read" json:"is_read"`
	ReadAt          *time.Time      `db:"read_at" json:"read_at"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
}

type OTCConfig struct {
	ID                int64     `db:"id" json:"id"`
	FromCurrencyID    int64     `db:"from_currency_id" json:"from_currency_id"`
	ToCurrencyID      int64     `db:"to_currency_id" json:"to_currency_id"`
	MinFromAmount     float64   `db:"min_from_amount" json:"min_from_amount"`
	PaymentTimeoutMin int       `db:"payment_timeout_min" json:"payment_timeout_min"`
	IsActive          bool      `db:"is_active" json:"is_active"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
}

type OTCOrderDetail struct {
	OTCOrder
	FromCurrency Currency     `db:"from_currency" json:"from_currency"`
	ToCurrency   Currency     `db:"to_currency" json:"to_currency"`
	Messages     []OTCMessage `json:"messages,omitempty"`
}
