package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "teamops:v1"

type Store interface {
	GetJSON(context.Context, string, any) (bool, error)
	SetJSON(context.Context, string, any, time.Duration) error
	Delete(context.Context, ...string) error
}

func OrganizationKey(organizationID string) string {
	return keyPrefix + ":organization:" + organizationID
}

func ProjectKey(projectID string) string {
	return keyPrefix + ":project:" + projectID
}

func MembershipKey(organizationID, userID string) string {
	return keyPrefix + ":membership:" + organizationID + ":" + userID
}

func RateLimitKey(scope, identity string, bucket int64) string {
	digest := sha256.Sum256([]byte(identity))
	return keyPrefix + ":rate:" + scope + ":" + fmt.Sprintf("%x", digest[:12]) + ":" + strconv.FormatInt(bucket, 10)
}

func LoginAttemptKey(kind, identity string) string {
	digest := sha256.Sum256([]byte(identity))
	return keyPrefix + ":login-attempt:" + kind + ":" + fmt.Sprintf("%x", digest[:12])
}

// Cache owns the Redis client. It is constructed in main and passed to its
// consumers; no package-level connection is used.
type Cache struct{ client *redis.Client }

func New(addr, password string) *Cache {
	return &Cache{client: redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
		MaxRetries:   1,
	})}
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
