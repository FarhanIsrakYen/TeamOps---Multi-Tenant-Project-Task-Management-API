package observability

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "teamops_http_requests_total", Help: "Total HTTP requests.",
	}, []string{"method", "route", "status"})
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "teamops_http_request_duration_seconds", Help: "HTTP request latency.", Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
	httpErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "teamops_http_errors_total", Help: "HTTP responses with a status code of 400 or greater.",
	}, []string{"method", "route", "status"})
	databaseDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "teamops_database_operation_duration_seconds", Help: "PostgreSQL operation latency without SQL statement labels.", Buckets: prometheus.DefBuckets,
	}, []string{"operation", "status"})
	redisOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "teamops_redis_operations_total", Help: "Redis operations by bounded command and result.",
	}, []string{"operation", "status"})
	redisDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "teamops_redis_operation_duration_seconds", Help: "Redis operation latency.", Buckets: prometheus.DefBuckets,
	}, []string{"operation", "status"})
	authenticationFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "teamops_authentication_failures_total", Help: "Authentication failures by bounded reason.",
	}, []string{"reason"})
	rateLimitEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "teamops_rate_limit_events_total", Help: "Rate-limit decisions and backing-store errors.",
	}, []string{"scope", "outcome"})
	metricsOnce sync.Once
)

func RegisterMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(
			httpRequests, httpDuration, httpErrors,
			databaseDuration, redisOperations, redisDuration,
			authenticationFailures, rateLimitEvents,
		)
	})
}

func ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	statusLabel := strconv.Itoa(status)
	httpRequests.WithLabelValues(method, route, statusLabel).Inc()
	httpDuration.WithLabelValues(method, route).Observe(duration.Seconds())
	if status >= 400 {
		httpErrors.WithLabelValues(method, route, statusLabel).Inc()
	}
}

func ObserveDatabase(operation string, err error, duration time.Duration) {
	databaseDuration.WithLabelValues(databaseOperation(operation), result(err)).Observe(duration.Seconds())
}

func ObserveRedis(operation string, err error, duration time.Duration) {
	operation = redisOperation(operation)
	status := result(err)
	redisOperations.WithLabelValues(operation, status).Inc()
	redisDuration.WithLabelValues(operation, status).Observe(duration.Seconds())
}

func AuthenticationFailure(reason string) {
	authenticationFailures.WithLabelValues(authenticationFailureReason(reason)).Inc()
}

func RateLimitEvent(scope, outcome string) {
	rateLimitEvents.WithLabelValues(rateLimitScope(scope), rateLimitOutcome(outcome)).Inc()
}

func result(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func databaseOperation(value string) string {
	fields := strings.Fields(strings.ToUpper(value))
	if len(fields) == 0 {
		return "OTHER"
	}
	switch fields[0] {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "BEGIN", "COMMIT", "ROLLBACK", "COPY", "WITH":
		return fields[0]
	default:
		return "OTHER"
	}
}

func redisOperation(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dial", "ping", "get", "set", "del", "incr", "expire", "mget", "mset", "exists", "eval", "evalsha", "pipeline":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "other"
	}
}

func authenticationFailureReason(value string) string {
	switch value {
	case "missing_bearer", "invalid_access_token", "invalid_credentials", "invalid_refresh_token", "login_blocked":
		return value
	default:
		return "other"
	}
}

func rateLimitScope(value string) string {
	switch value {
	case "api", "auth", "login", "test":
		return value
	default:
		return "other"
	}
}

func rateLimitOutcome(value string) string {
	switch value {
	case "limited", "store_error":
		return value
	default:
		return "other"
	}
}
