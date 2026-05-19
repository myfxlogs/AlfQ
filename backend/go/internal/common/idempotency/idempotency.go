// Package idempotency provides Redis-based idempotency key tools.
package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrDuplicate indicates the idempotency key has already been processed.
var ErrDuplicate = errors.New("idempotency: duplicate request")

// Store manages idempotency keys backed by Redis.
type Store struct {
	rdb redis.UniversalClient
	ttl time.Duration
}

// New creates an idempotency store backed by the given Redis client.
// ttl is the expiry for both the key marker and cached result. Defaults to 24h.
func New(rdb redis.UniversalClient, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{rdb: rdb, ttl: ttl}
}

// CheckAndSet atomically checks and sets an idempotency key.
// Returns (true, nil) if the key was newly set (request should proceed).
// Returns (false, ErrDuplicate) if the key already exists.
func (s *Store) CheckAndSet(ctx context.Context, key string) (bool, error) {
	ok, err := s.rdb.SetNX(ctx, "idem:"+key, "pending", s.ttl).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrDuplicate
	}
	return true, nil
}

// GetResult retrieves a cached result for an idempotency key.
// Returns ("", nil) if no result is cached yet.
func (s *Store) GetResult(ctx context.Context, key string) (string, error) {
	val, err := s.rdb.Get(ctx, "idem:"+key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return val, err
}

// SetResult stores a result for an idempotency key.
// This overwrites the initial "pending" marker set by CheckAndSet.
func (s *Store) SetResult(ctx context.Context, key, result string) error {
	return s.rdb.Set(ctx, "idem:"+key, result, s.ttl).Err()
}
