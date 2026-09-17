package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache owns the Redis client. It is constructed in main and passed to its
// consumers; no package-level connection is used.
type Cache struct{ client *redis.Client }

func New(addr, password string) *Cache {
	return &Cache{client: redis.NewClient(&redis.Options{Addr: addr, Password: password})}
}

func (c *Cache) Ping(ctx context.Context) error { return c.client.Ping(ctx).Err() }
func (c *Cache) Close() error                   { return c.client.Close() }
func (c *Cache) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get: %w", err)
	}
	if err = json.Unmarshal(raw, dst); err != nil {
		return false, fmt.Errorf("cache decode: %w", err)
	}
	return true, nil
}
func (c *Cache) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache encode: %w", err)
	}
	return c.client.Set(ctx, key, raw, ttl).Err()
}
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}
func (c *Cache) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = c.client.Expire(ctx, key, ttl).Err()
	}
	return n, nil
}
