package guard

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	"github.com/example/teamops/backend/internal/shared/errors"
)

type AttemptStore interface {
	Increment(context.Context, string, time.Duration) (int64, error)
	Delete(context.Context, ...string) error
}

// LoginProtector limits failed-login accumulation by both source IP and
// normalized account identifier. Redis errors fail open so authentication
// remains available during a cache outage; the endpoint IP limiter still
// provides an independent edge-friendly layer when Redis is healthy.
type LoginProtector struct {
	store  AttemptStore
	limit  int
	window time.Duration
}

func NewLoginProtector(store AttemptStore, limit int, window time.Duration) *LoginProtector {
	return &LoginProtector{store: store, limit: limit, window: window}
}

func (p *LoginProtector) Check(ctx context.Context, ip, email string) error {
	if p == nil || p.store == nil || p.limit <= 0 || p.window <= 0 {
		return nil
	}
	for _, key := range p.keys(ip, email) {
		attempts, err := p.store.Increment(ctx, key, p.window)
		if err != nil {
			continue
		}
		if attempts > int64(p.limit) {
			return apperror.New(http.StatusTooManyRequests, "login_temporarily_blocked", "too many login attempts; try again later")
		}
	}
	return nil
}

func (p *LoginProtector) Reset(ctx context.Context, ip, email string) {
	if p == nil || p.store == nil {
		return
	}
	_ = p.store.Delete(ctx, p.keys(ip, email)...)
}

func (p *LoginProtector) keys(ip, email string) []string {
	return []string{
		cache.LoginAttemptKey("ip", strings.TrimSpace(ip)),
		cache.LoginAttemptKey("account", strings.ToLower(strings.TrimSpace(email))),
	}
}
