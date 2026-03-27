package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
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
func (m *mockOTCRepo) ListAll(_ context.Context, _, _ int, _, _ string) ([]domain.OTCOrder, int64, error) {
	return nil, 0, nil
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
	return m.msgErr
}
func (m *mockOTCRepo) Agree(_ context.Context, _ int64, _, _, _ float64, _ time.Time) error {
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
func (m *mockOTCRepo) CompleteOrderAtomic(_ context.Context, _, _, _, _ int64, _, _, _ float64, _ string) error {
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
	balance  float64
	locked   float64
}

type lockCall struct {
	walletID int64
	amount   float64
}

type unlockCall struct {
	walletID int64
	amount   float64
}

type finalizeCall struct {
	walletID         int64
	fromAmount       float64
	agreedFromAmount float64
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

func (m *mockWalletRepo) UpdateBalance(_ context.Context, walletID int64, balance, locked float64) error {
	m.updateCalls = append(m.updateCalls, updateCall{walletID, balance, locked})
	if err, ok := m.updateErr[walletID]; ok {
		return err
	}
	for _, w := range m.wallets {
		if w.ID == walletID {
			w.Balance = balance
			w.Locked = locked
		}
	}
	return nil
}

func (m *mockWalletRepo) LockAmount(_ context.Context, walletID int64, amount float64) error {
	m.lockCalls = append(m.lockCalls, lockCall{walletID, amount})
	if err, ok := m.lockErr[walletID]; ok {
		return err
	}
	for _, w := range m.wallets {
		if w.ID == walletID {
			if w.Balance < amount {
				return errors.New("insufficient balance")
			}
			w.Balance -= amount
			w.Locked += amount
			return nil
		}
	}
	return errors.New("wallet not found")
}

func (m *mockWalletRepo) UnlockAmount(_ context.Context, walletID int64, amount float64) error {
	m.unlockCalls = append(m.unlockCalls, unlockCall{walletID, amount})
	if err, ok := m.unlockErr[walletID]; ok {
		return err
	}
	for _, w := range m.wallets {
		if w.ID == walletID {
			if w.Locked >= amount {
				w.Locked -= amount
				w.Balance += amount
			}
			return nil
		}
	}
	return nil // best-effort
}

func (m *mockWalletRepo) FinalizeFromLocked(_ context.Context, walletID int64, fromAmount, agreedFromAmount float64) error {
	m.finalizeCalls = append(m.finalizeCalls, finalizeCall{walletID, fromAmount, agreedFromAmount})
	if err, ok := m.finalizeErr[walletID]; ok {
		return err
	}
	for _, w := range m.wallets {
		if w.ID == walletID {
			if w.Locked < fromAmount {
				return errors.New("insufficient locked balance or funds cannot cover agreed amount")
			}
			w.Locked -= fromAmount
			w.Balance += (fromAmount - agreedFromAmount)
			return nil
		}
	}
	return errors.New("wallet not found")
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

func makeOTCService(otcRepo *mockOTCRepo, userRepo *mockUserRepo, walletRepo *mockWalletRepo) *OTCService {
	return &OTCService{
		otcRepo:    otcRepo,
		userRepo:   userRepo,
		walletRepo: walletRepo,
		notif:      &NoOpNotificationService{},
	}
}

func paymentReceivedOrder(operatorID int64) *domain.OTCOrderDetail {
	agreedFrom := 1000.0
	toAmt := 50.0
	return &domain.OTCOrderDetail{
		OTCOrder: domain.OTCOrder{
			ID:               1,
			UID:              "abc",
			UserID:           10,
			OperatorID:       ptr(operatorID),
			FromCurrencyID:   1,
			ToCurrencyID:     2,
			FromAmount:       1000.0,
			ProposedRate:     0.05,
			AgreedRate:       ptr(0.05),
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
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, UserID: order.UserID, CurrencyID: 1, Balance: 2000.0, Locked: 1000.0})
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, UserID: order.UserID, CurrencyID: 2, Balance: 10.0})

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
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, Balance: 2000.0, Locked: 1000.0})
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, Balance: 10.0})

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
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, Balance: 10.0})

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
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, Balance: 2000.0, Locked: 1000.0})
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
	wallets.setWallet(order.UserID, int32(order.FromCurrencyID), &domain.Wallet{ID: 1, Balance: 500.0, Locked: 0})
	wallets.setWallet(order.UserID, int32(order.ToCurrencyID), &domain.Wallet{ID: 2, Balance: 10.0})

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
	_, err := svc.CreateOrder(context.Background(), 1, 1, 2, 500, 0.05, nil)
	if err == nil || err.Error() != "KYC level 2 or higher required to create an OTC order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateOrder_KYCSufficient(t *testing.T) {
	otcRepo := &mockOTCRepo{}
	wallets := newMockWalletRepo()
	wallets.setWallet(1, 1, &domain.Wallet{ID: 1, Balance: 10000.0})
	wallets.setWallet(1, 2, &domain.Wallet{ID: 2, Balance: 0})
	svc := makeOTCService(
		otcRepo,
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 2, Email: "a@b.com"}},
		wallets,
	)
	order, err := svc.CreateOrder(context.Background(), 1, 1, 2, 500, 0.05, nil)
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
	_, err := svc.CreateOrder(context.Background(), 99, 1, 2, 500, 0.05, nil)
	if err == nil {
		t.Error("expected error when user not found")
	}
}

func TestCreateOrder_InsufficientBalanceForLock(t *testing.T) {
	wallets := newMockWalletRepo()
	wallets.setWallet(1, 1, &domain.Wallet{ID: 1, Balance: 10.0}) // not enough
	svc := makeOTCService(
		&mockOTCRepo{},
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 2}},
		wallets,
	)
	_, err := svc.CreateOrder(context.Background(), 1, 1, 2, 500, 0.05, nil)
	if err == nil || err.Error() != "insufficient balance to place OTC order" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateOrder_LockFails(t *testing.T) {
	wallets := newMockWalletRepo()
	wallets.setWallet(1, 1, &domain.Wallet{ID: 1, Balance: 1000.0})
	wallets.lockErr[1] = errors.New("db lock error")
	svc := makeOTCService(
		&mockOTCRepo{},
		&mockUserRepo{user: &domain.User{ID: 1, KycLevel: 2}},
		wallets,
	)
	_, err := svc.CreateOrder(context.Background(), 1, 1, 2, 500, 0.05, nil)
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
	msg := &domain.OTCMessage{
		ID:              1,
		OrderID:         1,
		SenderID:        42,
		MessageType:     domain.OTCMessageTypeOffer,
		OfferRate:       ptr(0.05),
		OfferToAmount:   ptr(50.0),
		OfferFromAmount: ptr(1000.0),
		OfferStatus:     &offerStatus,
	}
	order := &domain.OTCOrderDetail{OTCOrder: domain.OTCOrder{ID: 1, Status: domain.OTCStatusNegotiating}}
	otcRepo := &mockOTCRepo{order: order, message: msg}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, newMockWalletRepo())

	err := svc.AcceptOffer(context.Background(), "abc", 1, 42) // same sender as acceptor
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

	err := svc.AcceptOffer(context.Background(), "abc", 1, 99)
	if err == nil || err.Error() != "offer is no longer pending" {
		t.Errorf("unexpected error: %v", err)
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
			FromAmount:     500.0,
			Status:         domain.OTCStatusNegotiating,
		},
	}
	wallets := newMockWalletRepo()
	wallets.setWallet(10, 1, &domain.Wallet{ID: 5, Locked: 500.0, Balance: 0})

	otcRepo := &mockOTCRepo{order: order}
	svc := makeOTCService(otcRepo, &mockUserRepo{}, wallets)

	if err := svc.CancelOrder(context.Background(), "abc", 10, "client", "changed mind"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wallets.unlockCalls) != 1 {
		t.Fatalf("expected 1 UnlockAmount call, got %d", len(wallets.unlockCalls))
	}
	if wallets.unlockCalls[0].walletID != 5 || wallets.unlockCalls[0].amount != 500.0 {
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
