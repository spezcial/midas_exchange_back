package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type OTCRepository struct {
	db    *database.Postgres
	cache *cache.CacheService
}

func NewOTCRepository(db *database.Postgres, cache *cache.CacheService) *OTCRepository {
	return &OTCRepository{db: db, cache: cache}
}

// --- Orders ---

func (r *OTCRepository) Create(ctx context.Context, order *domain.OTCOrder) error {
	return r.db.QueryRowContext(
		ctx, queries.OTCOrderCreateQuery,
		order.UserID, order.FromCurrencyID, order.ToCurrencyID,
		order.FromAmount, order.ProposedRate, order.Comment,
	).Scan(&order.ID, &order.UID, &order.CreatedAt, &order.UpdatedAt)
}

// --- Config ---

func (r *OTCRepository) GetConfigs(ctx context.Context) ([]domain.OTCConfig, error) {
	if configs, ok := r.cache.GetOTCConfigs(); ok {
		return configs, nil
	}
	var configs []domain.OTCConfig
	if err := r.db.SelectContext(ctx, &configs, queries.OTCConfigListQuery); err != nil {
		return nil, err
	}
	r.cache.SetOTCConfigs(configs)
	return configs, nil
}

func (r *OTCRepository) GetConfigByID(ctx context.Context, id int64) (*domain.OTCConfig, error) {
	if config, ok := r.cache.GetOTCConfigByID(id); ok {
		return config, nil
	}
	var config domain.OTCConfig
	err := r.db.GetContext(ctx, &config, queries.OTCConfigGetByIDQuery, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("config not found")
	}
	if err != nil {
		return nil, err
	}
	r.cache.SetOTCConfig(&config)
	return &config, nil
}

func (r *OTCRepository) GetConfigByPair(ctx context.Context, fromCurrencyID, toCurrencyID int64) (*domain.OTCConfig, error) {
	if config, ok := r.cache.GetOTCConfigByPair(fromCurrencyID, toCurrencyID); ok {
		return config, nil
	}
	var config domain.OTCConfig
	err := r.db.GetContext(ctx, &config, queries.OTCConfigGetByPairQuery, fromCurrencyID, toCurrencyID)
	if err == sql.ErrNoRows {
		return nil, nil // no config for this pair is valid — don't cache nil
	}
	if err != nil {
		return nil, err
	}
	r.cache.SetOTCConfig(&config)
	return &config, nil
}

func (r *OTCRepository) CreateConfig(ctx context.Context, config *domain.OTCConfig) error {
	if err := r.db.QueryRowContext(
		ctx, queries.OTCConfigCreateQuery,
		config.FromCurrencyID, config.ToCurrencyID,
		config.MinFromAmount, config.PaymentTimeoutMin, config.IsActive,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt); err != nil {
		return err
	}
	r.cache.SetOTCConfig(config) // also invalidates the list
	return nil
}

func (r *OTCRepository) UpdateConfig(ctx context.Context, config *domain.OTCConfig) error {
	if err := r.db.QueryRowContext(
		ctx, queries.OTCConfigUpdateQuery,
		config.MinFromAmount, config.PaymentTimeoutMin, config.IsActive, config.ID,
	).Scan(&config.UpdatedAt); err != nil {
		return err
	}
	r.cache.SetOTCConfig(config) // also invalidates the list
	return nil
}

func (r *OTCRepository) DeleteConfig(ctx context.Context, id int64) error {
	// Fetch first so we have the pair IDs needed to evict all keys
	config, err := r.GetConfigByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, queries.OTCConfigDeleteQuery, id); err != nil {
		return err
	}
	r.cache.InvalidateOTCConfig(id, config.FromCurrencyID, config.ToCurrencyID)
	return nil
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

	msgs, err := r.ListMessages(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	order.Messages = msgs

	return &order, nil
}

func (r *OTCRepository) ListByUser(ctx context.Context, userID int64, limit, offset int, status string) ([]domain.OTCOrder, int64, error) {
	var (
		total      int64
		countQuery string
		countArgs  []interface{}
		listQuery  string
		listArgs   []interface{}
	)

	// Base queries already bind user_id as $1; additional params start at $2.
	if status != "" {
		countQuery = queries.OTCOrderCountByUserBaseQuery + ` AND status = $2`
		countArgs = []interface{}{userID, status}
		listQuery = queries.OTCOrderListByUserBaseQuery + ` AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		listArgs = []interface{}{userID, status, limit, offset}
	} else {
		countQuery = queries.OTCOrderCountByUserBaseQuery
		countArgs = []interface{}{userID}
		listQuery = queries.OTCOrderListByUserBaseQuery + ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		listArgs = []interface{}{userID, limit, offset}
	}

	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

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
			o.status, o.comment, o.cancel_reason, o.cancelled_by, o.payment_deadline, o.created_at, o.updated_at,
			(SELECT COUNT(*) FROM otc_messages m WHERE m.order_id = o.id AND m.sender_role = 'client' AND m.is_read = false) AS unread_count
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
	err := r.db.QueryRowContext(ctx, queries.OTCOrderTakeQuery, operatorID, orderID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("order is no longer available to take")
	}
	return err
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

func (r *OTCRepository) Expire(ctx context.Context, orderID int64) (bool, error) {
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, queries.OTCOrderExpireQuery, orderID).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return false, nil // already expired or status was not awaiting_payment
	}
	if err != nil {
		return false, err
	}
	return true, nil
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

func (r *OTCRepository) MarkMessagesRead(ctx context.Context, orderID, readerID int64) error {
	_, err := r.db.ExecContext(ctx, queries.OTCMessageMarkReadQuery, orderID, readerID)
	return err
}

// CompleteOrderAtomic wraps all five writes that finalise an OTC order into a
// single SQL transaction: finalize from-wallet, credit to-wallet, insert debit
// tx record, insert credit tx record, mark order completed.
// Either all writes commit or none do.
func (r *OTCRepository) CompleteOrderAtomic(
	ctx context.Context,
	orderID, fromWalletID, toWalletID, userID int64,
	fromAmount, agreedFromAmount, toAmount float64,
	orderUID string,
) error {
	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var ts time.Time

	// 1. Release from-wallet lock: locked -= fromAmount, balance += excess
	err = tx.QueryRowContext(ctx, queries.WalletFinalizeLockedQuery,
		fromAmount, agreedFromAmount, fromWalletID,
	).Scan(&ts)
	if err == sql.ErrNoRows {
		return fmt.Errorf("insufficient locked balance")
	}
	if err != nil {
		return fmt.Errorf("finalize from-wallet: %w", err)
	}

	// 2. Credit to-wallet
	err = tx.QueryRowContext(ctx, queries.WalletAddBalanceQuery, toAmount, toWalletID).Scan(&ts)
	if err != nil {
		return fmt.Errorf("credit to-wallet: %w", err)
	}

	// 3 & 4. Transaction records (debit + credit)
	var recID int64
	var createdAt, updatedAt time.Time
	for _, row := range []struct {
		walletID int64
		txType   string
		amount   float64
	}{
		{fromWalletID, string(domain.TransactionTypeOTCDebit), agreedFromAmount},
		{toWalletID, string(domain.TransactionTypeOTCCredit), toAmount},
	} {
		err = tx.QueryRowContext(ctx, queries.TransactionCreateQuery,
			userID, row.walletID, row.txType, row.amount, 0.0,
			string(domain.TransactionStatusCompleted), orderUID,
		).Scan(&recID, &createdAt, &updatedAt)
		if err != nil {
			return fmt.Errorf("record %s tx: %w", row.txType, err)
		}
	}

	// 5. Mark order completed
	if err = tx.QueryRowContext(ctx, queries.OTCOrderCompleteQuery, orderID).Scan(&ts); err != nil {
		return fmt.Errorf("complete order: %w", err)
	}

	return tx.Commit()
}

func (r *OTCRepository) UpdateOfferStatus(ctx context.Context, msgID int64, status domain.OTCOfferStatus) error {
	result, err := r.db.ExecContext(ctx, queries.OTCMessageUpdateOfferStatusQuery, string(status), msgID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("offer is no longer pending")
	}
	return nil
}
