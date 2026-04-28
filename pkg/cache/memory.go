package cache

import (
	"context"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// MemoryCache implements Store using an in-process go-cache.
// Used as the L1 cache for hot read-only data (exchange rates, currencies).
type MemoryCache struct {
	cache *gocache.Cache
	mu    sync.Mutex // guards SetNX and GetDel atomicity
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

// SetNX sets the key only if it does not already exist.
// Returns true if the key was newly set, false if it already existed.
func (m *MemoryCache) SetNX(_ context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, found := m.cache.Get(key); found {
		return false, nil
	}

	if ttl == NoExpiration {
		m.cache.Set(key, value, gocache.NoExpiration)
	} else {
		m.cache.Set(key, value, ttl)
	}
	return true, nil
}

// GetDel atomically retrieves and deletes a key.
// Returns the value and true if the key existed, nil and false otherwise.
func (m *MemoryCache) GetDel(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	val, found := m.cache.Get(key)
	if !found {
		return nil, false, nil
	}
	m.cache.Delete(key)

	b, ok := val.([]byte)
	if !ok {
		return nil, false, nil
	}
	return b, true, nil
}
