package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/teamops/backend/internal/cache"
	"github.com/example/teamops/backend/internal/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpointDoesNotRequireInfrastructure(t *testing.T) {
	t.Parallel()
	a := New(config.Config{Environment: "test", CORSOrigins: []string{"http://localhost"}}, nil, cache.New("127.0.0.1:0", ""), zerolog.Nop())
	t.Cleanup(func() { _ = a.JobPool.Shutdown(context.Background()) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "test-request-id")

	a.Router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "test-request-id", recorder.Header().Get("X-Request-ID"))
	require.JSONEq(t, `{"data":{"status":"ok"}}`, recorder.Body.String())
}

func TestHealthEndpointGeneratesRequestID(t *testing.T) {
	t.Parallel()
	a := New(config.Config{Environment: "test", CORSOrigins: []string{"http://localhost"}}, nil, cache.New("127.0.0.1:0", ""), zerolog.Nop())
	t.Cleanup(func() { _ = a.JobPool.Shutdown(context.Background()) })
	recorder := httptest.NewRecorder()

	a.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotEmpty(t, recorder.Header().Get("X-Request-ID"))
}

func TestOpenAPIAndSwaggerUIEndpoints(t *testing.T) {
	t.Parallel()
	a := New(config.Config{Environment: "test", CORSOrigins: []string{"http://localhost"}}, nil, cache.New("127.0.0.1:0", ""), zerolog.Nop())
	t.Cleanup(func() { _ = a.JobPool.Shutdown(context.Background()) })

	t.Run("raw contract", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		a.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Header().Get("Content-Type"), "application/yaml")
		require.Contains(t, recorder.Body.String(), "openapi: 3.1.0")
	})

	t.Run("canonical docs redirect", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		a.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs", nil))

		require.Equal(t, http.StatusTemporaryRedirect, recorder.Code)
		require.Equal(t, "/docs/", recorder.Header().Get("Location"))
	})

	t.Run("interactive documentation", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		a.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs/", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
		require.Contains(t, recorder.Header().Get("Content-Security-Policy"), "cdn.jsdelivr.net")
		require.Contains(t, recorder.Body.String(), "SwaggerUIBundle")
	})
}
