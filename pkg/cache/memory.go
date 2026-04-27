package cache

import (
	"context"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// MemoryCache implements Store using an in-process go-cache.
// Used as the L1 cache for hot read-only data (exchange rates, currencies).
type MemoryCache struct {
	cache *gocache.Cache
}

func NewMemoryCache(defaultExpiration, cleanupInterval time.Duration) *MemoryCache {
	return &MemoryCache{
		cache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

func (m *MemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	val, found := m.cache.Get(key)
	if !found {
		return nil, false, nil
	}
	b, ok := val.([]byte)
	if !ok {
		return nil, false, nil
	}
	return b, true, nil
}

// Set stores value bytes. ttl=0 (NoExpiration) means no TTL.
func (m *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == NoExpiration {
		m.cache.Set(key, value, gocache.NoExpiration)
	} else {
		m.cache.Set(key, value, ttl)
	}
	return nil
}

func (m *MemoryCache) Delete(_ context.Context, key string) error {
	m.cache.Delete(key)
	return nil
}

func (m *MemoryCache) Flush(_ context.Context) error {
	m.cache.Flush()
	return nil
}
