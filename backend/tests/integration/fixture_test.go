//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/app"
	"github.com/example/teamops/backend/internal/cache"
	"github.com/example/teamops/backend/internal/config"
	"github.com/example/teamops/backend/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type httpFixture struct {
	router *gin.Engine
	db     *pgxpool.Pool
	cache  *cache.Cache
	app    *app.App
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  json.RawMessage `json:"meta"`
	Error *struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

func resetPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url, 10)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`, pgx.QueryExecModeSimpleProtocol)
	require.NoError(t, err)
	migrationPaths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	require.NoError(t, err)
	sort.Strings(migrationPaths)
	for _, migrationPath := range migrationPaths {
		migration, readErr := os.ReadFile(migrationPath)
		require.NoError(t, readErr)
		_, err = pool.Exec(ctx, string(migration), pgx.QueryExecModeSimpleProtocol)
		require.NoError(t, err, migrationPath)
	}
	return pool
}

func newHTTPFixture(t *testing.T) *httpFixture {
	t.Helper()
	pool := resetPostgres(t)
	redisAddress := os.Getenv("TEST_REDIS_ADDR")
	if redisAddress == "" {
		redisAddress = "127.0.0.1:6379"
	}
	cacheClient := cache.New(redisAddress, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, cacheClient.Ping(ctx), "integration tests require Redis; set TEST_REDIS_ADDR")
	t.Cleanup(func() { require.NoError(t, cacheClient.Close()) })

	cfg := config.Config{
		Environment: "test", CORSOrigins: []string{"https://teamops.test"}, RequestBodyMaxBytes: 1 << 20,
		JWTSecret: "integration-test-secret-with-at-least-32-bytes", JWTIssuer: "teamops-integration", JWTAudience: "teamops-tests",
		AccessTokenTTL: 5 * time.Minute, RefreshTokenTTL: time.Hour, PasswordHashCost: 10,
		RateLimitPerMin: 10_000, AuthRateLimitPerMin: 10_000, LoginRateLimitPerMin: 10_000,
		LoginFailureLimit: 100, LoginFailureWindow: time.Minute,
		OrganizationCacheTTL: time.Minute, ProjectCacheTTL: time.Minute, MembershipCacheTTL: time.Minute,
		JobWorkers: 2, JobQueueSize: 128, JobMaxRetries: 1, JobInitialBackoff: time.Millisecond, JobMaxBackoff: 2 * time.Millisecond, JobTimeout: time.Second,
		StaleTaskAfter: 72 * time.Hour, StaleTaskScanInterval: time.Hour, ProjectStatisticsRefreshInterval: time.Hour, SessionCleanupInterval: time.Hour,
	}
	a := app.New(cfg, pool, cacheClient, zerolog.Nop())
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		require.NoError(t, a.JobPool.Shutdown(shutdownCtx))
	})
	return &httpFixture{router: a.Router, db: pool, cache: cacheClient, app: a}
}

func (f *httpFixture) request(t *testing.T, method, path, accessToken string, body any) (*httptest.ResponseRecorder, envelope) {
	t.Helper()
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("X-Request-ID", "integration-request")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	var decoded envelope
	if recorder.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &decoded), recorder.Body.String())
	}
	return recorder, decoded
}

func decodeData[T any](t *testing.T, response envelope) T {
	t.Helper()
	var value T
	require.NoError(t, json.Unmarshal(response.Data, &value))
	return value
}
