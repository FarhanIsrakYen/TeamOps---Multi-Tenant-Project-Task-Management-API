package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
