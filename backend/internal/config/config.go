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
	Environment                      string
	HTTPAddr                         string
	DatabaseURL                      string
	RedisAddr                        string
	RedisPassword                    string
	OrganizationCacheTTL             time.Duration
	ProjectCacheTTL                  time.Duration
	JWTSecret                        string
	JWTIssuer                        string
	JWTAudience                      string
	AccessTokenTTL                   time.Duration
	RefreshTokenTTL                  time.Duration
	PasswordHashCost                 int
	ShutdownTimeout                  time.Duration
	CORSOrigins                      []string
	RateLimitPerMin                  int
	AuthRateLimitPerMin              int
	LoginRateLimitPerMin             int
	LoginFailureLimit                int
	LoginFailureWindow               time.Duration
	RequestBodyMaxBytes              int64
	TrustedProxies                   []string
	JobQueueSize                     int
	JobWorkers                       int
	JobMaxRetries                    int
	JobInitialBackoff                time.Duration
	JobMaxBackoff                    time.Duration
	JobTimeout                       time.Duration
	StaleTaskAfter                   time.Duration
	StaleTaskScanInterval            time.Duration
	ProjectStatisticsRefreshInterval time.Duration
	SessionCleanupInterval           time.Duration
	OTelEndpoint                     string
	OTelInsecure                     bool
	OTelServiceName                  string
	OTelSampleRatio                  float64
	DatabaseMaxConns                 int32
	DatabaseMinConns                 int32
	DatabaseMaxConnLifetime          time.Duration
	DatabaseMaxConnIdleTime          time.Duration
	DatabaseHealthCheckPeriod        time.Duration
	DatabaseStatementTimeout         time.Duration
	DatabaseLockTimeout              time.Duration
	DatabaseIdleInTxTimeout          time.Duration
}

func Load() (Config, error) {
	if err := validateTypedEnvironment(); err != nil {
		return Config{}, err
	}
	c := Config{
		Environment:                      env("APP_ENV", "development"),
		HTTPAddr:                         env("HTTP_ADDR", ":8080"),
		DatabaseURL:                      env("DATABASE_URL", "postgres://teamops:teamops@localhost:5432/teamops?sslmode=disable"),
		RedisAddr:                        env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:                    os.Getenv("REDIS_PASSWORD"),
		OrganizationCacheTTL:             duration("ORGANIZATION_CACHE_TTL", 5*time.Minute),
		ProjectCacheTTL:                  duration("PROJECT_CACHE_TTL", 5*time.Minute),
		JWTSecret:                        os.Getenv("JWT_SECRET"),
		JWTIssuer:                        env("JWT_ISSUER", "teamops-api"),
		JWTAudience:                      env("JWT_AUDIENCE", "teamops-web"),
		AccessTokenTTL:                   duration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:                  duration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		PasswordHashCost:                 integer("BCRYPT_COST", 12),
		ShutdownTimeout:                  duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		CORSOrigins:                      split(env("CORS_ORIGINS", "http://localhost:5173")),
		RateLimitPerMin:                  integer("RATE_LIMIT_PER_MIN", 120),
		AuthRateLimitPerMin:              integer("AUTH_RATE_LIMIT_PER_MIN", 30),
		LoginRateLimitPerMin:             integer("LOGIN_RATE_LIMIT_PER_MIN", 10),
		LoginFailureLimit:                integer("LOGIN_FAILURE_LIMIT", 5),
		LoginFailureWindow:               duration("LOGIN_FAILURE_WINDOW", 15*time.Minute),
		RequestBodyMaxBytes:              int64(integer("REQUEST_BODY_MAX_BYTES", 1<<20)),
		TrustedProxies:                   split(os.Getenv("TRUSTED_PROXIES")),
		JobQueueSize:                     integer("JOB_QUEUE_SIZE", 1024),
		JobWorkers:                       integer("JOB_WORKERS", 4),
		JobMaxRetries:                    integer("JOB_MAX_RETRIES", 3),
		JobInitialBackoff:                duration("JOB_INITIAL_BACKOFF", 100*time.Millisecond),
		JobMaxBackoff:                    duration("JOB_MAX_BACKOFF", 5*time.Second),
		JobTimeout:                       duration("JOB_TIMEOUT", 10*time.Second),
		StaleTaskAfter:                   duration("STALE_TASK_AFTER", 72*time.Hour),
		StaleTaskScanInterval:            duration("STALE_TASK_SCAN_INTERVAL", 15*time.Minute),
		ProjectStatisticsRefreshInterval: duration("PROJECT_STATISTICS_REFRESH_INTERVAL", 5*time.Minute),
		SessionCleanupInterval:           duration("SESSION_CLEANUP_INTERVAL", time.Hour),
		OTelEndpoint:                     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTelInsecure:                     boolean("OTEL_EXPORTER_OTLP_INSECURE", false),
		OTelServiceName:                  env("OTEL_SERVICE_NAME", "teamops-api"),
		OTelSampleRatio:                  decimal("OTEL_TRACE_SAMPLE_RATIO", 0.1),
		DatabaseMaxConns:                 int32(integer("DATABASE_MAX_CONNS", 20)),
		DatabaseMinConns:                 int32(integer("DATABASE_MIN_CONNS", 2)),
		DatabaseMaxConnLifetime:          duration("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute),
		DatabaseMaxConnIdleTime:          duration("DATABASE_MAX_CONN_IDLE_TIME", 5*time.Minute),
		DatabaseHealthCheckPeriod:        duration("DATABASE_HEALTH_CHECK_PERIOD", time.Minute),
		DatabaseStatementTimeout:         duration("DATABASE_STATEMENT_TIMEOUT", 15*time.Second),
		DatabaseLockTimeout:              duration("DATABASE_LOCK_TIMEOUT", 5*time.Second),
		DatabaseIdleInTxTimeout:          duration("DATABASE_IDLE_IN_TRANSACTION_TIMEOUT", 30*time.Second),
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.JWTIssuer == "" || c.JWTAudience == "" {
		return Config{}, fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE must not be empty")
	}
	if c.AccessTokenTTL <= 0 || c.AccessTokenTTL > time.Hour || c.RefreshTokenTTL <= c.AccessTokenTTL || c.RefreshTokenTTL > 90*24*time.Hour || c.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("token TTLs and shutdown timeout are invalid")
	}
	if c.OrganizationCacheTTL < 0 || c.ProjectCacheTTL < 0 {
		return Config{}, fmt.Errorf("cache TTLs must not be negative")
	}
	if c.DatabaseMaxConns < 1 || c.DatabaseMinConns < 0 || c.DatabaseMinConns > c.DatabaseMaxConns || c.DatabaseMaxConnLifetime <= 0 || c.DatabaseMaxConnIdleTime <= 0 || c.DatabaseHealthCheckPeriod <= 0 || c.DatabaseStatementTimeout <= 0 || c.DatabaseLockTimeout <= 0 || c.DatabaseIdleInTxTimeout <= 0 {
		return Config{}, fmt.Errorf("database pool settings are invalid")
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
	if c.JobQueueSize < 1 || c.JobWorkers < 1 || c.JobMaxRetries < 0 || c.JobInitialBackoff <= 0 || c.JobMaxBackoff < c.JobInitialBackoff || c.JobTimeout <= 0 {
		return Config{}, fmt.Errorf("job worker settings are invalid")
	}
	if c.StaleTaskAfter <= 0 || c.StaleTaskScanInterval <= 0 || c.ProjectStatisticsRefreshInterval <= 0 || c.SessionCleanupInterval <= 0 {
		return Config{}, fmt.Errorf("job schedule settings must be positive")
	}
	if c.OTelServiceName == "" || c.OTelSampleRatio < 0 || c.OTelSampleRatio > 1 {
		return Config{}, fmt.Errorf("OpenTelemetry service name and sample ratio are invalid")
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

func validateTypedEnvironment() error {
	durationKeys := []string{
		"ORGANIZATION_CACHE_TTL", "PROJECT_CACHE_TTL", "ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "SHUTDOWN_TIMEOUT",
		"LOGIN_FAILURE_WINDOW", "JOB_INITIAL_BACKOFF", "JOB_MAX_BACKOFF", "JOB_TIMEOUT", "STALE_TASK_AFTER",
		"STALE_TASK_SCAN_INTERVAL", "PROJECT_STATISTICS_REFRESH_INTERVAL", "SESSION_CLEANUP_INTERVAL",
		"DATABASE_MAX_CONN_LIFETIME", "DATABASE_MAX_CONN_IDLE_TIME", "DATABASE_HEALTH_CHECK_PERIOD",
		"DATABASE_STATEMENT_TIMEOUT", "DATABASE_LOCK_TIMEOUT", "DATABASE_IDLE_IN_TRANSACTION_TIMEOUT",
	}
	for _, key := range durationKeys {
		if raw := os.Getenv(key); raw != "" {
			if _, err := time.ParseDuration(raw); err != nil {
				return fmt.Errorf("%s must be a valid duration: %w", key, err)
			}
		}
	}
	integerKeys := []string{
		"BCRYPT_COST", "RATE_LIMIT_PER_MIN", "AUTH_RATE_LIMIT_PER_MIN", "LOGIN_RATE_LIMIT_PER_MIN", "LOGIN_FAILURE_LIMIT",
		"REQUEST_BODY_MAX_BYTES", "JOB_QUEUE_SIZE", "JOB_WORKERS", "JOB_MAX_RETRIES", "DATABASE_MAX_CONNS", "DATABASE_MIN_CONNS",
	}
	for _, key := range integerKeys {
		if raw := os.Getenv(key); raw != "" {
			if _, err := strconv.Atoi(raw); err != nil {
				return fmt.Errorf("%s must be a valid integer: %w", key, err)
			}
		}
	}
	if raw := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); raw != "" {
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("OTEL_EXPORTER_OTLP_INSECURE must be a valid boolean: %w", err)
		}
	}
	if raw := os.Getenv("OTEL_TRACE_SAMPLE_RATIO"); raw != "" {
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return fmt.Errorf("OTEL_TRACE_SAMPLE_RATIO must be a valid number: %w", err)
		}
	}
	return nil
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
func boolean(key string, fallback bool) bool {
	v, err := strconv.ParseBool(env(key, strconv.FormatBool(fallback)))
	if err != nil {
		return fallback
	}
	return v
}
func decimal(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(env(key, strconv.FormatFloat(fallback, 'f', -1, 64)), 64)
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
