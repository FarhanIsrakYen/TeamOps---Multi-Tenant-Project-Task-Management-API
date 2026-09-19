package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetricLabelsAreBounded(t *testing.T) {
	require.Equal(t, "SELECT", databaseOperation(" select id from users"))
	require.Equal(t, "WITH", databaseOperation("WITH rows AS (SELECT 1) SELECT * FROM rows"))
	require.Equal(t, "OTHER", databaseOperation("VACUUM users"))
	require.Equal(t, "get", redisOperation("GET"))
	require.Equal(t, "other", redisOperation("user-controlled-command"))
	require.Equal(t, "other", authenticationFailureReason("user-controlled-reason"))
}

func TestMetricsRecordErrorsAndOperationalEvents(t *testing.T) {
	httpBefore := testutil.ToFloat64(httpErrors.WithLabelValues("GET", "/observability-test", "503"))
	authBefore := testutil.ToFloat64(authenticationFailures.WithLabelValues("invalid_credentials"))
	rateBefore := testutil.ToFloat64(rateLimitEvents.WithLabelValues("test", "limited"))
	redisBefore := testutil.ToFloat64(redisOperations.WithLabelValues("get", "error"))

	ObserveHTTPRequest("GET", "/observability-test", 503, time.Millisecond)
	AuthenticationFailure("invalid_credentials")
	RateLimitEvent("test", "limited")
	ObserveRedis("GET", errors.New("unavailable"), time.Millisecond)
	ObserveDatabase("SELECT", errors.New("unavailable"), time.Millisecond)

	require.Equal(t, httpBefore+1, testutil.ToFloat64(httpErrors.WithLabelValues("GET", "/observability-test", "503")))
	require.Equal(t, authBefore+1, testutil.ToFloat64(authenticationFailures.WithLabelValues("invalid_credentials")))
	require.Equal(t, rateBefore+1, testutil.ToFloat64(rateLimitEvents.WithLabelValues("test", "limited")))
	require.Equal(t, redisBefore+1, testutil.ToFloat64(redisOperations.WithLabelValues("get", "error")))
	require.Equal(t, 1, testutil.CollectAndCount(databaseDuration))
}

func TestSetupTracingIsNoOpWhenEndpointIsEmpty(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), TraceConfig{})
	require.NoError(t, err)
	require.NoError(t, shutdown(context.Background()))
}

func TestSetupTracingRejectsInvalidEndpoint(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), TraceConfig{Endpoint: "collector:4317"})
	require.Error(t, err)
	require.Nil(t, shutdown)
}
