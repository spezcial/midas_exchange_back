package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type OTCRepository struct {
	db *database.Postgres
}

func NewOTCRepository(db *database.Postgres) *OTCRepository {
	return &OTCRepository{db: db}
}

// --- Orders ---

func (r *OTCRepository) Create(ctx context.Context, order *domain.OTCOrder) error {
	return r.db.QueryRowContext(
		ctx, queries.OTCOrderCreateQuery,
		order.UserID, order.FromCurrencyID, order.ToCurrencyID,
		order.FromAmount, order.ProposedRate, order.Comment,
	).Scan(&order.ID, &order.UID, &order.CreatedAt, &order.UpdatedAt)
}

func (r *OTCRepository) GetByUID(ctx context.Context, uid string) (*domain.OTCOrderDetail, error) {
	var order domain.OTCOrderDetail
	err := r.db.GetContext(ctx, &order, queries.OTCOrderGetByUIDQuery, uid)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order not found")
	}
	if err != nil {
		return nil, err
	}

	// Lazy expiry
	if order.Status == domain.OTCStatusAwaitingPayment &&
		order.PaymentDeadline != nil &&
		order.PaymentDeadline.Before(time.Now()) {
		_ = r.Expire(ctx, order.ID)
		order.Status = domain.OTCStatusExpired
	}

	msgs, err := r.ListMessages(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	order.Messages = msgs

	return &order, nil
}

func (r *OTCRepository) ListByUser(ctx context.Context, userID int64, limit, offset int, status string) ([]domain.OTCOrder, int64, error) {
	// Count
	countQB := newQueryBuilder(queries.OTCOrderCountByUserBaseQuery)
	if status != "" {
		countQB.AddWhere(fmt.Sprintf("status = $%d", countQB.paramCounter), status)
	}
	countQuery, countArgs := countQB.Build("", "")
	countArgs = append([]interface{}{userID}, countArgs...)

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List
	listQB := newQueryBuilder(queries.OTCOrderListByUserBaseQuery)
	if status != "" {
		listQB.AddWhere(fmt.Sprintf("status = $%d", listQB.paramCounter), status)
	}
	listQuery, listArgs := listQB.Build(
		"ORDER BY created_at DESC",
		fmt.Sprintf("LIMIT $%d OFFSET $%d", listQB.paramCounter, listQB.paramCounter+1),
	)
	listArgs = append(listArgs, limit, offset)
	listArgs = append([]interface{}{userID}, listArgs...)

	var orders []domain.OTCOrder
	if err := r.db.SelectContext(ctx, &orders, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *OTCRepository) ListAll(ctx context.Context, limit, offset int, status, email string) ([]domain.OTCOrder, int64, error) {
	// For admin listing; email filter joins users table
	var (
		countQuery string
		listQuery  string
		countArgs  []interface{}
		listArgs   []interface{}
	)

	if email != "" {
		emailPattern := "%" + email + "%"
		countQuery = `SELECT COUNT(*) FROM otc_orders o JOIN users u ON o.user_id = u.id WHERE u.email ILIKE $1`
		listQuery = `SELECT o.id, o.uid, o.user_id, o.operator_id, o.from_currency_id, o.to_currency_id,
			o.from_amount, o.proposed_rate, o.agreed_rate, o.agreed_from_amount, o.to_amount,
			o.status, o.comment, o.cancel_reason, o.cancelled_by, o.payment_deadline, o.created_at, o.updated_at
			FROM otc_orders o JOIN users u ON o.user_id = u.id WHERE u.email ILIKE $1`
		countArgs = []interface{}{emailPattern}
		listArgs = []interface{}{emailPattern}

		if status != "" {
			countQuery += " AND o.status = $2"
			listQuery += " AND o.status = $2"
			countArgs = append(countArgs, status)
			listArgs = append(listArgs, status)
			listQuery += fmt.Sprintf(" ORDER BY o.created_at DESC LIMIT $3 OFFSET $4")
			listArgs = append(listArgs, limit, offset)
		} else {
			listQuery += fmt.Sprintf(" ORDER BY o.created_at DESC LIMIT $2 OFFSET $3")
			listArgs = append(listArgs, limit, offset)
		}
	} else {
		countQB := newQueryBuilder(queries.OTCOrderCountAllBaseQuery)
		if status != "" {
			countQB.AddWhere(fmt.Sprintf("status = $%d", countQB.paramCounter), status)
		}
		countQuery, countArgs = countQB.Build("", "")

		listQB := newQueryBuilder(queries.OTCOrderListAllBaseQuery)
		if status != "" {
			listQB.AddWhere(fmt.Sprintf("status = $%d", listQB.paramCounter), status)
		}
		listQuery, listArgs = listQB.Build(
			"ORDER BY created_at DESC",
			fmt.Sprintf("LIMIT $%d OFFSET $%d", listQB.paramCounter, listQB.paramCounter+1),
		)
		listArgs = append(listArgs, limit, offset)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	var orders []domain.OTCOrder
	if err := r.db.SelectContext(ctx, &orders, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *OTCRepository) Take(ctx context.Context, orderID, operatorID int64) error {
	var updatedAt time.Time
	return r.db.QueryRowContext(ctx, queries.OTCOrderTakeQuery, operatorID, orderID).Scan(&updatedAt)
}

func (r *OTCRepository) Agree(ctx context.Context, orderID int64, rate, fromAmt, toAmt float64, deadline time.Time) error {
	var updatedAt time.Time
	return r.db.QueryRowContext(ctx, queries.OTCOrderAgreeQuery, rate, fromAmt, toAmt, deadline, orderID).Scan(&updatedAt)
}

func (r *OTCRepository) Cancel(ctx context.Context, orderID int64, reason, cancelledBy string) error {
	var updatedAt time.Time
	return r.db.QueryRowContext(ctx, queries.OTCOrderCancelQuery, reason, cancelledBy, orderID).Scan(&updatedAt)
}

func (r *OTCRepository) SetPaymentReceived(ctx context.Context, orderID int64) error {
	var updatedAt time.Time
	return r.db.QueryRowContext(ctx, queries.OTCOrderSetPaymentReceivedQuery, orderID).Scan(&updatedAt)
}

func (r *OTCRepository) Complete(ctx context.Context, orderID int64) error {
	var updatedAt time.Time
	return r.db.QueryRowContext(ctx, queries.OTCOrderCompleteQuery, orderID).Scan(&updatedAt)
}

func (r *OTCRepository) Expire(ctx context.Context, orderID int64) error {
	var updatedAt time.Time
	return r.db.QueryRowContext(ctx, queries.OTCOrderExpireQuery, orderID).Scan(&updatedAt)
}

// --- Messages ---

func (r *OTCRepository) CreateMessage(ctx context.Context, msg *domain.OTCMessage) error {
	return r.db.QueryRowContext(
		ctx, queries.OTCMessageCreateQuery,
		msg.OrderID, msg.SenderID, msg.SenderRole, msg.MessageType,
		msg.Content, msg.OfferRate, msg.OfferFromAmount, msg.OfferToAmount, msg.OfferStatus,
	).Scan(&msg.ID, &msg.CreatedAt)
}

func (r *OTCRepository) ListMessages(ctx context.Context, orderID int64) ([]domain.OTCMessage, error) {
	var msgs []domain.OTCMessage
	err := r.db.SelectContext(ctx, &msgs, queries.OTCMessageListByOrderQuery, orderID)
	return msgs, err
}

func (r *OTCRepository) GetMessageByID(ctx context.Context, msgID int64) (*domain.OTCMessage, error) {
	var msg domain.OTCMessage
	err := r.db.GetContext(ctx, &msg, queries.OTCMessageGetByIDQuery, msgID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	return &msg, err
}

func (r *OTCRepository) UpdateOfferStatus(ctx context.Context, msgID int64, status domain.OTCOfferStatus) error {
	_, err := r.db.ExecContext(ctx, queries.OTCMessageUpdateOfferStatusQuery, string(status), msgID)
	return err
}
