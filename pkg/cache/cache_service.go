package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
)

const (
	// l1TTL is the lifetime of exchange-rate and currency entries in the L1 cache.
	// Short enough that a rate update (every 2 min) is reflected quickly; long enough
	// to absorb request bursts without hitting Redis on every call.
	l1TTL = 30 * time.Second
)

// CacheService provides typed read/write helpers over a primary Store (Redis)
// with an L1 in-memory cache for exchange rates and currencies.
type CacheService struct {
	store Store        // primary store — Redis
	l1    *MemoryCache // L1 in-memory cache for hot read-only data
}

func NewCacheService(store Store) *CacheService {
	// L1 is always in-memory; entries expire after l1TTL so they stay fresh.
	l1 := NewMemoryCache(l1TTL, 2*l1TTL)
	return &CacheService{store: store, l1: l1}
}

// ---- helpers ----------------------------------------------------------------

func marshalSet(ctx context.Context, s Store, key string, v interface{}, ttl time.Duration) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = s.Set(ctx, key, b, ttl)
}

func unmarshalGet[T any](ctx context.Context, s Store, key string) (*T, bool) {
	b, found, err := s.Get(ctx, key)
	if err != nil || !found {
		return nil, false
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, false
	}
	return &v, true
}

func unmarshalSliceGet[T any](ctx context.Context, s Store, key string) ([]T, bool) {
	b, found, err := s.Get(ctx, key)
	if err != nil || !found {
		return nil, false
	}
	var v []T
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, false
	}
	return v, true
}

// ---- User -------------------------------------------------------------------

func (cs *CacheService) GetUser(userID int64) (*domain.User, bool) {
	key := fmt.Sprintf("user:%d", userID)
	return unmarshalGet[domain.User](context.Background(), cs.store, key)
}

func (cs *CacheService) GetUserByEmail(email string) (*domain.User, bool) {
	key := fmt.Sprintf("user:email:%s", email)
	return unmarshalGet[domain.User](context.Background(), cs.store, key)
}

func (cs *CacheService) SetUser(user *domain.User) {
	ctx := context.Background()
	marshalSet(ctx, cs.store, fmt.Sprintf("user:%d", user.ID), user, NoExpiration)
	marshalSet(ctx, cs.store, fmt.Sprintf("user:email:%s", user.Email), user, NoExpiration)
}

func (cs *CacheService) InvalidateUser(userID int64) {
	_ = cs.store.Delete(context.Background(), fmt.Sprintf("user:%d", userID))
}

func (cs *CacheService) InvalidateUserEmail(email string) {
	_ = cs.store.Delete(context.Background(), fmt.Sprintf("user:email:%s", email))
}

// ---- Currency (L1 + Redis) --------------------------------------------------

func (cs *CacheService) GetCurrency(code string) (*domain.Currency, bool) {
	key := fmt.Sprintf("currency:%s", code)
	ctx := context.Background()
	// L1 first
	if v, ok := unmarshalGet[domain.Currency](ctx, cs.l1, key); ok {
		return v, true
	}
	// Redis fallback
	v, ok := unmarshalGet[domain.Currency](ctx, cs.store, key)
	if ok {
		marshalSet(ctx, cs.l1, key, v, l1TTL)
	}
	return v, ok
}

func (cs *CacheService) GetAllCurrencies() ([]domain.Currency, bool) {
	key := "currencies:all"
	ctx := context.Background()
	if v, ok := unmarshalSliceGet[domain.Currency](ctx, cs.l1, key); ok {
		return v, true
	}
	v, ok := unmarshalSliceGet[domain.Currency](ctx, cs.store, key)
	if ok {
		b, _ := json.Marshal(v)
		_ = cs.l1.Set(ctx, key, b, l1TTL)
	}
	return v, ok
}

func (cs *CacheService) SetCurrency(currency *domain.Currency) {
	key := fmt.Sprintf("currency:%s", currency.Code)
	ctx := context.Background()
	marshalSet(ctx, cs.store, key, currency, NoExpiration)
	marshalSet(ctx, cs.l1, key, currency, l1TTL)
}

func (cs *CacheService) SetAllCurrencies(currencies []domain.Currency) {
	ctx := context.Background()
	marshalSet(ctx, cs.store, "currencies:all", currencies, NoExpiration)
	marshalSet(ctx, cs.l1, "currencies:all", currencies, l1TTL)
	for i := range currencies {
		cs.SetCurrency(&currencies[i])
	}
}

// ---- Exchange Rate (L1 + Redis) ---------------------------------------------

func (cs *CacheService) GetExchangeRate(fromCurrencyID, toCurrencyID int32) (*domain.ExchangeRate, bool) {
	key := fmt.Sprintf("exchange_rate:%d:%d", fromCurrencyID, toCurrencyID)
	ctx := context.Background()
	if v, ok := unmarshalGet[domain.ExchangeRate](ctx, cs.l1, key); ok {
		return v, true
	}
	v, ok := unmarshalGet[domain.ExchangeRate](ctx, cs.store, key)
	if ok {
		marshalSet(ctx, cs.l1, key, v, l1TTL)
	}
	return v, ok
}

func (cs *CacheService) GetExchangeRateById(id int64) (*domain.ExchangeRate, bool) {
	key := fmt.Sprintf("exchange_rate:id:%d", id)
	ctx := context.Background()
	if v, ok := unmarshalGet[domain.ExchangeRate](ctx, cs.l1, key); ok {
		return v, true
	}
	v, ok := unmarshalGet[domain.ExchangeRate](ctx, cs.store, key)
	if ok {
		marshalSet(ctx, cs.l1, key, v, l1TTL)
	}
	return v, ok
}

func (cs *CacheService) SetExchangeRate(rate *domain.ExchangeRate) {
	key := fmt.Sprintf("exchange_rate:%d:%d", rate.FromCurrencyID, rate.ToCurrencyID)
	ctx := context.Background()
	marshalSet(ctx, cs.store, key, rate, NoExpiration)
	marshalSet(ctx, cs.l1, key, rate, l1TTL)
}

func (cs *CacheService) UpdateExchangeRateCache(rate *domain.ExchangeRate) {
	ctx := context.Background()
	keyPair := fmt.Sprintf("exchange_rate:%d:%d", rate.FromCurrencyID, rate.ToCurrencyID)
	keyID := fmt.Sprintf("exchange_rate:id:%d", rate.ID)
	marshalSet(ctx, cs.store, keyPair, rate, NoExpiration)
	marshalSet(ctx, cs.store, keyID, rate, NoExpiration)
	marshalSet(ctx, cs.l1, keyPair, rate, l1TTL)
	marshalSet(ctx, cs.l1, keyID, rate, l1TTL)
	_ = cs.store.Delete(ctx, "exchange_rates:all")
	_ = cs.store.Delete(ctx, "exchange_rates:active")
	_ = cs.l1.Delete(ctx, "exchange_rates:all")
	_ = cs.l1.Delete(ctx, "exchange_rates:active")
}

func (cs *CacheService) UpdateExchangeRateCacheOnly(rate *domain.ExchangeRate) {
	ctx := context.Background()
	keyPair := fmt.Sprintf("exchange_rate:%d:%d", rate.FromCurrencyID, rate.ToCurrencyID)
	keyID := fmt.Sprintf("exchange_rate:id:%d", rate.ID)
	marshalSet(ctx, cs.store, keyPair, rate, NoExpiration)
	marshalSet(ctx, cs.store, keyID, rate, NoExpiration)
	marshalSet(ctx, cs.l1, keyPair, rate, l1TTL)
	marshalSet(ctx, cs.l1, keyID, rate, l1TTL)
}

func (cs *CacheService) InvalidateExchangeRateCache(fromCurrencyID, toCurrencyID int32, id int64) {
	ctx := context.Background()
	_ = cs.store.Delete(ctx, fmt.Sprintf("exchange_rate:%d:%d", fromCurrencyID, toCurrencyID))
	_ = cs.store.Delete(ctx, fmt.Sprintf("exchange_rate:id:%d", id))
	_ = cs.store.Delete(ctx, "exchange_rates:all")
	_ = cs.store.Delete(ctx, "exchange_rates:active")
	_ = cs.l1.Delete(ctx, fmt.Sprintf("exchange_rate:%d:%d", fromCurrencyID, toCurrencyID))
	_ = cs.l1.Delete(ctx, fmt.Sprintf("exchange_rate:id:%d", id))
	_ = cs.l1.Delete(ctx, "exchange_rates:all")
	_ = cs.l1.Delete(ctx, "exchange_rates:active")
}

func (cs *CacheService) InvalidateExchangeRateListCaches() {
	ctx := context.Background()
	_ = cs.store.Delete(ctx, "exchange_rates:all")
	_ = cs.store.Delete(ctx, "exchange_rates:active")
	_ = cs.l1.Delete(ctx, "exchange_rates:all")
	_ = cs.l1.Delete(ctx, "exchange_rates:active")
}

func (cs *CacheService) GetAllExchangeRates() ([]domain.ExchangeRateWithCurrencies, bool) {
	ctx := context.Background()
	if v, ok := unmarshalSliceGet[domain.ExchangeRateWithCurrencies](ctx, cs.l1, "exchange_rates:all"); ok {
		return v, true
	}
	v, ok := unmarshalSliceGet[domain.ExchangeRateWithCurrencies](ctx, cs.store, "exchange_rates:all")
	if ok {
		b, _ := json.Marshal(v)
		_ = cs.l1.Set(ctx, "exchange_rates:all", b, l1TTL)
	}
	return v, ok
}

func (cs *CacheService) GetActiveExchangeRates() ([]domain.ExchangeRateWithCurrencies, bool) {
	ctx := context.Background()
	if v, ok := unmarshalSliceGet[domain.ExchangeRateWithCurrencies](ctx, cs.l1, "exchange_rates:active"); ok {
		return v, true
	}
	v, ok := unmarshalSliceGet[domain.ExchangeRateWithCurrencies](ctx, cs.store, "exchange_rates:active")
	if ok {
		b, _ := json.Marshal(v)
		_ = cs.l1.Set(ctx, "exchange_rates:active", b, l1TTL)
	}
	return v, ok
}

func (cs *CacheService) SetAllExchangeRates(rates []domain.ExchangeRateWithCurrencies) {
	ctx := context.Background()
	marshalSet(ctx, cs.store, "exchange_rates:all", rates, NoExpiration)
	marshalSet(ctx, cs.l1, "exchange_rates:all", rates, l1TTL)
}

func (cs *CacheService) SetActiveExchangeRates(rates []domain.ExchangeRateWithCurrencies) {
	ctx := context.Background()
	marshalSet(ctx, cs.store, "exchange_rates:active", rates, NoExpiration)
	marshalSet(ctx, cs.l1, "exchange_rates:active", rates, l1TTL)
}

// ---- Wallet -----------------------------------------------------------------

func (cs *CacheService) GetWallet(userID int64, currencyID int32) (*domain.Wallet, bool) {
	key := fmt.Sprintf("wallet:%d:%d", userID, currencyID)
	return unmarshalGet[domain.Wallet](context.Background(), cs.store, key)
}

func (cs *CacheService) GetUserWallets(userID int64) ([]domain.WalletWithCurrency, bool) {
	key := fmt.Sprintf("wallets:user:%d", userID)
	return unmarshalSliceGet[domain.WalletWithCurrency](context.Background(), cs.store, key)
}

func (cs *CacheService) SetWallet(wallet *domain.Wallet) {
	ctx := context.Background()
	marshalSet(ctx, cs.store, fmt.Sprintf("wallet:%d:%d", wallet.UserID, wallet.CurrencyID), wallet, NoExpiration)
	_ = cs.store.Delete(ctx, fmt.Sprintf("wallets:user:%d", wallet.UserID))
}

func (cs *CacheService) SetUserWallets(userID int64, wallets []domain.WalletWithCurrency) {
	marshalSet(context.Background(), cs.store, fmt.Sprintf("wallets:user:%d", userID), wallets, NoExpiration)
}

// ---- Session ----------------------------------------------------------------

func (cs *CacheService) GetSession(token string) (*domain.UserSession, bool) {
	key := fmt.Sprintf("session:%s", token)
	return unmarshalGet[domain.UserSession](context.Background(), cs.store, key)
}

func (cs *CacheService) SetSession(session *domain.UserSession, ttl time.Duration) {
	marshalSet(context.Background(), cs.store, fmt.Sprintf("session:%s", session.RefreshToken), session, ttl)
}

func (cs *CacheService) DeleteSession(token string) {
	_ = cs.store.Delete(context.Background(), fmt.Sprintf("session:%s", token))
}

// ---- OTC Config -------------------------------------------------------------

func (cs *CacheService) GetOTCConfigs() ([]domain.OTCConfig, bool) {
	return unmarshalSliceGet[domain.OTCConfig](context.Background(), cs.store, "otc_config:all")
}

func (cs *CacheService) GetOTCConfigByID(id int64) (*domain.OTCConfig, bool) {
	return unmarshalGet[domain.OTCConfig](context.Background(), cs.store, fmt.Sprintf("otc_config:id:%d", id))
}

func (cs *CacheService) GetOTCConfigByPair(fromCurrencyID, toCurrencyID int64) (*domain.OTCConfig, bool) {
	key := fmt.Sprintf("otc_config:pair:%d:%d", fromCurrencyID, toCurrencyID)
	return unmarshalGet[domain.OTCConfig](context.Background(), cs.store, key)
}

func (cs *CacheService) SetOTCConfigs(configs []domain.OTCConfig) {
	ctx := context.Background()
	marshalSet(ctx, cs.store, "otc_config:all", configs, NoExpiration)
	for i := range configs {
		cs.setOTCConfigKeys(ctx, &configs[i])
	}
}

func (cs *CacheService) SetOTCConfig(config *domain.OTCConfig) {
	ctx := context.Background()
	cs.setOTCConfigKeys(ctx, config)
	_ = cs.store.Delete(ctx, "otc_config:all")
}

func (cs *CacheService) InvalidateOTCConfig(id, fromCurrencyID, toCurrencyID int64) {
	ctx := context.Background()
	_ = cs.store.Delete(ctx, fmt.Sprintf("otc_config:id:%d", id))
	_ = cs.store.Delete(ctx, fmt.Sprintf("otc_config:pair:%d:%d", fromCurrencyID, toCurrencyID))
	_ = cs.store.Delete(ctx, "otc_config:all")
}

func (cs *CacheService) setOTCConfigKeys(ctx context.Context, config *domain.OTCConfig) {
	marshalSet(ctx, cs.store, fmt.Sprintf("otc_config:id:%d", config.ID), config, NoExpiration)
	marshalSet(ctx, cs.store, fmt.Sprintf("otc_config:pair:%d:%d", config.FromCurrencyID, config.ToCurrencyID), config, NoExpiration)
}
