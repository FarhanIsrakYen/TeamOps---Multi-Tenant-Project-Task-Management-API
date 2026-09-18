package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	sharedaudit "github.com/example/teamops/backend/internal/shared/audit"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var requests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "teamops_http_requests_total", Help: "Total HTTP requests."}, []string{"method", "route", "status"})
var latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "teamops_http_request_duration_seconds", Help: "HTTP request latency."}, []string{"method", "route"})
var metricsOnce sync.Once

func RegisterMetrics() { metricsOnce.Do(func() { prometheus.MustRegister(requests, latency) }) }
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" || len(id) > 128 || strings.ContainsAny(id, "\r\n\t") {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Request = c.Request.WithContext(sharedaudit.ContextWithRequestInfo(c.Request.Context(), sharedaudit.RequestInfo{
			RequestID: id,
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}))
		c.Next()
	}
}

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			response.Error(c, apperror.New(http.StatusRequestEntityTooLarge, "request_body_too_large", "request body exceeds the allowed size"))
			c.Abort()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func SecureHeaders(production bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Header("Cache-Control", "no-store")
		if production {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			response.Error(c, apperror.New(http.StatusForbidden, "cors_origin_forbidden", "origin is not allowed"))
			c.Abort()
			return
		}
		c.Writer.Header().Add("Vary", "Origin")
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID,X-RateLimit-Limit,X-RateLimit-Remaining,Retry-After")
		c.Header("Access-Control-Max-Age", "43200")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func Logging(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info().Str("request_id", c.GetString("request_id")).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Int("status", c.Writer.Status()).Dur("duration", time.Since(start)).Msg("request completed")
	}
}
func Recovery(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if v := recover(); v != nil {
				log.Error().Interface("panic", v).Bytes("stack", debug.Stack()).Msg("request panic")
				response.Error(c, fmt.Errorf("panic: %v", v))
				c.Abort()
			}
		}()
		c.Next()
	}
}
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		requests.WithLabelValues(c.Request.Method, route, fmt.Sprint(c.Writer.Status())).Inc()
		latency.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}

type Counter interface {
	Increment(context.Context, string, time.Duration) (int64, error)
}

func RateLimit(counter Counter, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if counter == nil || limit <= 0 || window <= 0 {
			c.Next()
			return
		}
		now := time.Now().UTC()
		windowSeconds := int64(window / time.Second)
		if windowSeconds < 1 {
			windowSeconds = 1
		}
		bucket := now.Unix() / windowSeconds
		key := cache.RateLimitKey(scope, c.ClientIP(), bucket)
		n, err := counter.Increment(c.Request.Context(), key, window)
		if err == nil && n > int64(limit) {
			retryAfter := windowSeconds - now.Unix()%windowSeconds
			c.Header("Retry-After", fmt.Sprint(retryAfter))
			c.Header("X-RateLimit-Limit", fmt.Sprint(limit))
			c.Header("X-RateLimit-Remaining", "0")
			response.Error(c, apperror.New(http.StatusTooManyRequests, "rate_limited", "too many requests"))
			c.Abort()
			return
		}
		if err == nil {
			remaining := int64(limit) - n
			if remaining < 0 {
				remaining = 0
			}
			c.Header("X-RateLimit-Limit", fmt.Sprint(limit))
			c.Header("X-RateLimit-Remaining", fmt.Sprint(remaining))
		}
		c.Next()
	}
}
