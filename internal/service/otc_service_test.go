package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// ---------- mock implementations ----------

type mockOTCRepo struct {
	order    *domain.OTCOrderDetail
	configs  []domain.OTCConfig
	orderErr error
	message  *domain.OTCMessage
	msgErr   error

	createCalled              bool
	takeCalled                bool
	agreeCalled               bool
	cancelCalled              bool
	setPaymentRecvCalled      bool
	completeAtomicCalled      bool
	completeAtomicErr         error
	updateOfferStatCalled     bool
	updateOfferStatErr        error
	expireCalled              bool
	expireResult              bool
	markMessagesReadCalled    bool
}

func (m *mockOTCRepo) Create(_ context.Context, order *domain.OTCOrder) error {
	m.createCalled = true
	order.UID = "test-uid"
	return m.orderErr
}

func (m *mockOTCRepo) GetConfigs(ctx context.Context) ([]domain.OTCConfig, error) {
	return m.configs, m.msgErr
}

func (m *mockOTCRepo) GetByUID(_ context.Context, _ string) (*domain.OTCOrderDetail, error) {
	return m.order, m.orderErr
}
func (m *mockOTCRepo) ListByUser(_ context.Context, _ int64, _, _ int, _ string) ([]domain.OTCOrder, int64, error) {
	return nil, 0, nil
}
func (m *mockOTCRepo) ListAll(_ context.Context, _, _ int, _ domain.OTCListFilter) ([]domain.OTCOrder, int64, error) {
	return nil, 0, nil
}
func (m *mockOTCRepo) ListAllForExport(_ context.Context, _ domain.OTCListFilter) ([]domain.OTCOrderExportRow, error) {
	return nil, nil
}
func (m *mockOTCRepo) CreateAuditLog(_ context.Context, _ *domain.OTCAuditLog) error { return nil }
func (m *mockOTCRepo) GetAuditLogs(_ context.Context, _ int64) ([]domain.OTCAuditLog, error) {
	return nil, nil
}
func (m *mockOTCRepo) GetAnalytics(_ context.Context, _, _ time.Time, _ string) (*domain.OTCAnalytics, error) {
	return nil, nil
}
func (m *mockOTCRepo) Take(_ context.Context, _, _ int64) error {
	m.takeCalled = true
	return m.orderErr
}
func (m *mockOTCRepo) CreateMessage(_ context.Context, msg *domain.OTCMessage) error {
	msg.ID = 1
	return m.msgErr
}
func (m *mockOTCRepo) GetMessageByID(_ context.Context, _ int64) (*domain.OTCMessage, error) {
	return m.message, m.msgErr
}
func (m *mockOTCRepo) UpdateOfferStatus(_ context.Context, _ int64, _ domain.OTCOfferStatus) error {
	m.updateOfferStatCalled = true
	if m.updateOfferStatErr != nil {
		return m.updateOfferStatErr
	}
	return m.msgErr
}
func (m *mockOTCRepo) Agree(_ context.Context, _ int64, _, _, _ decimal.Decimal, _ time.Time) error {
	m.agreeCalled = true
	return m.orderErr
}
func (m *mockOTCRepo) Cancel(_ context.Context, _ int64, _, _ string) error {
	m.cancelCalled = true
	return m.orderErr
}
func (m *mockOTCRepo) SetPaymentReceived(_ context.Context, _ int64) error {
	m.setPaymentRecvCalled = true
	return m.orderErr
}
func (m *mockOTCRepo) CompleteOrderAtomic(_ context.Context, _, _, _, _ int64, _, _, _ decimal.Decimal, _ string) error {
	m.completeAtomicCalled = true
	if m.completeAtomicErr != nil {
		return m.completeAtomicErr
	}
	return m.orderErr
}
func (m *mockOTCRepo) MarkMessagesRead(_ context.Context, _, _ int64) error {
	m.markMessagesReadCalled = true
	return nil
}
func (m *mockOTCRepo) Expire(_ context.Context, _ int64) (bool, error) {
	m.expireCalled = true
	return m.expireResult, m.orderErr
}
func (m *mockOTCRepo) GetConfigByID(_ context.Context, _ int64) (*domain.OTCConfig, error) {
	return nil, nil
}
func (m *mockOTCRepo) GetConfigByPair(_ context.Context, _, _ int64) (*domain.OTCConfig, error) {
	// Return a permissive config by default so existing tests are unaffected
	return &domain.OTCConfig{IsActive: true, MinFromAmount: decimal.Zero}, nil
}
func (m *mockOTCRepo) GetActiveConfigsWithCurrencies(_ context.Context) ([]domain.OTCConfigWithCurrencies, error) {
	return nil, nil
}
func (m *mockOTCRepo) GetAllConfigsWithCurrencies(_ context.Context) ([]domain.OTCConfigWithCurrencies, error) {
	return nil, nil
}
func (m *mockOTCRepo) CreateConfig(_ context.Context, _ *domain.OTCConfig) error  { return nil }
func (m *mockOTCRepo) UpdateConfig(_ context.Context, _ *domain.OTCConfig) error  { return nil }
func (m *mockOTCRepo) DeleteConfig(_ context.Context, _ int64) error              { return nil }

type mockUserRepo struct {
	user    *domain.User
	userErr error
}

func (m *mockUserRepo) GetByID(_ context.Context, _ int64) (*domain.User, error) {
	return m.user, m.userErr
}

type mockWalletRepo struct {
	wallets       map[string]*domain.Wallet // key: "userID:currencyID"
	updateErr     map[int64]error           // key: walletID
	lockErr       map[int64]error
	unlockErr     map[int64]error
	finalizeErr   map[int64]error
	updateCalls   []updateCall
	lockCalls     []lockCall
	unlockCalls   []unlockCall
	finalizeCalls []finalizeCall
}

type updateCall struct {
	walletID int64
	balance  decimal.Decimal
	locked   decimal.Decimal
}

type lockCall struct {
	walletID int64
	amount   decimal.Decimal
}

type unlockCall struct {
	walletID int64
	amount   decimal.Decimal
}

type finalizeCall struct {
	walletID         int64
	fromAmount       decimal.Decimal
	agreedFromAmount decimal.Decimal
}

func newMockWalletRepo() *mockWalletRepo {
	return &mockWalletRepo{
		wallets:     make(map[string]*domain.Wallet),
		updateErr:   make(map[int64]error),
		lockErr:     make(map[int64]error),
		unlockErr:   make(map[int64]error),
		finalizeErr: make(map[int64]error),
	}
}

func (m *mockWalletRepo) setWallet(userID int64, currencyID int32, wallet *domain.Wallet) {
	m.wallets[fmt.Sprintf("%d:%d", userID, currencyID)] = wallet
}

func (m *mockWalletRepo) GetByUserAndCurrency(_ context.Context, userID int64, currencyID int32) (*domain.Wallet, error) {
	key := fmt.Sprintf("%d:%d", userID, currencyID)
	w, ok := m.wallets[key]
	if !ok {
		return nil, errors.New("wallet not found")
	}
	cp := *w
	return &cp, nil
}

func (m *mockWalletRepo) LockAmount(_ context.Context, walletID int64, amount decimal.Decimal) error {
	m.lockCalls = append(m.lockCalls, lockCall{walletID, amount})
	if err, ok := m.lockErr[walletID]; ok {
		return err
	}
	for _, w := range m.wallets {
		if w.ID == walletID {
			if w.Balance.LessThan(amount) {
				return errors.New("insufficient balance")
			}
			w.Balance = w.Balance.Sub(amount)
			w.Locked = w.Locked.Add(amount)
			return nil
		}
	}
	return errors.New("wallet not found")
}

func (m *mockWalletRepo) UnlockAmount(_ context.Context, walletID int64, amount decimal.Decimal) error {
	m.unlockCalls = append(m.unlockCalls, unlockCall{walletID, amount})
	if err, ok := m.unlockErr[walletID]; ok {
		return err
	}
	for _, w := range m.wallets {
		if w.ID == walletID {
			if !w.Locked.LessThan(amount) {
				w.Locked = w.Locked.Sub(amount)
				w.Balance = w.Balance.Add(amount)
			}
			return nil
		}
	}
	return nil // best-effort
}

func (m *mockWalletRepo) RefreshWalletCache(_ context.Context, _ int64) error { return nil }

// ---------- mock tx repo ----------

type mockTxRepo struct {
	createErr error
	created   []*domain.Transaction
}

func (m *mockTxRepo) Create(_ context.Context, tx *domain.Transaction) error {
	if m.createErr != nil {
		return m.createErr
	}
	tx.ID = int64(len(m.created) + 1)
	m.created = append(m.created, tx)
	return nil
}

// selectiveTxRepo succeeds for the first `failAfter` calls, then returns an error.
type selectiveTxRepo struct {
	failAfter int
	calls     int
	created   []*domain.Transaction
}

func (m *selectiveTxRepo) Create(_ context.Context, tx *domain.Transaction) error {
	m.calls++
	if m.calls > m.failAfter {
		return errors.New("db error on tx insert")
	}
	tx.ID = int64(m.calls)
	m.created = append(m.created, tx)
	return nil
}

// ---------- helpers ----------

func ptr[T any](v T) *T { return &v }

func dec(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

func makeOTCService(otcRepo *mockOTCRepo, userRepo *mockUserRepo, walletRepo *mockWalletRepo) *OTCService {
	return &OTCService{
		otcRepo:    otcRepo,
		userRepo:   userRepo,
		walletRepo: walletRepo,
		notif:      &NoOpNotificationService{},
	}
}

func paymentReceivedOrder(operatorID int64) *domain.OTCOrderDetail {
	agreedFrom := dec(1000.0)
	toAmt := dec(50.0)
	proposedRate := dec(0.05)
	return &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:               1,
			UID:              "abc",
			UserID:           10,
			OperatorID:       ptr(operatorID),
			FromCurrencyID:   1,
			ToCurrencyID:     2,
			FromAmount:       dec(1000.0),
			ProposedRate:     dec(0.05),
			AgreedRate:       &proposedRate,
			AgreedFromAmount: &agreedFrom,
			ToAmount:         &toAmt,
			Status:           domain.OTCStatusPaymentReceived,
		},
	}
}

// ---------- CompleteOrder tests ----------

func TestCompleteOrder_HappyPath(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)

	otcRepo := &mockOTCRepo{order: order}
	wallets := newMockWalletRepo()
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, UserID: order.UserID, CurrencyID: 1, Balance: dec(2000.0), Locked: dec(1000.0)})
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, UserID: order.UserID, CurrencyID: 2, Balance: dec(10.0)})

	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)
	if err := svc.CompleteOrder(context.Background(), "abc", operatorID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All DB writes happen inside CompleteOrderAtomic (single SQL transaction).
	if !otcRepo.completeAtomicCalled {
		t.Error("expected CompleteOrderAtomic to be called on repo")
	}
}

func TestCompleteOrder_AtomicFails(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)

	otcRepo := &mockOTCRepo{order: order, completeAtomicErr: errors.New("db error")}
	wallets := newMockWalletRepo()
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, Balance: dec(2000.0), Locked: dec(1000.0)})
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, Balance: dec(10.0)})

	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)
	if err := svc.CompleteOrder(context.Background(), "abc", operatorID); err == nil {
		t.Fatal("expected error when atomic commit fails")
	}
}

func TestCompleteOrder_WrongStatus(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)
	order.Status = domain.OTCStatusNegotiating

	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.CompleteOrder(context.Background(), "abc", operatorID)
	if err == nil || err.Error() != "order must be in payment_received status to complete" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteOrder_WrongOperator(t *testing.T) {
	order := paymentReceivedOrder(99)

	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.CompleteOrder(context.Background(), "abc", 77) // different operator
	if err == nil || err.Error() != "only the assigned operator can complete the order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteOrder_NilOperator(t *testing.T) {
	order := paymentReceivedOrder(99)
	order.OperatorID = nil

	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.CompleteOrder(context.Background(), "abc", 99)
	if err == nil {
		t.Error("expected error for nil operator")
	}
}

func TestCompleteOrder_NilAgreedAmounts(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)
	order.AgreedFromAmount = nil

	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.CompleteOrder(context.Background(), "abc", operatorID)
	if err == nil {
		t.Error("expected error for nil agreed amounts")
	}
}

func TestCompleteOrder_FromWalletNotFound(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)

	wallets := newMockWalletRepo()
	// from-wallet not registered — only to-wallet exists
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, Balance: dec(10.0)})

	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, wallets)
	err := svc.CompleteOrder(context.Background(), "abc", operatorID)
	if err == nil || err.Error() != "client from-wallet not found: wallet not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteOrder_ToWalletNotFound(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)

	wallets := newMockWalletRepo()
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, Balance: dec(2000.0), Locked: dec(1000.0)})
	// to-wallet not registered

	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, wallets)
	err := svc.CompleteOrder(context.Background(), "abc", operatorID)
	if err == nil || err.Error() != "client to-wallet not found: wallet not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteOrder_InsufficientBalance(t *testing.T) {
	operatorID := int64(99)
	order := paymentReceivedOrder(operatorID)

	// Simulate the atomic method failing due to insufficient locked balance.
	otcRepo := &mockOTCRepo{order: order, completeAtomicErr: errors.New("insufficient locked balance")}
	wallets := newMockWalletRepo()
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, Balance: dec(500.0), Locked: decimal.Zero})
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, Balance: dec(10.0)})

	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)
	err := svc.CompleteOrder(context.Background(), "abc", operatorID)
	if err == nil {
		t.Error("expected error when insufficient locked balance")
	}
	if otcRepo.completeAtomicCalled {
		// atomic was called but should have returned the error above
	}
}

func TestCompleteOrder_OrderNotFound(t *testing.T) {
	otcRepo := &mockOTCRepo{orderErr: errors.New("order not found")}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())
	err := svc.CompleteOrder(context.Background(), "missing", 99)
	if err == nil {
		t.Error("expected error for missing order")
	}
}

// ---------- CreateOrder tests ----------

func TestCreateOrder_KYCTooLow(t *testing.T) {
	svc := makeOTCService(
		&mockOTCRepo{},
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 1}},
		newMockWalletRepo(),
	)
	_, err := svc.CreateOrder(context.Background(), 1, 1, 2, dec(500), dec(0.05), nil)
	if err == nil || err.Error() != "KYC level 2 or higher required to create an OTC order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateOrder_KYCSufficient(t *testing.T) {
	otcRepo := &mockOTCRepo{}
	wallets := newMockWalletRepo()
	wallets.setWallet(1, 1, &domain.Wallet{ID: 1, Balance: dec(10000.0)})
	wallets.setWallet(1, 2, &domain.Wallet{ID: 2, Balance: decimal.Zero})
	svc := makeOTCService(
		otcRepo,
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 2, Email: "a@b.com"}},
		wallets,
	)
	order, err := svc.CreateOrder(context.Background(), 1, 1, 2, dec(500), dec(0.05), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order == nil {
		t.Error("expected order to be returned")
	}
	if !otcRepo.createCalled {
		t.Error("expected Create to be called")
	}
	if len(wallets.lockCalls) != 1 {
		t.Fatalf("expected 1 LockAmount call, got %d", len(wallets.lockCalls))
	}
}

func TestCreateOrder_UserNotFound(t *testing.T) {
	svc := makeOTCService(
		&mockOTCRepo{},
		&mockUserRepo{userErr: errors.New("user not found")},
		newMockWalletRepo(),
	)
	_, err := svc.CreateOrder(context.Background(), 99, 1, 2, dec(500), dec(0.05), nil)
	if err == nil {
		t.Error("expected error when user not found")
	}
}

func TestCreateOrder_InsufficientBalanceForLock(t *testing.T) {
	wallets := newMockWalletRepo()
	wallets.setWallet(1, 1, &domain.Wallet{ID: 1, Balance: dec(10.0)}) // not enough
	svc := makeOTCService(
		&mockOTCRepo{},
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 2}},
		wallets,
	)
	_, err := svc.CreateOrder(context.Background(), 1, 1, 2, dec(500), dec(0.05), nil)
	if err == nil || err.Error() != "insufficient balance to place OTC order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateOrder_LockFails(t *testing.T) {
	wallets := newMockWalletRepo()
	wallets.setWallet(1, 1, &domain.Wallet{ID: 1, Balance: dec(1000.0)})
	wallets.lockErr[1] = errors.New("db lock error")
	svc := makeOTCService(
		&mockOTCRepo{},
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 2}},
		wallets,
	)
	_, err := svc.CreateOrder(context.Background(), 1, 1, 2, dec(500), dec(0.05), nil)
	if err == nil {
		t.Error("expected error when lock fails")
	}
}

// ---------- TakeOrder tests ----------

func TestTakeOrder_WrongStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.TakeOrder(context.Background(), "abc", 5)
	if err == nil || err.Error() != "order must be in awaiting_review status to take" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTakeOrder_Success(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusAwaitingReview}}
	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())
	if err := svc.TakeOrder(context.Background(), "abc", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !otcRepo.takeCalled {
		t.Error("expected Take to be called")
	}
}

// ---------- AcceptOffer tests ----------

func TestAcceptOffer_CannotAcceptOwnOffer(t *testing.T) {
	offerStatus := domain.OTCOfferStatusPending
	rate := dec(0.05)
	to := dec(50.0)
	from := dec(1000.0)
	msg := &domain.OTCMessage{
		ID:              1,
		OrderID:         1,
		SenderID:        42,
		MessageType:     domain.OTCMessageTypeOffer,
		OfferRate:       &rate,
		OfferToAmount:   &to,
		OfferFromAmount: &from,
		OfferStatus:     &offerStatus,
	}
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	otcRepo := &mockOTCRepo{order: order, message: msg}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 42, "client") // same sender as acceptor
	if err == nil || err.Error() != "cannot accept your own offer" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcceptOffer_AlreadyNotPending(t *testing.T) {
	accepted := domain.OTCOfferStatusAccepted
	msg := &domain.OTCMessage{
		ID:          1,
		OrderID:     1,
		SenderID:    42,
		MessageType: domain.OTCMessageTypeOffer,
		OfferStatus: &accepted,
	}
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	otcRepo := &mockOTCRepo{order: order, message: msg}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 99, "client")
	if err == nil || err.Error() != "offer is no longer pending" {
		t.Errorf("unexpected error: %v", err)
	}
}

func makeOffer(orderID, senderID int64) *domain.OTCMessage {
	status := domain.OTCOfferStatusPending
	rate := dec(0.05)
	from := dec(1000.0)
	to := dec(50.0)
	return &domain.OTCMessage{
		ID:              1,
		OrderID:         orderID,
		SenderID:        senderID,
		SenderRole:      "operator",
		MessageType:     domain.OTCMessageTypeOffer,
		OfferRate:       &rate,
		OfferFromAmount: &from,
		OfferToAmount:   &to,
		OfferStatus:     &status,
	}
}

func TestAcceptOffer_HappyPath(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating}}
	msg := makeOffer(1, 5) // operator sent the offer
	otcRepo := &mockOTCRepo{order: order, message: msg}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 10, "client") // client accepts
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !otcRepo.updateOfferStatCalled {
		t.Error("expected UpdateOfferStatus to be called")
	}
	if !otcRepo.agreeCalled {
		t.Error("expected Agree to be called")
	}
}

func TestAcceptOffer_WrongStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusAwaitingPayment}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil || err.Error() != "order must be negotiating to accept an offer" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcceptOffer_MessageNotFound(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	otcRepo := &mockOTCRepo{order: order, msgErr: errors.New("not found")}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 99, 10, "client")
	if err == nil {
		t.Error("expected error when message not found")
	}
}

func TestAcceptOffer_MessageBelongsToDifferentOrder(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	msg := makeOffer(999, 5) // belongs to a different order
	svc := makeOTCService(&mockOTCRepo{order: order, message: msg}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil || err.Error() != "message does not belong to this order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcceptOffer_MessageNotAnOffer(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	msg := makeOffer(1, 5)
	msg.MessageType = domain.OTCMessageTypeText
	svc := makeOTCService(&mockOTCRepo{order: order, message: msg}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil || err.Error() != "message is not an offer" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcceptOffer_UpdateOfferStatusFails(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating}}
	msg := makeOffer(1, 5)
	otcRepo := &mockOTCRepo{order: order, message: msg, updateOfferStatErr: errors.New("db error")}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil {
		t.Error("expected error when UpdateOfferStatus fails")
	}
	if otcRepo.agreeCalled {
		t.Error("Agree must not be called when UpdateOfferStatus fails")
	}
}

// ---------- CancelOrder tests ----------

func TestCancelOrder_ClientCannotCancelOtherOrder(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.CancelOrder(context.Background(), "abc", 99, "client", "reason") // callerID=99, order.UserID=10
	if err == nil || err.Error() != "not authorized to cancel this order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCancelOrder_ClientCannotCancelFromTerminalStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusCompleted}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.CancelOrder(context.Background(), "abc", 10, "client", "reason")
	if err == nil {
		t.Error("expected error when cancelling completed order as client")
	}
}

func TestCancelOrder_OperatorCannotCancelTerminal(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusCompleted}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.CancelOrder(context.Background(), "abc", 99, "operator", "reason")
	if err == nil || err.Error() != "order is already in a terminal status" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCancelOrder_UnlocksWallet(t *testing.T) {
	order := &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:             1,
			UID:            "abc",
			UserID:         10,
			FromCurrencyID: 1,
			ToCurrencyID:   2,
			FromAmount:     dec(500.0),
			Status:         domain.OTCStatusNegotiating,
		},
	}
	wallets := newMockWalletRepo()
	wallets.setWallet(10, 1, &domain.Wallet{ID: 5, Locked: dec(500.0), Balance: decimal.Zero})

	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)

	if err := svc.CancelOrder(context.Background(), "abc", 10, "client", "changed mind"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wallets.unlockCalls) != 1 {
		t.Fatalf("expected 1 UnlockAmount call, got %d", len(wallets.unlockCalls))
	}
	if wallets.unlockCalls[0].walletID != 5 || !wallets.unlockCalls[0].amount.Equal(dec(500.0)) {
		t.Errorf("unexpected unlock call: %+v", wallets.unlockCalls[0])
	}
}

// ---------- ConfirmPaymentReceived tests ----------

func TestConfirmPaymentReceived_WrongOperator(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		OperatorID: ptr(int64(5)),
		Status:     domain.OTCStatusAwaitingPayment,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.ConfirmPaymentReceived(context.Background(), "abc", 99) // operatorID != 5
	if err == nil || err.Error() != "only the assigned operator can confirm payment" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfirmPaymentReceived_WrongStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		OperatorID: ptr(int64(5)),
		Status:     domain.OTCStatusNegotiating,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.ConfirmPaymentReceived(context.Background(), "abc", 5)
	if err == nil {
		t.Error("expected error for wrong status")
	}
}

// ---------- AcceptAsProposed tests ----------

func TestAcceptAsProposed_Success(t *testing.T) {
	operatorID := int64(7)
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:           1,
		OperatorID:   ptr(operatorID),
		Status:       domain.OTCStatusNegotiating,
		FromAmount:   dec(1000.0),
		ProposedRate: dec(0.05),
	}}
	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	if err := svc.AcceptAsProposed(context.Background(), "abc", operatorID, "operator"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !otcRepo.agreeCalled {
		t.Error("expected Agree to be called")
	}
}

func TestAcceptAsProposed_WrongStatus(t *testing.T) {
	operatorID := int64(7)
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		OperatorID: ptr(operatorID),
		Status:     domain.OTCStatusAwaitingReview,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.AcceptAsProposed(context.Background(), "abc", operatorID, "operator")
	if err == nil || err.Error() != "order must be negotiating to accept the proposed rate" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcceptAsProposed_WrongOperator(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		OperatorID: ptr(int64(7)),
		Status:     domain.OTCStatusNegotiating,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.AcceptAsProposed(context.Background(), "abc", 99, "operator")
	if err == nil || err.Error() != "only the assigned operator can accept the proposed rate" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAcceptAsProposed_NilOperator(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		OperatorID: nil,
		Status:     domain.OTCStatusNegotiating,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.AcceptAsProposed(context.Background(), "abc", 7, "operator")
	if err == nil {
		t.Error("expected error when order has no assigned operator")
	}
}

// ---------- SendMessage tests ----------

func TestSendMessage_Success_Negotiating(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:     1,
		UserID: 10,
		Status: domain.OTCStatusNegotiating,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	msg, err := svc.SendMessage(context.Background(), "abc", 10, "client", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil || msg.Content == nil || *msg.Content != "hello" {
		t.Error("expected message with correct content")
	}
}

func TestSendMessage_Success_AwaitingPayment(t *testing.T) {
	operatorID := int64(5)
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		UserID:     10,
		OperatorID: ptr(operatorID),
		Status:     domain.OTCStatusAwaitingPayment,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	msg, err := svc.SendMessage(context.Background(), "abc", operatorID, "operator", "payment details")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Error("expected message to be returned")
	}
}

func TestSendMessage_WrongStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:     1,
		UserID: 10,
		Status: domain.OTCStatusAwaitingReview,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.SendMessage(context.Background(), "abc", 10, "client", "hi")
	if err == nil || err.Error() != "messages can only be sent while order is negotiating or awaiting payment" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendMessage_NotParticipant(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:     1,
		UserID: 10,
		Status: domain.OTCStatusNegotiating,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.SendMessage(context.Background(), "abc", 99, "client", "hi")
	if err == nil || err.Error() != "not authorized to message on this order" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------- SendOffer tests ----------

func TestSendOffer_Success(t *testing.T) {
	operatorID := int64(5)
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:         1,
		UserID:     10,
		OperatorID: ptr(operatorID),
		Status:     domain.OTCStatusNegotiating,
	}}
	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())
	msg, err := svc.SendOffer(context.Background(), "abc", operatorID, "operator", dec(0.05), dec(1000.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil || msg.OfferRate == nil || !msg.OfferRate.Equal(dec(0.05)) {
		t.Error("expected offer message with correct rate")
	}
	if msg.OfferToAmount == nil || !msg.OfferToAmount.Equal(dec(50.0)) {
		t.Errorf("expected to_amount=50, got %v", msg.OfferToAmount)
	}
}

func TestSendOffer_WrongStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:     1,
		UserID: 10,
		Status: domain.OTCStatusAwaitingPayment,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.SendOffer(context.Background(), "abc", 10, "client", dec(0.05), dec(1000.0))
	if err == nil || err.Error() != "offers can only be sent while order is negotiating" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendOffer_NotParticipant(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:     1,
		UserID: 10,
		Status: domain.OTCStatusNegotiating,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.SendOffer(context.Background(), "abc", 99, "client", dec(0.05), dec(1000.0))
	if err == nil || err.Error() != "not authorized to send offers on this order" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------- RejectOffer tests ----------

func TestRejectOffer_Success(t *testing.T) {
	offerStatus := domain.OTCOfferStatusPending
	msg := &domain.OTCMessage{
		ID:          1,
		OrderID:     1,
		SenderID:    5, // operator sent the offer
		SenderRole:  "operator",
		MessageType: domain.OTCMessageTypeOffer,
		OfferStatus: &offerStatus,
	}
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating}}
	otcRepo := &mockOTCRepo{order: order, message: msg}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.RejectOffer(context.Background(), "abc", 1, 10, "client") // client rejects operator offer
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !otcRepo.updateOfferStatCalled {
		t.Error("expected UpdateOfferStatus to be called")
	}
}

func TestRejectOffer_CannotRejectOwnOffer(t *testing.T) {
	offerStatus := domain.OTCOfferStatusPending
	msg := &domain.OTCMessage{
		ID:          1,
		OrderID:     1,
		SenderID:    10,
		MessageType: domain.OTCMessageTypeOffer,
		OfferStatus: &offerStatus,
	}
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating}}
	svc := makeOTCService(&mockOTCRepo{order: order, message: msg}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.RejectOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil || err.Error() != "cannot reject your own offer" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRejectOffer_NotPending(t *testing.T) {
	accepted := domain.OTCOfferStatusAccepted
	msg := &domain.OTCMessage{
		ID:          1,
		OrderID:     1,
		SenderID:    5,
		MessageType: domain.OTCMessageTypeOffer,
		OfferStatus: &accepted,
	}
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating}}
	svc := makeOTCService(&mockOTCRepo{order: order, message: msg}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.RejectOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil || err.Error() != "offer is no longer pending" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRejectOffer_WrongStatus(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusAwaitingPayment}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())

	err := svc.RejectOffer(context.Background(), "abc", 1, 10, "client")
	if err == nil || err.Error() != "order must be negotiating to reject an offer" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------- GetOrder (lazy expiry) tests ----------

func TestGetOrder_LazyExpiry_UnlocksWallet(t *testing.T) {
	deadline := time.Now().Add(-1 * time.Minute) // already past
	order := &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:             1,
			UserID:         10,
			Status:         domain.OTCStatusAwaitingPayment,
			FromCurrencyID: 1,
			FromAmount:     dec(500.0),
			PaymentDeadline: &deadline,
		},
	}
	wallets := newMockWalletRepo()
	wallets.setWallet(10, 1, &domain.Wallet{ID: 5, Balance: decimal.Zero, Locked: dec(500.0)})

	otcRepo := &mockOTCRepo{order: order, expireResult: true}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)

	result, err := svc.GetOrder(context.Background(), "abc", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.OTCStatusExpired {
		t.Errorf("expected expired status, got %v", result.Status)
	}
	if !otcRepo.expireCalled {
		t.Error("expected Expire to be called")
	}
	if len(wallets.unlockCalls) != 1 || !wallets.unlockCalls[0].amount.Equal(dec(500.0)) {
		t.Errorf("expected wallet unlock of 500, got %+v", wallets.unlockCalls)
	}
}

func TestGetOrder_LazyExpiry_AlreadyExpiredByOther(t *testing.T) {
	// Expire returns false — another caller already expired it
	deadline := time.Now().Add(-1 * time.Minute)
	order := &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:              1,
			UserID:          10,
			Status:          domain.OTCStatusAwaitingPayment,
			FromCurrencyID:  1,
			FromAmount:      dec(500.0),
			PaymentDeadline: &deadline,
		},
	}
	wallets := newMockWalletRepo()
	wallets.setWallet(10, 1, &domain.Wallet{ID: 5, Balance: decimal.Zero, Locked: decimal.Zero})

	otcRepo := &mockOTCRepo{order: order, expireResult: false}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)

	result, err := svc.GetOrder(context.Background(), "abc", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.OTCStatusExpired {
		t.Errorf("expected expired status, got %v", result.Status)
	}
	if len(wallets.unlockCalls) != 0 {
		t.Error("expected no unlock when another caller already expired")
	}
}

func TestGetOrder_NoExpiry_FutureDeadline(t *testing.T) {
	deadline := time.Now().Add(10 * time.Minute) // still in the future
	order := &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:              1,
			UserID:          10,
			Status:          domain.OTCStatusAwaitingPayment,
			FromCurrencyID:  1,
			PaymentDeadline: &deadline,
		},
	}
	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	result, err := svc.GetOrder(context.Background(), "abc", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.OTCStatusAwaitingPayment {
		t.Errorf("expected awaiting_payment, got %v", result.Status)
	}
	if otcRepo.expireCalled {
		t.Error("expected Expire NOT to be called for future deadline")
	}
}

func TestGetOrder_CountsUnreadMessages(t *testing.T) {
	order := &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{ID: 1, UserID: 10, Status: domain.OTCStatusNegotiating},
		Messages: []domain.OTCMessage{
			{SenderID: 5, IsRead: false},  // unread from other party
			{SenderID: 5, IsRead: true},   // already read from other party
			{SenderID: 10, IsRead: false}, // from caller — don't count
		},
	}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())

	result, err := svc.GetOrder(context.Background(), "abc", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UnreadCount != 1 {
		t.Errorf("expected unread count 1, got %d", result.UnreadCount)
	}
}

// ---------- CancelOrder awaiting_payment tests ----------

func TestCancelOrder_ClientCannotCancelAwaitingPayment(t *testing.T) {
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{
		ID:     1,
		UserID: 10,
		Status: domain.OTCStatusAwaitingPayment,
	}}
	svc := makeOTCService(&mockOTCRepo{order: order}, &mockUserRepo{}, newMockWalletRepo())
	err := svc.CancelOrder(context.Background(), "abc", 10, "client", "can't pay")
	if err == nil {
		t.Error("expected error — client cannot cancel from awaiting_payment")
	}
}

func TestCancelOrder_OperatorCanCancelAwaitingPayment(t *testing.T) {
	order := &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:             1,
			UserID:         10,
			FromCurrencyID: 1,
			FromAmount:     dec(1000.0),
			Status:         domain.OTCStatusAwaitingPayment,
		},
	}
	wallets := newMockWalletRepo()
	wallets.setWallet(10, 1, &domain.Wallet{ID: 5, Locked: dec(1000.0)})

	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)
	err := svc.CancelOrder(context.Background(), "abc", 99, "operator", "client no-show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !otcRepo.cancelCalled {
		t.Error("expected Cancel to be called")
	}
}

// ---------- CreateConfig tests ----------

func TestCreateConfig_SameCurrencyError(t *testing.T) {
	svc := makeOTCService(&mockOTCRepo{}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.CreateConfig(context.Background(), 1, 1, dec(100), 30, true)
	if err == nil || err.Error() != "from_currency_id and to_currency_id must differ" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateConfig_NegativeMinAmount(t *testing.T) {
	svc := makeOTCService(&mockOTCRepo{}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.CreateConfig(context.Background(), 1, 2, dec(-10), 30, true)
	if err == nil || err.Error() != "min_from_amount must be non-negative" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateConfig_ZeroTimeout(t *testing.T) {
	svc := makeOTCService(&mockOTCRepo{}, &mockUserRepo{}, newMockWalletRepo())
	_, err := svc.CreateConfig(context.Background(), 1, 2, dec(100), 0, true)
	if err == nil || err.Error() != "payment_timeout_min must be positive" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateConfig_Success(t *testing.T) {
	svc := makeOTCService(&mockOTCRepo{}, &mockUserRepo{}, newMockWalletRepo())
	cfg, err := svc.CreateConfig(context.Background(), 1, 2, dec(500.0), 30, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || !cfg.MinFromAmount.Equal(dec(500.0)) {
		t.Error("expected config with min_from_amount=500")
	}
}
