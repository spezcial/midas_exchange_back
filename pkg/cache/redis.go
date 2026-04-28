package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Store backed by a Redis instance.
type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

// Set stores value bytes. ttl=0 (NoExpiration) means the key never expires.
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) Flush(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// SetNX sets the key only if it does not already exist (SET NX EX).
// Returns true if the key was newly set, false if it already existed.
func (r *RedisCache) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	cmd := r.client.SetArgs(ctx, key, value, redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	})
	if err := cmd.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil // key already exists
		}
		return false, err
	}
	return true, nil
}

// GetDel atomically retrieves and deletes the key (Redis GETDEL command).
// Returns the value and true if the key existed, nil and false otherwise.
func (r *RedisCache) GetDel(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.client.GetDel(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}
