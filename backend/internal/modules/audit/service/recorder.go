package service

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	sharedaudit "github.com/example/teamops/backend/internal/shared/audit"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var auditDroppedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "teamops_audit_events_dropped_total",
	Help: "Audit events dropped before durable persistence.",
}, []string{"reason"})
var auditMetricsOnce sync.Once

type Writer interface {
	Insert(context.Context, auditmodel.Entry) error
}

// Recorder decouples business requests from audit persistence. Record never
// waits for PostgreSQL: it copies an immutable event into a bounded queue or
// drops it with an operational warning when the queue is saturated.
type Recorder struct {
	writer       Writer
	log          zerolog.Logger
	queue        chan auditmodel.Entry
	writeTimeout time.Duration
	wg           sync.WaitGroup
	queueMu      sync.RWMutex
	closed       bool
	dropped      atomic.Uint64
}

func NewRecorder(writer Writer, log zerolog.Logger, queueSize, workers int, writeTimeout time.Duration) *Recorder {
	auditMetricsOnce.Do(func() { prometheus.MustRegister(auditDroppedTotal) })
	if queueSize < 1 {
		queueSize = 1
	}
	if workers < 1 {
		workers = 1
	}
	if writeTimeout <= 0 {
		writeTimeout = 3 * time.Second
	}
	r := &Recorder{writer: writer, log: log, queue: make(chan auditmodel.Entry, queueSize), writeTimeout: writeTimeout}
	for range workers {
		r.wg.Add(1)
		go r.run()
	}
	return r
}

func (r *Recorder) Record(ctx context.Context, orgID, actorID uuid.UUID, action, resourceType, resourceID, requestID string, metadata map[string]any) error {
	if r == nil || r.writer == nil {
		return nil
	}
	request := sharedaudit.RequestInfoFromContext(ctx)
	if requestID == "" {
		requestID = request.RequestID
	}
	entry := auditmodel.Entry{
		OrganizationID: optionalUUID(orgID),
		ActorUserID:    optionalUUID(actorID),
		Action:         strings.TrimSpace(action),
		ResourceType:   strings.TrimSpace(resourceType),
		ResourceID:     strings.TrimSpace(resourceID),
		Metadata:       cloneMetadata(metadata),
		RequestID:      truncate(requestID, 128),
		IPAddress:      validIP(request.IPAddress),
		UserAgent:      truncate(strings.TrimSpace(request.UserAgent), 512),
	}
	if entry.Action == "" || entry.ResourceType == "" || entry.ResourceID == "" {
		r.drop("invalid")
		return nil
	}

	r.queueMu.RLock()
	defer r.queueMu.RUnlock()
	if r.closed {
		r.drop("closed")
		return nil
	}
	select {
	case r.queue <- entry:
	default:
		r.drop("queue_full")
	}
	return nil
}

func (r *Recorder) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.queueMu.Lock()
	if !r.closed {
		r.closed = true
		close(r.queue)
	}
	r.queueMu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Recorder) Dropped() uint64 { return r.dropped.Load() }

func (r *Recorder) run() {
	defer r.wg.Done()
	for entry := range r.queue {
		ctx, cancel := context.WithTimeout(context.Background(), r.writeTimeout)
		err := r.writer.Insert(ctx, entry)
		cancel()
		if err != nil {
			r.drop("write_failed")
			r.log.Error().Err(err).Str("action", entry.Action).Msg("audit event persistence failed")
		}
	}
}

func (r *Recorder) drop(reason string) {
	r.dropped.Add(1)
	auditDroppedTotal.WithLabelValues(reason).Inc()
}

func optionalUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	value := id
	return &value
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	copy := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if sensitiveMetadataKey(key) {
			continue
		}
		copy[key] = sanitizeMetadataValue(value)
	}
	return copy
}

func sensitiveMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	for _, sensitive := range []string{"password", "token", "secret", "authorization", "credential"} {
		if strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}

func sanitizeMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMetadata(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sanitizeMetadataValue(item)
		}
		return out
	default:
		return value
	}
}

func validIP(value string) string {
	if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
		return ip.String()
	}
	return ""
}

func truncate(value string, maxRunes int) string {
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}
