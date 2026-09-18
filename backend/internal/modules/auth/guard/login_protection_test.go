package guard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type loginAttemptStore struct {
	values map[string]int64
	err    error
}

func (s *loginAttemptStore) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.values[key]++
	return s.values[key], nil
}
func (s *loginAttemptStore) Delete(_ context.Context, keys ...string) error {
	for _, key := range keys {
		delete(s.values, key)
	}
	return s.err
}

func TestLoginProtectorBlocksAndSuccessfulLoginCanReset(t *testing.T) {
	t.Parallel()
	store := &loginAttemptStore{values: make(map[string]int64)}
	protector := NewLoginProtector(store, 2, time.Minute)
	ctx := context.Background()

	require.NoError(t, protector.Check(ctx, "192.0.2.1", "Person@Example.com"))
	require.NoError(t, protector.Check(ctx, "192.0.2.1", "person@example.com"))
	require.Error(t, protector.Check(ctx, "192.0.2.1", "person@example.com"))

	protector.Reset(ctx, "192.0.2.1", "person@example.com")
	require.NoError(t, protector.Check(ctx, "192.0.2.1", "person@example.com"))
}

func TestLoginProtectorFailsOpenWithoutRedis(t *testing.T) {
	t.Parallel()
	protector := NewLoginProtector(&loginAttemptStore{err: errors.New("redis unavailable")}, 1, time.Minute)
	require.NoError(t, protector.Check(context.Background(), "192.0.2.1", "person@example.com"))
}
