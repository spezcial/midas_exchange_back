package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Audit log action constants
const (
	OTCAuditActionTookOrder        = "took_order"
	OTCAuditActionSentOffer        = "sent_offer"
	OTCAuditActionAcceptedOffer    = "accepted_offer"
	OTCAuditActionRejectedOffer    = "rejected_offer"
	OTCAuditActionConfirmedPayment = "confirmed_payment"
	OTCAuditActionCompleted        = "completed"
	OTCAuditActionCancelled        = "cancelled"
)

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
	ID               int64            `db:"id" json:"id"`
	UID              string           `db:"uid" json:"uid"`
	UserID           int64            `db:"user_id" json:"user_id"`
	OperatorID       *int64           `db:"operator_id" json:"operator_id"`
	FromCurrencyID   int64            `db:"from_currency_id" json:"from_currency_id"`
	ToCurrencyID     int64            `db:"to_currency_id" json:"to_currency_id"`
	FromCurrencyCode string           `db:"from_currency_code" json:"from_currency_code"`
	ToCurrencyCode   string           `db:"to_currency_code" json:"to_currency_code"`
	FromAmount       decimal.Decimal  `db:"from_amount" json:"from_amount"`
	ProposedRate     decimal.Decimal  `db:"proposed_rate" json:"proposed_rate"`
	AgreedRate       *decimal.Decimal `db:"agreed_rate" json:"agreed_rate"`
	AgreedFromAmount *decimal.Decimal `db:"agreed_from_amount" json:"agreed_from_amount"`
	ToAmount         *decimal.Decimal `db:"to_amount" json:"to_amount"`
	Status           OTCStatus        `db:"status" json:"status"`
	Comment          *string          `db:"comment" json:"comment"`
	CancelReason     *string          `db:"cancel_reason" json:"cancel_reason"`
	CancelledBy      *string          `db:"cancelled_by" json:"cancelled_by"`
	PaymentDeadline  *time.Time       `db:"payment_deadline" json:"payment_deadline"`
	CreatedAt        time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time        `db:"updated_at" json:"updated_at"`
	UnreadCount      int              `db:"unread_count" json:"unread_count"`
}

type OTCMessage struct {
	ID              int64            `db:"id" json:"id"`
	OrderID         int64            `db:"order_id" json:"order_id"`
	SenderID        int64            `db:"sender_id" json:"sender_id"`
	SenderRole      string           `db:"sender_role" json:"sender_role"`
	MessageType     OTCMessageType   `db:"message_type" json:"message_type"`
	Content         *string          `db:"content" json:"content"`
	OfferRate       *decimal.Decimal `db:"offer_rate" json:"offer_rate"`
	OfferFromAmount *decimal.Decimal `db:"offer_from_amount" json:"offer_from_amount"`
	OfferToAmount   *decimal.Decimal `db:"offer_to_amount" json:"offer_to_amount"`
	OfferStatus     *OTCOfferStatus  `db:"offer_status" json:"offer_status"`
	IsRead          bool             `db:"is_read" json:"is_read"`
	ReadAt          *time.Time       `db:"read_at" json:"read_at"`
	CreatedAt       time.Time        `db:"created_at" json:"created_at"`
}

type OTCConfigWithCurrencies struct {
	OTCConfig
	FromCurrency Currency `db:"from_currency" json:"from_currency"`
	ToCurrency   Currency `db:"to_currency" json:"to_currency"`
}

type OTCConfig struct {
	ID                int64           `db:"id" json:"id"`
	FromCurrencyID    int64           `db:"from_currency_id" json:"from_currency_id"`
	ToCurrencyID      int64           `db:"to_currency_id" json:"to_currency_id"`
	MinFromAmount     decimal.Decimal `db:"min_from_amount" json:"min_from_amount"`
	PaymentTimeoutMin int             `db:"payment_timeout_min" json:"payment_timeout_min"`
	IsActive          bool            `db:"is_active" json:"is_active"`
	CreatedAt         time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at" json:"updated_at"`
}

type OTCOrderDetail struct {
	OTCOrder
	FromCurrency Currency     `db:"from_currency" json:"from_currency"`
	ToCurrency   Currency     `db:"to_currency" json:"to_currency"`
	Messages     []OTCMessage `json:"messages,omitempty"`
}

// OTCListFilter holds optional filters for the admin order list / export.
type OTCListFilter struct {
	Status         string
	Email          string
	FromDate       *time.Time
	ToDate         *time.Time
	FromCurrencyID *int64
	ToCurrencyID   *int64
	OperatorID     *int64
}

// OTCAuditLog records a single operator action on an OTC order.
type OTCAuditLog struct {
	ID        int64     `db:"id"         json:"id"`
	OrderID   int64     `db:"order_id"   json:"order_id"`
	ActorID   int64     `db:"actor_id"   json:"actor_id"`
	ActorRole string    `db:"actor_role" json:"actor_role"`
	Action    string    `db:"action"     json:"action"`
	Details   *string   `db:"details"    json:"details,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// OTCOrderExportRow is a flat row used for CSV export (joins currencies + users).
type OTCOrderExportRow struct {
	UID              string           `db:"uid"`
	Status           string           `db:"status"`
	ClientEmail      string           `db:"client_email"`
	OperatorEmail    *string          `db:"operator_email"`
	FromCurrencyCode string           `db:"from_currency_code"`
	ToCurrencyCode   string           `db:"to_currency_code"`
	FromAmount       decimal.Decimal  `db:"from_amount"`
	ProposedRate     decimal.Decimal  `db:"proposed_rate"`
	AgreedRate       *decimal.Decimal `db:"agreed_rate"`
	AgreedFromAmount *decimal.Decimal `db:"agreed_from_amount"`
	ToAmount         *decimal.Decimal `db:"to_amount"`
	CancelReason     *string          `db:"cancel_reason"`
	CreatedAt        time.Time        `db:"created_at"`
	UpdatedAt        time.Time        `db:"updated_at"`
}

// OTCAnalyticsSummary is the high-level aggregate block in the analytics response.
type OTCAnalyticsSummary struct {
	TotalOrders    int64           `json:"total_orders"    db:"total_orders"`
	Completed      int64           `json:"completed"       db:"completed"`
	Cancelled      int64           `json:"cancelled"       db:"cancelled"`
	Expired        int64           `json:"expired"         db:"expired"`
	ConversionRate float64         `json:"conversion_rate"` // computed after DB scan, ratio not money
	TotalVolume    decimal.Decimal `json:"total_volume"    db:"total_volume"`
	AvgSpreadPct   decimal.Decimal `json:"avg_spread_pct"  db:"avg_spread_pct"`
}

// OTCAnalyticsPeriod is one bucket in the time-series breakdown.
type OTCAnalyticsPeriod struct {
	Period    string          `json:"period"    db:"period"`
	Total     int64           `json:"total"     db:"total"`
	Completed int64           `json:"completed" db:"completed"`
	Cancelled int64           `json:"cancelled" db:"cancelled"`
	Expired   int64           `json:"expired"   db:"expired"`
	Volume    decimal.Decimal `json:"volume"    db:"volume"`
}

// OTCAnalytics is the full analytics response payload.
type OTCAnalytics struct {
	Summary  OTCAnalyticsSummary  `json:"summary"`
	ByPeriod []OTCAnalyticsPeriod `json:"by_period"`
}
