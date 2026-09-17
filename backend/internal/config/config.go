package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment      string
	HTTPAddr         string
	DatabaseURL      string
	RedisAddr        string
	RedisPassword    string
	JWTSecret        string
	JWTIssuer        string
	JWTAudience      string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	PasswordHashCost int
	ShutdownTimeout  time.Duration
	CORSOrigins      []string
	RateLimitPerMin  int
	DatabaseMaxConns int32
}

func Load() (Config, error) {
	c := Config{
		Environment: env("APP_ENV", "development"), HTTPAddr: env("HTTP_ADDR", ":8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://teamops:teamops@localhost:5432/teamops?sslmode=disable"),
		RedisAddr:   env("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"),
		JWTSecret: os.Getenv("JWT_SECRET"), JWTIssuer: env("JWT_ISSUER", "teamops-api"), JWTAudience: env("JWT_AUDIENCE", "teamops-web"), AccessTokenTTL: duration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: duration("REFRESH_TOKEN_TTL", 7*24*time.Hour), ShutdownTimeout: duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		PasswordHashCost: integer("BCRYPT_COST", 12),
		CORSOrigins:      split(env("CORS_ORIGINS", "http://localhost:5173")), RateLimitPerMin: integer("RATE_LIMIT_PER_MIN", 120),
		DatabaseMaxConns: int32(integer("DATABASE_MAX_CONNS", 20)),
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.JWTIssuer == "" || c.JWTAudience == "" {
		return Config{}, fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE must not be empty")
	}
	if c.PasswordHashCost < 10 || c.PasswordHashCost > 14 {
		return Config{}, fmt.Errorf("BCRYPT_COST must be between 10 and 14")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func duration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(env(key, fallback.String()))
	if err != nil {
		return fallback
	}
	return v
}
func integer(key string, fallback int) int {
	v, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return v
}
func split(v string) []string {
	out := []string{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
