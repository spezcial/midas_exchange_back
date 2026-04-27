package cache

import (
	"context"
	"time"
)

// NoExpiration means the key will never expire.
// Use 0 with the Store interface; each implementation translates accordingly.
const NoExpiration = time.Duration(0)

// Store is the low-level key/value interface backed by either an in-memory
// cache or Redis. Values are raw JSON bytes; callers handle (un)marshalling.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Flush(ctx context.Context) error
}
