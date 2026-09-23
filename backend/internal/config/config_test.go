package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRequiresStrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	_, err := Load()
	require.EqualError(t, err, "JWT_SECRET must be at least 32 characters")
}
func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("JWT_ISSUER", "teamops-test-api")
	t.Setenv("JWT_AUDIENCE", "teamops-test-client")
	t.Setenv("BCRYPT_COST", "11")
	t.Setenv("RATE_LIMIT_PER_MIN", "42")
	t.Setenv("CORS_ORIGINS", "https://one.example, https://two.example")
	t.Setenv("ORGANIZATION_CACHE_TTL", "2m")
	t.Setenv("PROJECT_CACHE_TTL", "3m")
	t.Setenv("AUTH_RATE_LIMIT_PER_MIN", "21")
	t.Setenv("LOGIN_RATE_LIMIT_PER_MIN", "7")
	t.Setenv("LOGIN_FAILURE_LIMIT", "4")
	t.Setenv("LOGIN_FAILURE_WINDOW", "10m")
	t.Setenv("REQUEST_BODY_MAX_BYTES", "65536")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	t.Setenv("JOB_QUEUE_SIZE", "256")
	t.Setenv("JOB_WORKERS", "3")
	t.Setenv("JOB_MAX_RETRIES", "4")
	t.Setenv("JOB_TIMEOUT", "2s")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_SERVICE_NAME", "teamops-test")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "0.25")
	t.Setenv("DATABASE_MAX_CONNS", "30")
	t.Setenv("DATABASE_MIN_CONNS", "4")
	t.Setenv("DATABASE_MAX_CONN_LIFETIME", "45m")
	t.Setenv("DATABASE_MAX_CONN_IDLE_TIME", "10m")
	t.Setenv("DATABASE_HEALTH_CHECK_PERIOD", "30s")
	t.Setenv("DATABASE_STATEMENT_TIMEOUT", "20s")
	t.Setenv("DATABASE_LOCK_TIMEOUT", "3s")
	t.Setenv("DATABASE_IDLE_IN_TRANSACTION_TIMEOUT", "25s")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "teamops-test-api", cfg.JWTIssuer)
	require.Equal(t, "teamops-test-client", cfg.JWTAudience)
	require.Equal(t, 11, cfg.PasswordHashCost)
	require.Equal(t, 42, cfg.RateLimitPerMin)
	require.Equal(t, []string{"https://one.example", "https://two.example"}, cfg.CORSOrigins)
	require.Equal(t, 2*time.Minute, cfg.OrganizationCacheTTL)
	require.Equal(t, 3*time.Minute, cfg.ProjectCacheTTL)
	require.Equal(t, 21, cfg.AuthRateLimitPerMin)
	require.Equal(t, 7, cfg.LoginRateLimitPerMin)
	require.Equal(t, 4, cfg.LoginFailureLimit)
	require.Equal(t, 10*time.Minute, cfg.LoginFailureWindow)
	require.Equal(t, int64(65536), cfg.RequestBodyMaxBytes)
	require.Equal(t, []string{"127.0.0.1", "10.0.0.0/8"}, cfg.TrustedProxies)
	require.Equal(t, 256, cfg.JobQueueSize)
	require.Equal(t, 3, cfg.JobWorkers)
	require.Equal(t, 4, cfg.JobMaxRetries)
	require.Equal(t, 2*time.Second, cfg.JobTimeout)
	require.Equal(t, "http://collector:4317", cfg.OTelEndpoint)
	require.True(t, cfg.OTelInsecure)
	require.Equal(t, "teamops-test", cfg.OTelServiceName)
	require.Equal(t, 0.25, cfg.OTelSampleRatio)
	require.Equal(t, int32(30), cfg.DatabaseMaxConns)
	require.Equal(t, int32(4), cfg.DatabaseMinConns)
	require.Equal(t, 45*time.Minute, cfg.DatabaseMaxConnLifetime)
	require.Equal(t, 10*time.Minute, cfg.DatabaseMaxConnIdleTime)
	require.Equal(t, 30*time.Second, cfg.DatabaseHealthCheckPeriod)
	require.Equal(t, 20*time.Second, cfg.DatabaseStatementTimeout)
	require.Equal(t, 3*time.Second, cfg.DatabaseLockTimeout)
	require.Equal(t, 25*time.Second, cfg.DatabaseIdleInTxTimeout)
}

func TestLoadRejectsInvalidTokenAndPoolSettings(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("ACCESS_TOKEN_TTL", "2h")
	t.Setenv("REFRESH_TOKEN_TTL", "1h")
	_, err := Load()
	require.EqualError(t, err, "token TTLs and shutdown timeout are invalid")

	t.Setenv("ACCESS_TOKEN_TTL", "15m")
	t.Setenv("REFRESH_TOKEN_TTL", "168h")
	t.Setenv("DATABASE_MAX_CONNS", "2")
	t.Setenv("DATABASE_MIN_CONNS", "3")
	_, err = Load()
	require.EqualError(t, err, "database pool settings are invalid")
}

func TestLoadRejectsUnsafeBcryptCost(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("BCRYPT_COST", "9")
	_, err := Load()
	require.EqualError(t, err, "BCRYPT_COST must be between 10 and 14")
}

func TestLoadRejectsMalformedTypedConfiguration(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("DATABASE_MAX_CONNS", "many")
	_, err := Load()
	require.ErrorContains(t, err, "DATABASE_MAX_CONNS must be a valid integer")
}

func TestLoadRejectsUnsafeCORSAndProxyConfiguration(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("CORS_ORIGINS", "*")
	_, err := Load()
	require.EqualError(t, err, "CORS_ORIGINS must not contain a wildcard")

	t.Setenv("CORS_ORIGINS", "https://app.example")
	t.Setenv("TRUSTED_PROXIES", "not-a-proxy")
	_, err = Load()
	require.EqualError(t, err, `TRUSTED_PROXIES contains invalid IP or CIDR "not-a-proxy"`)
}
