package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	"github.com/example/teamops/backend/internal/platform/jobs"
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
	writer    Writer
	submitter jobs.Submitter
	log       zerolog.Logger
	dropped   atomic.Uint64
}

func NewRecorder(writer Writer, submitter jobs.Submitter, log zerolog.Logger) *Recorder {
	auditMetricsOnce.Do(func() { prometheus.MustRegister(auditDroppedTotal) })
	return &Recorder{writer: writer, submitter: submitter, log: log}
}

func (r *Recorder) Record(ctx context.Context, orgID, actorID uuid.UUID, action, resourceType, resourceID, requestID string, metadata map[string]any) error {
	if r == nil || r.writer == nil || r.submitter == nil {
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

	err := r.submitter.Submit(jobs.JobFunc{JobName: "audit_event_processing", Handler: func(jobCtx context.Context) error {
		return r.writer.Insert(jobCtx, entry)
	}})
	if err != nil {
		reason := "submit_failed"
		if errors.Is(err, jobs.ErrQueueFull) {
			reason = "queue_full"
		} else if errors.Is(err, jobs.ErrClosed) {
			reason = "closed"
		}
		r.drop(reason)
		r.log.Warn().Err(err).Str("action", entry.Action).Msg("audit event was not queued")
	}
	return nil
}

func (r *Recorder) Dropped() uint64 { return r.dropped.Load() }

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
