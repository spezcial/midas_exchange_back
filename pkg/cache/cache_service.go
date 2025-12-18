package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/logger"
)

type WriteOperation struct {
	Type   string
	Key    string
	Value  interface{}
	Action string // "insert", "update", "delete"
}

type CacheService struct {
	cache       *MemoryCache
	logger      *logger.Logger
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	workerCount int
}

const NoExpiration time.Duration = -1

func NewCacheService(cache *MemoryCache, logger *logger.Logger) *CacheService {
	ctx, cancel := context.WithCancel(context.Background())
	return &CacheService{
		cache:  cache,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// User cache operations
func (cs *CacheService) GetUser(userID int64) (*domain.User, bool) {
	key := fmt.Sprintf("user:%d", userID)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedUser, ok := val.(*domain.User)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	userCopy := *cachedUser
	return &userCopy, true
}

func (cs *CacheService) GetUserByEmail(email string) (*domain.User, bool) {
	key := fmt.Sprintf("user:email:%s", email)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedUser, ok := val.(*domain.User)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	userCopy := *cachedUser
	return &userCopy, true
}

func (cs *CacheService) SetUser(user *domain.User) {
	keyByID := fmt.Sprintf("user:%d", user.ID)
	keyByEmail := fmt.Sprintf("user:email:%s", user.Email)

	// No TTL - synced with DB via cache_writer
	cs.cache.Set(keyByID, user, NoExpiration)
	cs.cache.Set(keyByEmail, user, NoExpiration)
}

// Currency cache operations
func (cs *CacheService) GetCurrency(code string) (*domain.Currency, bool) {
	key := fmt.Sprintf("currency:%s", code)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedCurrency, ok := val.(*domain.Currency)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	currencyCopy := *cachedCurrency
	return &currencyCopy, true
}

func (cs *CacheService) GetAllCurrencies() ([]domain.Currency, bool) {
	val, found := cs.cache.Get("currencies:all")
	if !found {
		return nil, false
	}
	currencies, ok := val.([]domain.Currency)
	return currencies, ok
}

func (cs *CacheService) SetCurrency(currency *domain.Currency) {
	key := fmt.Sprintf("currency:%s", currency.Code)
	// No TTL - synced with DB via cache_writer
	cs.cache.Set(key, currency, 0)
}

func (cs *CacheService) SetAllCurrencies(currencies []domain.Currency) {
	// No TTL - synced with DB via cache_writer
	cs.cache.Set("currencies:all", currencies, 0)
	for i := range currencies {
		cs.SetCurrency(&currencies[i])
	}
}

// Exchange Rate cache operations
func (cs *CacheService) GetExchangeRate(fromCurrencyID, toCurrencyID int32) (*domain.ExchangeRate, bool) {
	key := fmt.Sprintf("exchange_rate:%d:%d", fromCurrencyID, toCurrencyID)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedRate, ok := val.(*domain.ExchangeRate)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	rateCopy := *cachedRate
	return &rateCopy, true
}

// Exchange Rate cache operations
func (cs *CacheService) GetExchangeRateById(id int64) (*domain.ExchangeRate, bool) {
	key := fmt.Sprintf("exchange_rate:id:%d", id)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedRate, ok := val.(*domain.ExchangeRate)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	rateCopy := *cachedRate
	return &rateCopy, true
}

func (cs *CacheService) SetExchangeRate(rate *domain.ExchangeRate) {
	key := fmt.Sprintf("exchange_rate:%d:%d", rate.FromCurrencyID, rate.ToCurrencyID)
	// No TTL - synced with DB via cache_writer
	cs.cache.Set(key, rate, 0)
}

// UpdateExchangeRateCache updates all cache keys for a single exchange rate
// This should be called after any Create/Update operation
func (cs *CacheService) UpdateExchangeRateCache(rate *domain.ExchangeRate) {
	// Update cache by pair (from_currency_id, to_currency_id)
	keyByPair := fmt.Sprintf("exchange_rate:%d:%d", rate.FromCurrencyID, rate.ToCurrencyID)
	cs.cache.Set(keyByPair, rate, NoExpiration)

	// Update cache by ID
	keyByID := fmt.Sprintf("exchange_rate:id:%d", rate.ID)
	cs.cache.Set(keyByID, rate, NoExpiration)

	// Invalidate aggregate caches (they need to be reloaded)
	cs.cache.Delete("exchange_rates:all")
	cs.cache.Delete("exchange_rates:active")
}

// InvalidateExchangeRateCache removes all cache keys for a specific exchange rate
// This should be called after Delete operation
func (cs *CacheService) InvalidateExchangeRateCache(fromCurrencyID, toCurrencyID int32, id int64) {
	// Delete cache by pair
	keyByPair := fmt.Sprintf("exchange_rate:%d:%d", fromCurrencyID, toCurrencyID)
	cs.cache.Delete(keyByPair)

	// Delete cache by ID
	keyByID := fmt.Sprintf("exchange_rate:id:%d", id)
	cs.cache.Delete(keyByID)

	// Invalidate aggregate caches
	cs.cache.Delete("exchange_rates:all")
	cs.cache.Delete("exchange_rates:active")
}

// InvalidateExchangeRateListCaches invalidates only the aggregate list caches
// Useful when multiple rates are updated at once (batch operations)
func (cs *CacheService) InvalidateExchangeRateListCaches() {
	cs.cache.Delete("exchange_rates:all")
	cs.cache.Delete("exchange_rates:active")
}

// UpdateExchangeRateCacheOnly updates individual cache keys without invalidating lists
// Useful for batch operations where you want to invalidate lists only once at the end
func (cs *CacheService) UpdateExchangeRateCacheOnly(rate *domain.ExchangeRate) {
	// Update cache by pair (from_currency_id, to_currency_id)
	keyByPair := fmt.Sprintf("exchange_rate:%d:%d", rate.FromCurrencyID, rate.ToCurrencyID)
	cs.cache.Set(keyByPair, rate, NoExpiration)

	// Update cache by ID
	keyByID := fmt.Sprintf("exchange_rate:id:%d", rate.ID)
	cs.cache.Set(keyByID, rate, NoExpiration)
}

// Wallet cache operations
func (cs *CacheService) GetWallet(userID int64, currencyID int32) (*domain.Wallet, bool) {
	key := fmt.Sprintf("wallet:%d:%d", userID, currencyID)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedWallet, ok := val.(*domain.Wallet)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	walletCopy := *cachedWallet
	return &walletCopy, true
}

func (cs *CacheService) GetUserWallets(userID int64) ([]domain.WalletWithCurrency, bool) {
	key := fmt.Sprintf("wallets:user:%d", userID)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	wallets, ok := val.([]domain.WalletWithCurrency)
	return wallets, ok
}

func (cs *CacheService) SetWallet(wallet *domain.Wallet) {
	key := fmt.Sprintf("wallet:%d:%d", wallet.UserID, wallet.CurrencyID)
	// No TTL - synced with DB via cache_writer
	cs.cache.Set(key, wallet, 0)

	// Invalidate user wallets cache
	userWalletsKey := fmt.Sprintf("wallets:user:%d", wallet.UserID)
	cs.cache.Delete(userWalletsKey)
}

func (cs *CacheService) SetUserWallets(userID int64, wallets []domain.WalletWithCurrency) {
	key := fmt.Sprintf("wallets:user:%d", userID)
	// No TTL - synced with DB via cache_writer
	cs.cache.Set(key, wallets, 0)
}

// Session cache operations
func (cs *CacheService) GetSession(token string) (*domain.UserSession, bool) {
	key := fmt.Sprintf("session:%s", token)
	val, found := cs.cache.Get(key)
	if !found {
		return nil, false
	}
	cachedSession, ok := val.(*domain.UserSession)
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutations affecting the cache
	sessionCopy := *cachedSession
	return &sessionCopy, true
}

func (cs *CacheService) SetSession(session *domain.UserSession, ttl time.Duration) {
	key := fmt.Sprintf("session:%s", session.RefreshToken)
	cs.cache.Set(key, session, ttl)
}

func (cs *CacheService) DeleteSession(token string) {
	key := fmt.Sprintf("session:%s", token)
	cs.cache.Delete(key)
}

func (cs *CacheService) GetAllExchangeRates() ([]domain.ExchangeRateWithCurrencies, bool) {
	val, found := cs.cache.Get("exchange_rates:all")
	if !found {
		return nil, false
	}
	rates, ok := val.([]domain.ExchangeRateWithCurrencies)
	return rates, ok
}

func (cs *CacheService) GetActiveExchangeRates() ([]domain.ExchangeRateWithCurrencies, bool) {
	val, found := cs.cache.Get("exchange_rates:active")
	if !found {
		return nil, false
	}
	rates, ok := val.([]domain.ExchangeRateWithCurrencies)
	return rates, ok
}

func (cs *CacheService) SetAllExchangeRates(rates []domain.ExchangeRateWithCurrencies) {
	cs.cache.Set("exchange_rates:all", rates, NoExpiration)
}

func (cs *CacheService) SetActiveExchangeRates(rates []domain.ExchangeRateWithCurrencies) {
	cs.cache.Set("exchange_rates:active", rates, NoExpiration)
}

func (cs *CacheService) DeleteExchangeRate(from, to int32) {
	key := fmt.Sprintf("exchange_rate:%d:%d", from, to)
	cs.cache.Delete(key)

	// Invalidate list caches
	cs.cache.Delete("exchange_rates:all")
	cs.cache.Delete("exchange_rates:active")
}

// Utility functions
func (cs *CacheService) InvalidateUser(userID int64) {
	keyByID := fmt.Sprintf("user:%d", userID)
	cs.cache.Delete(keyByID)
}
