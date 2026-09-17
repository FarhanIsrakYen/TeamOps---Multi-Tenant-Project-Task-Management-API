package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/example/teamops/backend/internal/cache"
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
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
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
func RateLimit(cache *cache.Cache, limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := c.ClientIP()
		if id, ok := c.Get("user_id"); ok {
			identity = id.(uuid.UUID).String()
		}
		bucket := time.Now().UTC().Format("200601021504")
		n, err := cache.Increment(c, "rate:"+identity+":"+bucket, time.Minute)
		if err == nil && n > int64(limit) {
			c.Header("Retry-After", "60")
			response.Error(c, apperror.New(http.StatusTooManyRequests, "rate_limited", "too many requests"))
			c.Abort()
			return
		}
		c.Next()
	}
}
