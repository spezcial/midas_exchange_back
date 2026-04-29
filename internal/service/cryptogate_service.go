package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/cryptogate"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

// ErrUnsupportedCurrency is returned when the requested currency has no crypto-gate chain mapping.
// Handlers can check for this to return 400 rather than 500.
var ErrUnsupportedCurrency = errors.New("currency not supported by crypto gateway")

// CryptoGateService orchestrates deposit address management and on-chain withdrawals
// via the crypto-gate payment gateway.
type CryptoGateService struct {
	client       *cryptogate.Client
	platform     string // gateway platform slug, included in every API call
	addrRepo     *repository.DepositAddressRepository
	walletRepo   *repository.WalletRepository
	txRepo       *repository.TransactionRepository
	userRepo     repository.UserRepo
	twofaService TwoFactorService
	log          *logger.Logger
}

func NewCryptoGateService(
	client *cryptogate.Client,
	platform string,
	addrRepo *repository.DepositAddressRepository,
	walletRepo *repository.WalletRepository,
	txRepo *repository.TransactionRepository,
	userRepo repository.UserRepo,
	twofaService TwoFactorService,
	log *logger.Logger,
) *CryptoGateService {
	return &CryptoGateService{
		client:       client,
		platform:     platform,
		addrRepo:     addrRepo,
		walletRepo:   walletRepo,
		txRepo:       txRepo,
		userRepo:     userRepo,
		twofaService: twofaService,
		log:          log,
	}
}

// GetOrCreateDepositAddressByCode resolves the currency code to its DB ID then
// delegates to GetOrCreateDepositAddress. Used by the HTTP handler layer.
func (s *CryptoGateService) GetOrCreateDepositAddressByCode(ctx context.Context, userID int64, currencyCode string) (*domain.DepositAddress, error) {
	currency, err := s.walletRepo.GetCurrencyByCode(ctx, currencyCode)
	if err != nil {
		return nil, fmt.Errorf("currency %s not found: %w", currencyCode, err)
	}
	return s.GetOrCreateDepositAddress(ctx, userID, currencyCode, int64(currency.ID))
}

// GetOrCreateDepositAddress returns the user's deposit address for a currency,
// creating one via crypto-gate if it doesn't exist yet.
func (s *CryptoGateService) GetOrCreateDepositAddress(ctx context.Context, userID int64, currencyCode string, currencyID int64) (*domain.DepositAddress, error) {
	chainCfg, ok := domain.CurrencyChainFor(currencyCode)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCurrency, currencyCode)
	}

	// Check existing address
	existing, err := s.addrRepo.GetByUserAndCurrency(ctx, userID, currencyID)
	if err != nil {
		return nil, fmt.Errorf("deposit address lookup: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Create a new address via crypto-gate.
	// The gateway registers this address immediately in its deposit scanner — there is
	// no rollback. If our DB save fails the gateway address becomes orphaned (it will
	// receive deposits but we cannot credit anyone). We log this prominently so it can
	// be recovered manually if needed.
	address, err := s.client.GetDepositAddress(ctx, chainCfg.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate deposit address: %w", err)
	}

	a := &domain.DepositAddress{
		UserID:     userID,
		CurrencyID: currencyID,
		Chain:      chainCfg.Chain,
		Address:    address,
	}
	if err := s.addrRepo.Create(ctx, a); err != nil {
		if errors.Is(err, repository.ErrAddressConflict) {
			// Another concurrent request already created an address for this user+currency
			// while we were calling the gateway. The address we just got from the gateway
			// is now orphaned in the gateway's scanner — log it so it can be investigated.
			s.log.Warn("deposit address race: orphaned gateway address — log for manual review",
				"user_id", userID, "currency_id", currencyID, "chain", chainCfg.Chain, "orphaned_address", address)
			// Return the address that won the race.
			existing, readErr := s.addrRepo.GetByUserAndCurrency(ctx, userID, currencyID)
			if readErr != nil || existing == nil {
				return nil, fmt.Errorf("deposit address conflict but re-read failed: %w", readErr)
			}
			return existing, nil
		}
		// DB error after a successful gateway call — the gateway address is orphaned.
		s.log.Error("failed to save deposit address — gateway address orphaned, manual recovery needed",
			"user_id", userID, "currency_id", currencyID, "chain", chainCfg.Chain, "orphaned_address", address, "error", err)
		return nil, fmt.Errorf("failed to save deposit address: %w", err)
	}
	return a, nil
}

// AddressAuditResult is returned by AuditAddresses.
type AddressAuditResult struct {
	// GatewayTotal is the number of addresses the gateway has for this platform.
	GatewayTotal int `json:"gateway_total"`
	// LocalTotal is the number of addresses in our DB.
	LocalTotal int `json:"local_total"`
	// OrphanedInGateway are addresses the gateway has but we don't — we can't credit
	// deposits to these addresses. Requires manual investigation to assign to a user.
	OrphanedInGateway []string `json:"orphaned_in_gateway"`
	// MissingFromGateway are addresses in our DB that the gateway doesn't know about.
	// This should never happen; if it does, the address is dead (gateway won't scan it).
	MissingFromGateway []OrphanedLocalAddress `json:"missing_from_gateway"`
}

type OrphanedLocalAddress struct {
	Address    string `json:"address"`
	Chain      string `json:"chain"`
	UserID     int64  `json:"user_id"`
	CurrencyID int64  `json:"currency_id"`
}

// AuditAddresses compares the gateway's address list against our local DB.
// Returns counts and any discrepancies in both directions.
func (s *CryptoGateService) AuditAddresses(ctx context.Context) (*AddressAuditResult, error) {
	gatewayAddrs, err := s.client.ListPlatformAddresses(ctx)
	if err != nil {
		return nil, fmt.Errorf("audit: fetch gateway addresses: %w", err)
	}

	localAddrs, err := s.addrRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("audit: fetch local addresses: %w", err)
	}

	// Index local addresses by address string for O(1) lookup.
	localByAddr := make(map[string]domain.DepositAddress, len(localAddrs))
	for _, a := range localAddrs {
		localByAddr[a.Address] = a
	}

	// Index gateway addresses for O(1) lookup.
	gatewayByAddr := make(map[string]struct{}, len(gatewayAddrs))
	for _, a := range gatewayAddrs {
		gatewayByAddr[a.Address] = struct{}{}
	}

	result := &AddressAuditResult{
		GatewayTotal: len(gatewayAddrs),
		LocalTotal:   len(localAddrs),
	}

	// Addresses in gateway but not in our DB.
	for _, a := range gatewayAddrs {
		if _, ok := localByAddr[a.Address]; !ok {
			result.OrphanedInGateway = append(result.OrphanedInGateway, a.Address)
		}
	}

	// Addresses in our DB but not in gateway — should never happen.
	for _, a := range localAddrs {
		if _, ok := gatewayByAddr[a.Address]; !ok {
			result.MissingFromGateway = append(result.MissingFromGateway, OrphanedLocalAddress{
				Address:    a.Address,
				Chain:      a.Chain,
				UserID:     a.UserID,
				CurrencyID: a.CurrencyID,
			})
		}
	}

	return result, nil
}

// InitiateWithdrawal sends the withdrawal to crypto-gate.
// The balance was already deducted by WalletService.Withdraw before this is called.
// On crypto-gate failure: refunds atomically and marks transaction failed.
// Final status (confirmed/failed) arrives via the withdrawal webhook.
func (s *CryptoGateService) InitiateWithdrawal(
	ctx context.Context,
	tx *domain.Transaction,
	toAddress string,
	currencyCode string,
) error {
	chainCfg, ok := domain.CurrencyChainFor(currencyCode)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedCurrency, currencyCode)
	}

	amountStr := strconv.FormatFloat(tx.Amount, 'f', -1, 64)
	txHash, err := s.client.Withdraw(ctx, cryptogate.WithdrawRequest{
		UUID:     strconv.FormatInt(tx.ID, 10),
		Address:  toAddress,
		Amount:   amountStr,
		Asset:    chainCfg.Asset,
		Platform: s.platform,
	})
	if err != nil {
		// Mark failed first so the withdrawal webhook (if it ever arrives) skips double-refund.
		tx.Status = domain.TransactionStatusFailed
		if uErr := s.txRepo.Update(ctx, tx); uErr != nil {
			s.log.Error("InitiateWithdrawal: failed to mark tx failed", "tx_id", tx.ID, "error", uErr)
		}
		// Refund atomically — no read-modify-write.
		if cErr := s.walletRepo.AtomicCredit(ctx, tx.WalletID, tx.Amount+tx.Fee); cErr != nil {
			s.log.Error("InitiateWithdrawal: refund failed — MANUAL INTERVENTION REQUIRED",
				"tx_id", tx.ID, "wallet_id", tx.WalletID, "amount", tx.Amount+tx.Fee, "error", cErr)
		}
		return fmt.Errorf("crypto-gate withdrawal failed: %w", err)
	}

	// Store the initial hash returned by crypto-gate (may be empty until confirmed).
	if txHash != "" {
		tx.TxHash = &txHash
		if uErr := s.txRepo.Update(ctx, tx); uErr != nil {
			s.log.Error("InitiateWithdrawal: failed to store tx hash", "tx_id", tx.ID, "error", uErr)
		}
	}
	return nil
}

// HandleDepositWebhook processes an inbound deposit notification from crypto-gate.
// It credits the user's wallet and records a completed deposit transaction atomically.
func (s *CryptoGateService) HandleDepositWebhook(ctx context.Context, payload domain.DepositWebhook) error {
	// Resolve address → user + currency
	addrRecord, err := s.addrRepo.GetByAddress(ctx, payload.Address)
	if err != nil {
		return fmt.Errorf("deposit webhook: address lookup: %w", err)
	}
	if addrRecord == nil {
		return fmt.Errorf("deposit webhook: unknown address %s", payload.Address)
	}

	wallet, err := s.walletRepo.GetByUserAndCurrency(ctx, addrRecord.UserID, int32(addrRecord.CurrencyID))
	if err != nil {
		return fmt.Errorf("deposit webhook: wallet lookup: %w", err)
	}

	// Idempotency: skip if we already recorded this tx hash.
	if payload.Hash != "" {
		exists, err := s.txRepo.ExistsByTxHash(ctx, payload.Hash)
		if err != nil {
			return fmt.Errorf("deposit webhook: idempotency check: %w", err)
		}
		if exists {
			s.log.Info("deposit webhook: duplicate tx_hash, skipping", "hash", payload.Hash)
			return nil
		}
	}

	// Parse the amount — the gateway sends it as a decimal string, e.g. "100.000000000".
	amount, err := strconv.ParseFloat(payload.Amount, 64)
	if err != nil || amount <= 0 {
		return fmt.Errorf("deposit webhook: invalid amount %q: %w", payload.Amount, err)
	}

	// RecordDeposit wraps the transaction insert and balance credit in a single DB
	// transaction — a crash between the two writes cannot leave them inconsistent.
	tx, err := s.walletRepo.RecordDeposit(ctx, addrRecord.UserID, wallet.ID, amount, hashPtr(payload.Hash))
	if err != nil {
		return fmt.Errorf("deposit webhook: record deposit: %w", err)
	}

	s.log.Info("deposit credited",
		"user_id", addrRecord.UserID,
		"tx_id", tx.ID,
		"amount", payload.Amount,
		"asset", payload.Asset,
		"hash", payload.Hash,
	)

	// Fire-and-forget Telegram notification if the user has a verified phone.
	go s.notifyDeposit(addrRecord.UserID, payload.Amount, payload.Asset)

	return nil
}

func (s *CryptoGateService) notifyDeposit(userID int64, amount, asset string) {
	if s.twofaService == nil || s.userRepo == nil {
		return
	}
	ctx := context.Background()
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user.Phone == nil || !user.PhoneVerified {
		return
	}
	msg := fmt.Sprintf("Your CaspianEx account has been credited: %s %s", amount, asset)
	s.twofaService.SendSecurityAlert(ctx, *user.Phone, msg)
}

// HandleWithdrawWebhook processes an inbound withdrawal status update from crypto-gate.
// On confirmed: marks transaction completed.
// On failed: marks transaction failed and refunds the user's balance atomically.
func (s *CryptoGateService) HandleWithdrawWebhook(ctx context.Context, payload domain.WithdrawWebhook) error {
	txID, err := strconv.ParseInt(payload.UUID, 10, 64)
	if err != nil {
		return fmt.Errorf("withdraw webhook: invalid UUID %q: %w", payload.UUID, err)
	}

	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("withdraw webhook: transaction %d not found: %w", txID, err)
	}

	// Idempotency guard: if the transaction already reached a terminal state
	// (e.g. this webhook fired twice, or InitiateWithdrawal already marked it failed),
	// skip all action to prevent double-refunds or overwriting completed status.
	if tx.Status != domain.TransactionStatusPending {
		s.log.Info("withdraw webhook: tx not pending, skipping",
			"tx_id", txID, "status", tx.Status, "webhook_status", payload.Status)
		return nil
	}

	// Gateway sends "success" or "failed" — no intermediate "pending" status.
	switch payload.Status {
	case "success":
		tx.Status = domain.TransactionStatusCompleted
		tx.TxHash = hashPtr(payload.Hash)
		if err := s.txRepo.Update(ctx, tx); err != nil {
			return fmt.Errorf("withdraw webhook: update transaction: %w", err)
		}
		s.log.Info("withdrawal confirmed", "tx_id", txID, "hash", payload.Hash)

	case "failed":
		tx.Status = domain.TransactionStatusFailed
		tx.TxHash = hashPtr(payload.Hash)
		if err := s.txRepo.Update(ctx, tx); err != nil {
			return fmt.Errorf("withdraw webhook: mark transaction failed: %w", err)
		}
		// Refund atomically — no read-modify-write.
		if err := s.walletRepo.AtomicCredit(ctx, tx.WalletID, tx.Amount+tx.Fee); err != nil {
			// Do NOT return nil here — that would ack the webhook and forfeit retries.
			return fmt.Errorf("withdraw webhook: refund credit failed — MANUAL INTERVENTION REQUIRED: tx_id=%d wallet_id=%d amount=%v: %w",
				txID, tx.WalletID, tx.Amount+tx.Fee, err)
		}
		s.log.Info("withdrawal failed — balance refunded", "tx_id", txID)

	default:
		s.log.Warn("withdraw webhook: unrecognised status, ignoring", "tx_id", txID, "status", payload.Status)
	}

	return nil
}

// hashPtr converts a blockchain hash string to a pointer, returning nil for empty strings.
func hashPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
