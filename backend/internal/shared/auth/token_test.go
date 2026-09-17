package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	testSecret   = "a-secret-that-is-definitely-at-least-32-bytes"
	testIssuer   = "teamops-api"
	testAudience = "teamops-web"
)

func TestTokenManagerAcceptsValidToken(t *testing.T) {
	t.Parallel()
	manager := NewTokenManager(testSecret, testIssuer, testAudience, time.Minute)
	userID := uuid.New()
	raw, expiresAt, err := manager.AccessToken(userID, "engineer@example.com")
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(time.Minute), expiresAt, 2*time.Second)

	claims, err := manager.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, userID.String(), claims.Subject)
	require.Equal(t, "engineer@example.com", claims.Email)
	require.Equal(t, testIssuer, claims.Issuer)
	require.Contains(t, claims.Audience, testAudience)
	require.NotEmpty(t, claims.ID)
	require.NotNil(t, claims.IssuedAt)
	require.NotNil(t, claims.ExpiresAt)
}

func TestTokenManagerRejectsInvalidTokens(t *testing.T) {
	t.Parallel()
	manager := NewTokenManager(testSecret, testIssuer, testAudience, time.Minute)
	userID := uuid.New()
	now := time.Now().UTC()
	validClaims := func() Claims {
		return Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    testIssuer,
				Subject:   userID.String(),
				Audience:  jwt.ClaimStrings{testAudience},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
				IssuedAt:  jwt.NewNumericDate(now),
				ID:        uuid.NewString(),
			},
			UserID: userID,
			Email:  "engineer@example.com",
		}
	}
	sign := func(secret string, claims Claims) string {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		require.NoError(t, err)
		return raw
	}

	tests := []struct {
		name string
		raw  func() string
	}{
		{name: "expired", raw: func() string {
			claims := validClaims()
			claims.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
			return sign(testSecret, claims)
		}},
		{name: "invalid signature", raw: func() string {
			return sign("another-secret-that-is-at-least-32-bytes", validClaims())
		}},
		{name: "wrong issuer", raw: func() string {
			claims := validClaims()
			claims.Issuer = "another-api"
			return sign(testSecret, claims)
		}},
		{name: "wrong audience", raw: func() string {
			claims := validClaims()
			claims.Audience = jwt.ClaimStrings{"another-client"}
			return sign(testSecret, claims)
		}},
		{name: "malformed", raw: func() string { return "not-a-jwt" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := manager.Parse(tt.raw())
			require.Error(t, err)
		})
	}
}

func TestRefreshTokensAreRandomAndHashable(t *testing.T) {
	t.Parallel()
	first, firstHash, err := NewRefreshToken()
	require.NoError(t, err)
	second, _, err := NewRefreshToken()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	require.Equal(t, firstHash, HashRefreshToken(first))
	require.Len(t, firstHash, 32)
}
