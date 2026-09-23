package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	"github.com/example/teamops/backend/internal/platform/observability"
	sharedaudit "github.com/example/teamops/backend/internal/shared/audit"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func RegisterMetrics() { observability.RegisterMetrics() }
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
		status := c.Writer.Status()
		event := log.Info()
		if status >= http.StatusInternalServerError {
			event = log.Error()
		} else if status >= http.StatusBadRequest {
			event = log.Warn()
		}
		event = event.Str("request_id", c.GetString("request_id")).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Int("status", status).Dur("duration", time.Since(start))
		if status >= http.StatusInternalServerError && len(c.Errors) > 0 {
			event = event.Err(c.Errors.Last().Err)
		}
		if userID := requestUserID(c); userID != "" {
			event = event.Str("user_id", userID)
		}
		event = withTraceContext(event, c)
		event.Msg("request completed")
	}
}
func Recovery(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if v := recover(); v != nil {
				event := log.Error().Str("request_id", c.GetString("request_id")).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Bytes("stack", debug.Stack())
				if userID := requestUserID(c); userID != "" {
					event = event.Str("user_id", userID)
				}
				event = withTraceContext(event, c)
				event.Msg("request panic")
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
		observability.ObserveHTTPRequest(c.Request.Method, route, c.Writer.Status(), time.Since(start))
	}
}

func Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := otel.Tracer("teamops/http").Start(ctx, "HTTP "+c.Request.Method,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("teamops.request_id", c.GetString("request_id")),
			),
		)
		c.Request = c.Request.WithContext(ctx)
		defer func() {
			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}
			status := c.Writer.Status()
			span.SetName(c.Request.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
			if userID := requestUserID(c); userID != "" {
				span.SetAttributes(attribute.String("user.id", userID))
			}
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			span.End()
		}()
		c.Next()
	}
}

func requestUserID(c *gin.Context) string {
	value, exists := c.Get("user_id")
	if !exists || value == nil {
		return ""
	}
	if stringer, ok := value.(fmt.Stringer); ok {
		return stringer.String()
	}
	if value, ok := value.(string); ok {
		return value
	}
	return ""
}

func withTraceContext(event *zerolog.Event, c *gin.Context) *zerolog.Event {
	spanContext := trace.SpanContextFromContext(c.Request.Context())
	if !spanContext.IsValid() {
		return event
	}
	return event.Str("trace_id", spanContext.TraceID().String()).Str("span_id", spanContext.SpanID().String())
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
			observability.RateLimitEvent(scope, "limited")
			retryAfter := windowSeconds - now.Unix()%windowSeconds
			c.Header("Retry-After", fmt.Sprint(retryAfter))
			c.Header("X-RateLimit-Limit", fmt.Sprint(limit))
			c.Header("X-RateLimit-Remaining", "0")
			response.Error(c, apperror.New(http.StatusTooManyRequests, "rate_limited", "too many requests"))
			c.Abort()
			return
		}
		if err != nil {
			observability.RateLimitEvent(scope, "store_error")
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
