//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRedisCacheAndRateCounterIntegration(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("TEST_REDIS_ADDR is not set")
	}
	client := cache.New(address, "")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx))

	t.Run("JSON cache round trip and invalidation", func(t *testing.T) {
		key := cache.OrganizationKey(uuid.NewString())
		want := struct {
			Name    string `json:"name"`
			Version int    `json:"version"`
		}{Name: "Cached organization", Version: 3}
		require.NoError(t, client.SetJSON(ctx, key, want, time.Minute))
		var got struct {
			Name    string `json:"name"`
			Version int    `json:"version"`
		}
		hit, err := client.GetJSON(ctx, key, &got)
		require.NoError(t, err)
		require.True(t, hit)
		require.Equal(t, want, got)
		require.NoError(t, client.Delete(ctx, key))
		hit, err = client.GetJSON(ctx, key, &got)
		require.NoError(t, err)
		require.False(t, hit)
	})

	t.Run("rate counter increments atomically", func(t *testing.T) {
		key := cache.RateLimitKey("integration", uuid.NewString(), time.Now().Unix())
		const increments = 20
		type result struct {
			value int64
			err   error
		}
		results := make(chan result, increments)
		for range increments {
			go func() {
				value, err := client.Increment(ctx, key, time.Minute)
				results <- result{value: value, err: err}
			}()
		}
		seen := make(map[int64]bool, increments)
		for range increments {
			result := <-results
			require.NoError(t, result.err)
			seen[result.value] = true
		}
		for expected := int64(1); expected <= increments; expected++ {
			require.True(t, seen[expected], "missing atomic increment result %d", expected)
		}
		require.NoError(t, client.Delete(ctx, key))
	})
}
