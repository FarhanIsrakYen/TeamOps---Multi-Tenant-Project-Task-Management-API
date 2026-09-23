package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLoggingWritesStructuredRequestContextWithoutCredentials(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	log := zerolog.New(&output).With().Timestamp().Logger()
	userID := uuid.New()
	router := gin.New()
	router.Use(RequestID(), Logging(log))
	router.GET("/projects/:projectId", func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Status(http.StatusAccepted)
	})
	request := httptest.NewRequest(http.MethodGet, "/projects/project-1?access_token=do-not-log", nil)
	request.Header.Set("Authorization", "Bearer do-not-log")
	request.Header.Set("X-Request-ID", "request-123")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &entry))
	require.Equal(t, "info", entry["level"])
	require.NotEmpty(t, entry["time"])
	require.Equal(t, "request-123", entry["request_id"])
	require.Equal(t, http.MethodGet, entry["method"])
	require.Equal(t, "/projects/project-1", entry["path"])
	require.Equal(t, float64(http.StatusAccepted), entry["status"])
	require.Equal(t, userID.String(), entry["user_id"])
	require.Contains(t, entry, "duration")
	require.NotContains(t, output.String(), "do-not-log")
}

func TestLoggingRecordsInternalCauseWithoutExposingItToClient(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	log := zerolog.New(&output)
	router := gin.New()
	router.Use(RequestID(), Logging(log))
	router.GET("/failure", func(c *gin.Context) {
		response.Error(c, errors.New("database connection details"))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/failure", nil))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "database connection details")
	require.Contains(t, output.String(), "database connection details")
}

type fakeCounter struct {
	values map[string]int64
	err    error
}

func (f *fakeCounter) Increment(_ context.Context, key string, _ time.Duration) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.values[key]++
	return f.values[key], nil
}

func TestRateLimitUsesClientIPAndReturnsConsistentError(t *testing.T) {
	t.Parallel()
	counter := &fakeCounter{values: make(map[string]int64)}
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(RequestID(), RateLimit(counter, "test", 1, time.Minute))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.Header.Set("X-Forwarded-For", "198.51.100.10")
	router.ServeHTTP(first, firstRequest)
	require.Equal(t, http.StatusNoContent, first.Code)
	require.Equal(t, "0", first.Header().Get("X-RateLimit-Remaining"))

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.Header.Set("X-Forwarded-For", "198.51.100.11")
	router.ServeHTTP(second, secondRequest)
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.NotEmpty(t, second.Header().Get("Retry-After"))
	require.Contains(t, second.Body.String(), `"code":"rate_limited"`)
	require.Contains(t, second.Body.String(), `"requestId"`)
}

func TestRateLimitFailsOpenWhenRedisIsUnavailable(t *testing.T) {
	t.Parallel()
	router := gin.New()
	router.Use(RateLimit(&fakeCounter{err: errors.New("redis unavailable")}, "test", 1, time.Minute))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for range 3 {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}
}

func TestBodyLimitAndSecureHeaders(t *testing.T) {
	t.Parallel()
	router := gin.New()
	router.Use(RequestID(), SecureHeaders(true), BodyLimit(8))
	router.POST("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	require.Contains(t, recorder.Header().Get("Strict-Transport-Security"), "max-age=")
}

func TestCORSAllowListAndConsistentRejection(t *testing.T) {
	t.Parallel()
	router := gin.New()
	router.Use(RequestID(), CORS([]string{"https://app.example"}))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	allowedRequest.Header.Set("Origin", "https://app.example")
	router.ServeHTTP(allowed, allowedRequest)
	require.Equal(t, http.StatusNoContent, allowed.Code)
	require.Equal(t, "https://app.example", allowed.Header().Get("Access-Control-Allow-Origin"))

	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	deniedRequest.Header.Set("Origin", "https://evil.example")
	router.ServeHTTP(denied, deniedRequest)
	require.Equal(t, http.StatusForbidden, denied.Code)
	require.Contains(t, denied.Body.String(), `"code":"cors_origin_forbidden"`)
}
