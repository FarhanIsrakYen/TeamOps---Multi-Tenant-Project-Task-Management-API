package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyConventions(t *testing.T) {
	t.Parallel()
	require.Equal(t, "teamops:v1:organization:org-id", OrganizationKey("org-id"))
	require.Equal(t, "teamops:v1:project:project-id", ProjectKey("project-id"))
	require.Equal(t, "teamops:v1:membership:org-id:user-id", MembershipKey("org-id", "user-id"))
}

func TestSecurityKeysHashClientIdentifiers(t *testing.T) {
	t.Parallel()
	rateKey := RateLimitKey("login", "person@example.com", 42)
	loginKey := LoginAttemptKey("account", "person@example.com")
	require.NotContains(t, rateKey, "person@example.com")
	require.NotContains(t, loginKey, "person@example.com")
	require.Contains(t, rateKey, "teamops:v1:rate:login:")
	require.Contains(t, loginKey, "teamops:v1:login-attempt:account:")
}
