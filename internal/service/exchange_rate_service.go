package service

import (
	"context"
	"fmt"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/models"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

type ExchangeRatesService struct {
	ratesRepo *repository.ExchangeRateRepository
	log       *logger.Logger
}

func NewExchangeRatesService(ratesRepo *repository.ExchangeRateRepository, log *logger.Logger) *ExchangeRatesService {
	return &ExchangeRatesService{
		ratesRepo: ratesRepo,
		log:       log,
	}
}

// Public methods
func (s *ExchangeRatesService) GetActiveRates(ctx context.Context) ([]domain.ExchangeRateWithCurrencies, error) {
	return s.ratesRepo.GetActive(ctx)
}

// Admin methods
func (s *ExchangeRatesService) GetAllRates(ctx context.Context) ([]domain.ExchangeRateWithCurrencies, error) {
	return s.ratesRepo.GetAll(ctx)
}

func (s *ExchangeRatesService) GetRateByPair(ctx context.Context, fromId, toId int32) (*domain.ExchangeRate, error) {
	return s.ratesRepo.GetByPair(ctx, fromId, toId)
}

func (s *ExchangeRatesService) GetRateByID(ctx context.Context, id int64) (*domain.ExchangeRate, error) {
	return s.ratesRepo.GetByID(ctx, id)
}

func (s *ExchangeRatesService) CreateRate(ctx context.Context, req *models.CreateExchangeRatesRequest) (*domain.ExchangeRate, error) {
	if req.FromCurrencyID == req.ToCurrencyID {
		return nil, fmt.Errorf("base and quote currencies must be different")
	}

	rate := &domain.ExchangeRate{
		FromCurrencyID: req.FromCurrencyID,
		ToCurrencyID:   req.ToCurrencyID,
		Fee:            req.Fee,
		Rate:           req.Rate,
		IsActive:       req.IsActive,
	}

	if err := s.ratesRepo.Create(ctx, rate); err != nil {
		return nil, fmt.Errorf("failed to create exchange rate: %w", err)
	}

	return rate, nil
}

func (s *ExchangeRatesService) UpdateRate(ctx context.Context, id int64, req *models.UpdateExchangeRatesRequest) (*domain.ExchangeRate, error) {
	rate, err := s.ratesRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	rate.Fee = req.Fee
	rate.IsActive = req.IsActive

	if err := s.ratesRepo.Update(ctx, rate); err != nil {
		return nil, fmt.Errorf("failed to update exchange rate: %w", err)
	}

	return rate, nil
}

func (s *ExchangeRatesService) DeleteRate(ctx context.Context, id int64) error {
	return s.ratesRepo.Delete(ctx, id)
}

// BatchUpdateRates updates multiple exchange rates in a single database transaction
func (s *ExchangeRatesService) BatchUpdateRates(ctx context.Context, updates []repository.RateUpdateData) error {
	if len(updates) == 0 {
		return nil
	}

	s.log.Info("Batch updating exchange rates", "count", len(updates))

	if err := s.ratesRepo.BatchUpdate(ctx, updates); err != nil {
		return fmt.Errorf("failed to batch update rates: %w", err)
	}

	s.log.Info("Batch update completed successfully")
	return nil
}

// GetActiveRatesWithCurrencies returns all active exchange rates with currency details
func (s *ExchangeRatesService) GetActiveRatesWithCurrencies(ctx context.Context) ([]domain.ExchangeRateWithCurrencies, error) {
	return s.ratesRepo.GetActive(ctx)
}

// UpdateAllRates is called by the worker - it should not contain API logic
// The worker handles fetching rates from Binance and calls BatchUpdateRates
func (s *ExchangeRatesService) UpdateAllRates(ctx context.Context) error {
	// This method is now a placeholder - the actual logic is in the worker
	// keeping it for backward compatibility
	return nil
}
