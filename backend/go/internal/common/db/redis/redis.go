// Package redis provides Redis client utilities.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps a redis client with rate-limit and lock helpers.
type Client struct {
	*redis.Client
}

// Connect creates a new Redis client.
func Connect(ctx context.Context, addr, password string) (*Client, error) {
	c := &Client{Client: redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})}
	if err := c.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: connect: %w", err)
	}
	return c, nil
}

// Lock acquires a distributed lock. Returns true if acquired.
func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.SetNX(ctx, "lock:"+key, "1", ttl).Result()
}

// Unlock releases a distributed lock.
func (c *Client) Unlock(ctx context.Context, key string) error {
	return c.Del(ctx, "lock:"+key).Err()
}

// RateLimit checks if a request is within the rate limit (token bucket).
func (c *Client) RateLimit(ctx context.Context, key string, maxTokens int, window time.Duration) (bool, error) {
	pipe := c.Pipeline()
	incr := pipe.Incr(ctx, "rl:"+key)
	pipe.Expire(ctx, "rl:"+key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return incr.Val() <= int64(maxTokens), nil
}

// Close releases the client.
func (c *Client) Close() error { return c.Client.Close() }
