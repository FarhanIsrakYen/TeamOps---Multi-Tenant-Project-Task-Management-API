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
	t.Setenv("MEMBERSHIP_CACHE_TTL", "15s")
	t.Setenv("AUTH_RATE_LIMIT_PER_MIN", "21")
	t.Setenv("LOGIN_RATE_LIMIT_PER_MIN", "7")
	t.Setenv("LOGIN_FAILURE_LIMIT", "4")
	t.Setenv("LOGIN_FAILURE_WINDOW", "10m")
	t.Setenv("REQUEST_BODY_MAX_BYTES", "65536")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	t.Setenv("AUDIT_QUEUE_SIZE", "256")
	t.Setenv("AUDIT_WORKERS", "3")
	t.Setenv("AUDIT_WRITE_TIMEOUT", "2s")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "teamops-test-api", cfg.JWTIssuer)
	require.Equal(t, "teamops-test-client", cfg.JWTAudience)
	require.Equal(t, 11, cfg.PasswordHashCost)
	require.Equal(t, 42, cfg.RateLimitPerMin)
	require.Equal(t, []string{"https://one.example", "https://two.example"}, cfg.CORSOrigins)
	require.Equal(t, 2*time.Minute, cfg.OrganizationCacheTTL)
	require.Equal(t, 3*time.Minute, cfg.ProjectCacheTTL)
	require.Equal(t, 15*time.Second, cfg.MembershipCacheTTL)
	require.Equal(t, 21, cfg.AuthRateLimitPerMin)
	require.Equal(t, 7, cfg.LoginRateLimitPerMin)
	require.Equal(t, 4, cfg.LoginFailureLimit)
	require.Equal(t, 10*time.Minute, cfg.LoginFailureWindow)
	require.Equal(t, int64(65536), cfg.RequestBodyMaxBytes)
	require.Equal(t, []string{"127.0.0.1", "10.0.0.0/8"}, cfg.TrustedProxies)
	require.Equal(t, 256, cfg.AuditQueueSize)
	require.Equal(t, 3, cfg.AuditWorkers)
	require.Equal(t, 2*time.Second, cfg.AuditWriteTimeout)
}

func TestLoadRejectsUnsafeBcryptCost(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("BCRYPT_COST", "9")
	_, err := Load()
	require.EqualError(t, err, "BCRYPT_COST must be between 10 and 14")
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
