package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	sharedaudit "github.com/example/teamops/backend/internal/shared/audit"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type recordingWriter struct {
	mu      sync.Mutex
	entries []auditmodel.Entry
	err     error
	started chan struct{}
	release chan struct{}
}

func (w *recordingWriter) Insert(_ context.Context, entry auditmodel.Entry) error {
	if w.started != nil {
		select {
		case w.started <- struct{}{}:
		default:
		}
	}
	if w.release != nil {
		<-w.release
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, entry)
	return w.err
}

func TestRecorderCapturesRequestContextAndScrubsSecrets(t *testing.T) {
	t.Parallel()
	writer := &recordingWriter{}
	recorder := NewRecorder(writer, zerolog.Nop(), 4, 1, time.Second)
	ctx := sharedaudit.ContextWithRequestInfo(context.Background(), sharedaudit.RequestInfo{
		RequestID: "request-1", IPAddress: "192.0.2.25", UserAgent: "TeamOps test client",
	})
	organizationID, actorID := uuid.New(), uuid.New()
	require.NoError(t, recorder.Record(ctx, organizationID, actorID, "project.created", "project", "resource-1", "", map[string]any{
		"name": "Safe", "accessToken": "must-not-be-stored", "nested": map[string]any{"password": "must-not-be-stored"},
	}))
	require.NoError(t, recorder.Close(context.Background()))

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

func TestRecorderIsNonBlockingAndCountsDroppedEvents(t *testing.T) {
	t.Parallel()
	writer := &recordingWriter{started: make(chan struct{}, 1), release: make(chan struct{})}
	recorder := NewRecorder(writer, zerolog.Nop(), 1, 1, time.Second)
	ctx := context.Background()
	require.NoError(t, recorder.Record(ctx, uuid.New(), uuid.New(), "first", "test", "1", "", nil))
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	require.NoError(t, recorder.Record(ctx, uuid.New(), uuid.New(), "second", "test", "2", "", nil))
	require.NoError(t, recorder.Record(ctx, uuid.New(), uuid.New(), "third", "test", "3", "", nil))
	require.Equal(t, uint64(1), recorder.Dropped())
	close(writer.release)
	require.NoError(t, recorder.Close(context.Background()))
}

func TestRecorderPersistenceFailureDoesNotReachBusinessCaller(t *testing.T) {
	t.Parallel()
	writer := &recordingWriter{err: errors.New("database unavailable")}
	recorder := NewRecorder(writer, zerolog.Nop(), 1, 1, time.Second)
	require.NoError(t, recorder.Record(context.Background(), uuid.Nil, uuid.New(), "auth.login", "user", uuid.NewString(), "", nil))
	require.NoError(t, recorder.Close(context.Background()))
	require.Equal(t, uint64(1), recorder.Dropped())
}
