package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment          string
	HTTPAddr             string
	DatabaseURL          string
	RedisAddr            string
	RedisPassword        string
	OrganizationCacheTTL time.Duration
	ProjectCacheTTL      time.Duration
	MembershipCacheTTL   time.Duration
	JWTSecret            string
	JWTIssuer            string
	JWTAudience          string
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	PasswordHashCost     int
	ShutdownTimeout      time.Duration
	CORSOrigins          []string
	RateLimitPerMin      int
	AuthRateLimitPerMin  int
	LoginRateLimitPerMin int
	LoginFailureLimit    int
	LoginFailureWindow   time.Duration
	RequestBodyMaxBytes  int64
	TrustedProxies       []string
	AuditQueueSize       int
	AuditWorkers         int
	AuditWriteTimeout    time.Duration
	DatabaseMaxConns     int32
}

func Load() (Config, error) {
	c := Config{
		Environment:          env("APP_ENV", "development"),
		HTTPAddr:             env("HTTP_ADDR", ":8080"),
		DatabaseURL:          env("DATABASE_URL", "postgres://teamops:teamops@localhost:5432/teamops?sslmode=disable"),
		RedisAddr:            env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        os.Getenv("REDIS_PASSWORD"),
		OrganizationCacheTTL: duration("ORGANIZATION_CACHE_TTL", 5*time.Minute),
		ProjectCacheTTL:      duration("PROJECT_CACHE_TTL", 5*time.Minute),
		MembershipCacheTTL:   duration("MEMBERSHIP_CACHE_TTL", 30*time.Second),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		JWTIssuer:            env("JWT_ISSUER", "teamops-api"),
		JWTAudience:          env("JWT_AUDIENCE", "teamops-web"),
		AccessTokenTTL:       duration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:      duration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		PasswordHashCost:     integer("BCRYPT_COST", 12),
		ShutdownTimeout:      duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		CORSOrigins:          split(env("CORS_ORIGINS", "http://localhost:5173")),
		RateLimitPerMin:      integer("RATE_LIMIT_PER_MIN", 120),
		AuthRateLimitPerMin:  integer("AUTH_RATE_LIMIT_PER_MIN", 30),
		LoginRateLimitPerMin: integer("LOGIN_RATE_LIMIT_PER_MIN", 10),
		LoginFailureLimit:    integer("LOGIN_FAILURE_LIMIT", 5),
		LoginFailureWindow:   duration("LOGIN_FAILURE_WINDOW", 15*time.Minute),
		RequestBodyMaxBytes:  int64(integer("REQUEST_BODY_MAX_BYTES", 1<<20)),
		TrustedProxies:       split(os.Getenv("TRUSTED_PROXIES")),
		AuditQueueSize:       integer("AUDIT_QUEUE_SIZE", 1024),
		AuditWorkers:         integer("AUDIT_WORKERS", 2),
		AuditWriteTimeout:    duration("AUDIT_WRITE_TIMEOUT", 3*time.Second),
		DatabaseMaxConns:     int32(integer("DATABASE_MAX_CONNS", 20)),
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
	if c.RateLimitPerMin < 1 || c.AuthRateLimitPerMin < 1 || c.LoginRateLimitPerMin < 1 || c.LoginFailureLimit < 1 {
		return Config{}, fmt.Errorf("rate limits must be positive")
	}
	if c.LoginFailureWindow <= 0 || c.RequestBodyMaxBytes < 1 {
		return Config{}, fmt.Errorf("LOGIN_FAILURE_WINDOW and REQUEST_BODY_MAX_BYTES must be positive")
	}
	if c.AuditQueueSize < 1 || c.AuditWorkers < 1 || c.AuditWriteTimeout <= 0 {
		return Config{}, fmt.Errorf("audit queue settings must be positive")
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			return Config{}, fmt.Errorf("CORS_ORIGINS must not contain a wildcard")
		}
	}
	for _, proxy := range c.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return Config{}, fmt.Errorf("TRUSTED_PROXIES contains invalid IP or CIDR %q", proxy)
			}
		}
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
