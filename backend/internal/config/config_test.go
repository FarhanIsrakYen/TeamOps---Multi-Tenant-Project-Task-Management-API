package config

import (
	"github.com/stretchr/testify/require"
	"testing"
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
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "teamops-test-api", cfg.JWTIssuer)
	require.Equal(t, "teamops-test-client", cfg.JWTAudience)
	require.Equal(t, 11, cfg.PasswordHashCost)
	require.Equal(t, 42, cfg.RateLimitPerMin)
	require.Equal(t, []string{"https://one.example", "https://two.example"}, cfg.CORSOrigins)
}

func TestLoadRejectsUnsafeBcryptCost(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-long-enough-test-secret-value")
	t.Setenv("BCRYPT_COST", "9")
	_, err := Load()
	require.EqualError(t, err, "BCRYPT_COST must be between 10 and 14")
}
