package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

const BINANCE_API = `https://api.binance.com/api/v3/ticker/price?symbol=%s`

// RateUpdaterConfig holds configuration for the rate updater worker
type RateUpdaterConfig struct {
	UpdateInterval time.Duration // How often to update rates
	UpdateTimeout  time.Duration // Timeout for each update operation
	MaxRetries     int           // Maximum number of retries on failure
	RetryBackoff   time.Duration // Base backoff duration between retries
}

// DefaultRateUpdaterConfig returns sensible defaults
func DefaultRateUpdaterConfig() RateUpdaterConfig {
	return RateUpdaterConfig{
		UpdateInterval: 2 * time.Minute,
		UpdateTimeout:  30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   5 * time.Second,
	}
}

// RateUpdater is a background worker that periodically updates exchange rates
type RateUpdater struct {
	config          RateUpdaterConfig
	exchangeService *service.ExchangeRatesService
	log             *logger.Logger

	// State management
	running     atomic.Bool
	mu          sync.Mutex
	lastRunTime time.Time
	lastError   error
	runCount    uint64
	failCount   uint64

	// Lifecycle
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewRateUpdater creates a new rate updater worker
func NewRateUpdater(
	config RateUpdaterConfig,
	exchangeService *service.ExchangeRatesService,
	log *logger.Logger,
) *RateUpdater {
	return &RateUpdater{
		config:          config,
		exchangeService: exchangeService,
		log:             log,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
	}
}

// Start begins the background update process
func (ru *RateUpdater) Start(ctx context.Context) {
	if !ru.running.CompareAndSwap(false, true) {
		ru.log.Warn("Rate updater is already running")
		return
	}

	ru.log.Info("Starting exchange rate updater",
		"interval", ru.config.UpdateInterval,
		"timeout", ru.config.UpdateTimeout,
		"max_retries", ru.config.MaxRetries,
	)

	go ru.run(ctx)
}

// Stop gracefully stops the background updater
func (ru *RateUpdater) Stop() {
	if !ru.running.Load() {
		return
	}

	ru.log.Info("Stopping exchange rate updater")
	close(ru.stopChan)

	// Wait for the worker to finish with timeout
	select {
	case <-ru.doneChan:
		ru.log.Info("Exchange rate updater stopped gracefully")
	case <-time.After(10 * time.Second):
		ru.log.Warn("Exchange rate updater stop timeout")
	}
}

// run is the main worker loop
func (ru *RateUpdater) run(ctx context.Context) {
	defer close(ru.doneChan)
	defer ru.running.Store(false)

	ticker := time.NewTicker(ru.config.UpdateInterval)
	defer ticker.Stop()

	// Run immediately on startup
	ru.executeUpdate(ctx)

	for {
		select {
		case <-ctx.Done():
			ru.log.Info("Rate updater stopped due to context cancellation")
			return

		case <-ru.stopChan:
			ru.log.Info("Rate updater stopped via Stop()")
			return

		case <-ticker.C:
			ru.executeUpdate(ctx)
		}
	}
}

// BinanceTickerResponse represents the Binance API response
type BinanceTickerResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// fetchBinancePrice fetches price from Binance API for a given symbol
func (ru *RateUpdater) fetchBinancePrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf(BINANCE_API, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch from Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("binance API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var ticker BinanceTickerResponse
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price: %w", err)
	}

	return price, nil
}

// getBinanceRate fetches exchange rate from Binance
// First tries direct pair (e.g., BTCETH), if fails, calculates via USDT
func (ru *RateUpdater) getBinanceRate(ctx context.Context, fromCode, toCode string) (float64, error) {
	// Normalize currency codes to uppercase
	fromCode = strings.ToUpper(fromCode)
	toCode = strings.ToUpper(toCode)

	// Try direct pair first (FROM/TO)
	directSymbol := fromCode + toCode
	price, err := ru.fetchBinancePrice(ctx, directSymbol)
	if err == nil {
		ru.log.Debug("Fetched direct pair from Binance", "symbol", directSymbol, "price", price)
		return price, nil
	}

	// Try reverse pair (TO/FROM) and invert
	reverseSymbol := toCode + fromCode
	price, err = ru.fetchBinancePrice(ctx, reverseSymbol)
	if err == nil {
		if price == 0 {
			return 0, fmt.Errorf("reverse pair price is zero")
		}
		invertedPrice := 1.0 / price
		ru.log.Debug("Fetched reverse pair from Binance", "symbol", reverseSymbol, "price", price, "inverted", invertedPrice)
		return invertedPrice, nil
	}

	// If direct pair fails, calculate via USDT
	ru.log.Debug("Direct pair not found, calculating via USDT", "from", fromCode, "to", toCode)

	// Fetch FROM/USDT
	fromUSDTSymbol := fromCode + "USDT"
	fromUSDTPrice, err := ru.fetchBinancePrice(ctx, fromUSDTSymbol)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch %s: %w", fromUSDTSymbol, err)
	}

	// Fetch TO/USDT
	toUSDTSymbol := toCode + "USDT"
	toUSDTPrice, err := ru.fetchBinancePrice(ctx, toUSDTSymbol)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch %s: %w", toUSDTSymbol, err)
	}

	if toUSDTPrice == 0 {
		return 0, fmt.Errorf("to currency USDT price is zero")
	}

	// Calculate cross rate: (FROM/USDT) / (TO/USDT) = FROM/TO
	crossRate := fromUSDTPrice / toUSDTPrice

	return crossRate, nil
}

// executeUpdate performs a single update with retry logic
func (ru *RateUpdater) executeUpdate(parentCtx context.Context) {
	// Prevent concurrent updates
	if !ru.mu.TryLock() {
		ru.log.Warn("Skipping update - previous update still in progress")
		return
	}
	defer ru.mu.Unlock()

	atomic.AddUint64(&ru.runCount, 1)
	startTime := time.Now()

	ru.log.Debug("Starting exchange rate update")

	var lastErr error
	for attempt := 0; attempt <= ru.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := ru.config.RetryBackoff * time.Duration(1<<uint(attempt-1)) // Exponential backoff
			ru.log.Info("Retrying rate update",
				"attempt", attempt,
				"max_retries", ru.config.MaxRetries,
				"backoff", backoff,
			)
			time.Sleep(backoff)
		}

		// Create context with timeout for this update
		updateCtx, cancel := context.WithTimeout(parentCtx, ru.config.UpdateTimeout)
		err := ru.performUpdate(updateCtx)
		cancel()

		if err == nil {
			duration := time.Since(startTime)
			ru.lastRunTime = time.Now()
			ru.lastError = nil

			ru.log.Info("Exchange rate update completed successfully",
				"duration_ms", duration.Milliseconds(),
				"total_runs", atomic.LoadUint64(&ru.runCount),
			)
			return
		}

		lastErr = err
		ru.log.Error("Exchange rate update failed",
			"error", err,
			"attempt", attempt+1,
			"max_attempts", ru.config.MaxRetries+1,
		)
	}

	// All retries failed
	atomic.AddUint64(&ru.failCount, 1)
	ru.lastError = lastErr
	ru.log.Error("Exchange rate update failed after all retries",
		"error", lastErr,
		"total_failures", atomic.LoadUint64(&ru.failCount),
	)
}

// performUpdate fetches rates from Binance and updates the database
func (ru *RateUpdater) performUpdate(ctx context.Context) error {
	// 1. Fetch all active exchange rates from the database
	rates, err := ru.exchangeService.GetActiveRatesWithCurrencies(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active rates: %w", err)
	}

	if len(rates) == 0 {
		ru.log.Info("No active exchange rates to update")
		return nil
	}

	ru.log.Info("Fetching rates from Binance", "pairs_count", len(rates))

	// 2. Fetch rates from Binance for each pair
	updates := make([]repository.RateUpdateData, 0, len(rates))
	successCount := 0
	failCount := 0

	for _, rate := range rates {
		// TODO: Handle fiat currencies
		if rate.FromCurrency.IsCrypto || rate.ToCurrency.IsCrypto {
			continue
		}
		// Fetch rate from Binance
		newRate, err := ru.getBinanceRate(ctx, rate.FromCurrency.Code, rate.ToCurrency.Code)
		if err != nil {
			ru.log.Warn("Failed to fetch rate from Binance",
				"from", rate.FromCurrency.Code,
				"to", rate.ToCurrency.Code,
				"error", err,
			)
			failCount++
			continue
		}

		updates = append(updates, repository.RateUpdateData{
			ID:   rate.ID,
			Rate: newRate,
		})
		successCount++

		ru.log.Debug("Fetched rate from Binance",
			"from", rate.FromCurrency.Code,
			"to", rate.ToCurrency.Code,
			"rate", newRate,
		)
	}

	ru.log.Info("Binance rate fetch completed",
		"total", len(rates),
		"success", successCount,
		"failed", failCount,
	)

	// 3. Batch update all rates in the database
	if len(updates) > 0 {
		if err := ru.exchangeService.BatchUpdateRates(ctx, updates); err != nil {
			return fmt.Errorf("failed to batch update rates: %w", err)
		}

		ru.log.Info("Successfully updated rates in database", "count", len(updates))
	} else {
		ru.log.Warn("No rates were fetched successfully from Binance")
	}

	return nil
}

// HealthStatus represents the health status of the worker
type HealthStatus struct {
	Running     bool      `json:"running"`
	LastRunTime time.Time `json:"last_run_time"`
	LastError   string    `json:"last_error,omitempty"`
	RunCount    uint64    `json:"run_count"`
	FailCount   uint64    `json:"fail_count"`
	Uptime      string    `json:"uptime,omitempty"`
}

// Health returns the current health status of the worker
func (ru *RateUpdater) Health() HealthStatus {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	status := HealthStatus{
		Running:     ru.running.Load(),
		LastRunTime: ru.lastRunTime,
		RunCount:    atomic.LoadUint64(&ru.runCount),
		FailCount:   atomic.LoadUint64(&ru.failCount),
	}

	if ru.lastError != nil {
		status.LastError = ru.lastError.Error()
	}

	if !ru.lastRunTime.IsZero() {
		status.Uptime = time.Since(ru.lastRunTime).String()
	}

	return status
}

// IsHealthy returns true if the worker is running and hasn't failed recently
func (ru *RateUpdater) IsHealthy() error {
	if !ru.running.Load() {
		return fmt.Errorf("rate updater is not running")
	}

	ru.mu.Lock()
	defer ru.mu.Unlock()

	// Consider unhealthy if last update failed and it's been too long
	if ru.lastError != nil && time.Since(ru.lastRunTime) > ru.config.UpdateInterval*2 {
		return fmt.Errorf("last update failed and stale: %w", ru.lastError)
	}

	return nil
}
