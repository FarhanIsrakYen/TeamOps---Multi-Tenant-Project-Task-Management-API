package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func testConfig() Config {
	return Config{Workers: 1, QueueSize: 8, MaxRetries: 3, InitialBackoff: time.Millisecond, MaxBackoff: 4 * time.Millisecond, JobTimeout: time.Second}
}

func TestPoolSuccessfulJob(t *testing.T) {
	pool := NewPool(testConfig(), zerolog.Nop())
	var calls atomic.Int32
	require.NoError(t, pool.Submit(JobFunc{JobName: "success", Handler: func(context.Context) error {
		calls.Add(1)
		return nil
	}}))
	require.NoError(t, pool.Shutdown(context.Background()))
	require.Equal(t, int32(1), calls.Load())
}

func TestPoolRetriesTransientFailure(t *testing.T) {
	pool := NewPool(testConfig(), zerolog.Nop())
	var calls atomic.Int32
	require.NoError(t, pool.Submit(JobFunc{JobName: "retry", Handler: func(context.Context) error {
		if calls.Add(1) < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}}))
	require.NoError(t, pool.Shutdown(context.Background()))
	require.Equal(t, int32(3), calls.Load())
}

func TestPoolDoesNotRetryPermanentFailure(t *testing.T) {
	pool := NewPool(testConfig(), zerolog.Nop())
	var calls atomic.Int32
	require.NoError(t, pool.Submit(JobFunc{JobName: "permanent", Handler: func(context.Context) error {
		calls.Add(1)
		return Permanent(errors.New("invalid input"))
	}}))
	require.NoError(t, pool.Shutdown(context.Background()))
	require.Equal(t, int32(1), calls.Load())
}

func TestPoolCancelsActiveJobWhenShutdownDeadlineExpires(t *testing.T) {
	pool := NewPool(testConfig(), zerolog.Nop())
	started := make(chan struct{})
	canceled := make(chan struct{})
	require.NoError(t, pool.Submit(JobFunc{JobName: "cancellation", Handler: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	}}))
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, pool.Shutdown(ctx), context.DeadlineExceeded)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("active job did not observe cancellation")
	}
	require.NoError(t, pool.Shutdown(context.Background()))
}

func TestPoolShutdownDrainsQueueAndRejectsNewJobs(t *testing.T) {
	pool := NewPool(testConfig(), zerolog.Nop())
	var calls atomic.Int32
	for range 5 {
		require.NoError(t, pool.Submit(JobFunc{JobName: "drain", Handler: func(context.Context) error {
			calls.Add(1)
			return nil
		}}))
	}
	require.NoError(t, pool.Shutdown(context.Background()))
	require.Equal(t, int32(5), calls.Load())
	require.ErrorIs(t, pool.Submit(JobFunc{JobName: "late", Handler: func(context.Context) error { return nil }}), ErrClosed)
}

func TestPoolReportsQueueSaturation(t *testing.T) {
	config := testConfig()
	config.QueueSize = 1
	pool := NewPool(config, zerolog.Nop())
	started := make(chan struct{})
	release := make(chan struct{})
	require.NoError(t, pool.Submit(JobFunc{JobName: "blocking", Handler: func(context.Context) error {
		close(started)
		<-release
		return nil
	}}))
	<-started
	require.NoError(t, pool.Submit(JobFunc{JobName: "queued", Handler: func(context.Context) error { return nil }}))
	require.Equal(t, 1, pool.QueueDepth())
	require.ErrorIs(t, pool.Submit(JobFunc{JobName: "rejected", Handler: func(context.Context) error { return nil }}), ErrQueueFull)
	close(release)
	require.NoError(t, pool.Shutdown(context.Background()))
}
