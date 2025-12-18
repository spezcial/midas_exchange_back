package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type MemoryCache struct {
	cache *gocache.Cache
}

func NewMemoryCache(defaultExpiration, cleanupInterval time.Duration) *MemoryCache {
	return &MemoryCache{
		cache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

func (m *MemoryCache) Get(key string) (interface{}, bool) {
	return m.cache.Get(key)
}

func (m *MemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	m.cache.Set(key, value, ttl)
	return nil
}

func (m *MemoryCache) Delete(key string) error {
	m.cache.Delete(key)
	return nil
}

func (m *MemoryCache) Flush() error {
	m.cache.Flush()
	return nil
}
