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

	// SetNX sets key to value only if the key does not already exist.
	// Returns true if the key was set, false if it already existed.
	// Used for rate limiting and deduplication.
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)

	// GetDel atomically retrieves and deletes a key.
	// Returns the value and true if the key existed, nil and false otherwise.
	// Used for single-use tokens (OTPs, action tokens).
	GetDel(ctx context.Context, key string) ([]byte, bool, error)
}
