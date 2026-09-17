package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"uid"`
	Email  string    `json:"email"`
}
type TokenManager struct {
	secret    []byte
	issuer    string
	audience  string
	accessTTL time.Duration
}

type Identity struct {
	UserID  uuid.UUID
	Email   string
	TokenID string
}

type identityContextKey struct{}

func NewTokenManager(secret, issuer, audience string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, audience: audience, accessTTL: ttl}
}
func (m *TokenManager) AccessToken(userID uuid.UUID, email string) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(m.accessTTL)
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: m.issuer, Subject: userID.String(), Audience: jwt.ClaimStrings{m.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp), ID: uuid.NewString()}, UserID: userID, Email: email}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return raw, exp, err
}
func (m *TokenManager) Parse(raw string) (Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience), jwt.WithIssuedAt(), jwt.WithExpirationRequired())
	if err != nil {
		return Claims{}, err
	}
	c, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || c.UserID == uuid.Nil || c.Subject != c.UserID.String() || c.ID == "" {
		return Claims{}, errors.New("invalid token")
	}
	return *c, nil
}
func NewRefreshToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}
func HashRefreshToken(raw string) []byte { sum := sha256.Sum256([]byte(raw)); return sum[:] }

func ContextWithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}
