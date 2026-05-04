package service

import (
	"context"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/shopspring/decimal"
)

type PlatformFeeService struct {
	repo *repository.PlatformFeeRepository
	log  *logger.Logger
}

func NewPlatformFeeService(repo *repository.PlatformFeeRepository, log *logger.Logger) *PlatformFeeService {
	return &PlatformFeeService{repo: repo, log: log}
}

// Record saves a fee entry asynchronously so it never blocks the caller.
// grossAmount is the full transaction amount in the fee currency (before fee deduction).
// Note: failed writes are only logged — there is no retry. Reconcile against
// the currency_exchanges / transactions tables if records appear missing.
func (s *PlatformFeeService) Record(userID int64, operation domain.FeeOperation, currencyID int32, grossAmount, fee decimal.Decimal) {
	go func() {
		entry := &domain.PlatformFee{
			UserID:      userID,
			Operation:   operation,
			CurrencyID:  currencyID,
			GrossAmount: grossAmount,
			Fee:         fee,
		}
		if err := s.repo.Create(context.Background(), entry); err != nil {
			s.log.Error("failed to record platform fee", "operation", operation, "user_id", userID, "fee", fee, "error", err)
		}
	}()
}

func (s *PlatformFeeService) List(ctx context.Context, limit, offset int) ([]domain.PlatformFeeWithDetails, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *PlatformFeeService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *PlatformFeeService) Totals(ctx context.Context) (map[string]decimal.Decimal, error) {
	exchangeTotal, withdrawalTotal, err := s.repo.Totals(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]decimal.Decimal{
		string(domain.FeeOperationExchange):   exchangeTotal,
		string(domain.FeeOperationWithdrawal): withdrawalTotal,
	}, nil
}
