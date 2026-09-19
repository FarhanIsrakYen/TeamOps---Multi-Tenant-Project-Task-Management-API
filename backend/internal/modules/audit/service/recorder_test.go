package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	"github.com/example/teamops/backend/internal/platform/jobs"
	sharedaudit "github.com/example/teamops/backend/internal/shared/audit"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type recordingWriter struct {
	mu      sync.Mutex
	entries []auditmodel.Entry
	err     error
}

func (w *recordingWriter) Insert(_ context.Context, entry auditmodel.Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, entry)
	return w.err
}

type immediateSubmitter struct{}

func (immediateSubmitter) Submit(job jobs.Job) error {
	_ = job.Run(context.Background())
	return nil
}

type rejectingSubmitter struct{ err error }

func (s rejectingSubmitter) Submit(jobs.Job) error { return s.err }

func TestRecorderCapturesRequestContextAndScrubsSecrets(t *testing.T) {
	t.Parallel()
	writer := &recordingWriter{}
	recorder := NewRecorder(writer, immediateSubmitter{}, zerolog.Nop())
	ctx := sharedaudit.ContextWithRequestInfo(context.Background(), sharedaudit.RequestInfo{
		RequestID: "request-1", IPAddress: "192.0.2.25", UserAgent: "TeamOps test client",
	})
	organizationID, actorID := uuid.New(), uuid.New()
	require.NoError(t, recorder.Record(ctx, organizationID, actorID, "project.created", "project", "resource-1", "", map[string]any{
		"name": "Safe", "accessToken": "must-not-be-stored", "nested": map[string]any{"password": "must-not-be-stored"},
	}))

	require.Len(t, writer.entries, 1)
	entry := writer.entries[0]
	require.Equal(t, &organizationID, entry.OrganizationID)
	require.Equal(t, &actorID, entry.ActorUserID)
	require.Equal(t, "request-1", entry.RequestID)
	require.Equal(t, "192.0.2.25", entry.IPAddress)
	require.Equal(t, "TeamOps test client", entry.UserAgent)
	require.Equal(t, "Safe", entry.Metadata["name"])
	require.NotContains(t, entry.Metadata, "accessToken")
	require.Empty(t, entry.Metadata["nested"].(map[string]any))
}

func TestRecorderCountsQueueRejectionWithoutFailingBusinessCaller(t *testing.T) {
	t.Parallel()
	recorder := NewRecorder(&recordingWriter{}, rejectingSubmitter{err: jobs.ErrQueueFull}, zerolog.Nop())
	require.NoError(t, recorder.Record(context.Background(), uuid.New(), uuid.New(), "project.created", "project", "1", "", nil))
	require.Equal(t, uint64(1), recorder.Dropped())
}

func TestRecorderPersistenceFailureDoesNotReachBusinessCaller(t *testing.T) {
	t.Parallel()
	writer := &recordingWriter{err: errors.New("database unavailable")}
	recorder := NewRecorder(writer, immediateSubmitter{}, zerolog.Nop())
	require.NoError(t, recorder.Record(context.Background(), uuid.Nil, uuid.New(), "auth.login", "user", uuid.NewString(), "", nil))
	require.Len(t, writer.entries, 1)
}
