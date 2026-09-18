package guard

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type fakeMembershipReader struct {
	roles map[uuid.UUID]orgmodel.Role
	err   error
}

type countingMembershipReader struct {
	mu    sync.Mutex
	role  orgmodel.Role
	calls int
}

func (f *countingMembershipReader) Role(context.Context, uuid.UUID, uuid.UUID) (orgmodel.Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.role, nil
}

type memoryStore struct {
	mu        sync.Mutex
	values    map[string][]byte
	getErr    error
	setErr    error
	deleteErr error
}

func (s *memoryStore) GetJSON(_ context.Context, key string, dst any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return false, s.getErr
	}
	raw, ok := s.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (s *memoryStore) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setErr != nil {
		return s.setErr
	}
	raw, err := json.Marshal(value)
	if err == nil {
		s.values[key] = raw
	}
	return err
}

func (s *memoryStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for _, key := range keys {
		delete(s.values, key)
	}
	return nil
}

func (f fakeMembershipReader) Role(_ context.Context, _ uuid.UUID, userID uuid.UUID) (orgmodel.Role, error) {
	if f.err != nil {
		return "", f.err
	}
	role, ok := f.roles[userID]
	if !ok {
		return "", pgx.ErrNoRows
	}
	return role, nil
}

func TestPermissionMatrix(t *testing.T) {
	t.Parallel()
	organizationID := uuid.New()
	permissions := []Permission{ReadOrganization, ManageOrganization, DeleteOrganization, ManageMembers, ManageProjects, CreateUpdateTasks, DeleteTasks, CommentTasks, ManageLabels, ViewAuditLogs}
	tests := []struct {
		role    orgmodel.Role
		allowed map[Permission]bool
	}{
		{role: orgmodel.RoleOwner, allowed: map[Permission]bool{ReadOrganization: true, ManageOrganization: true, DeleteOrganization: true, ManageMembers: true, ManageProjects: true, CreateUpdateTasks: true, DeleteTasks: true, CommentTasks: true, ManageLabels: true, ViewAuditLogs: true}},
		{role: orgmodel.RoleAdmin, allowed: map[Permission]bool{ReadOrganization: true, ManageOrganization: true, ManageMembers: true, ManageProjects: true, CreateUpdateTasks: true, DeleteTasks: true, CommentTasks: true, ManageLabels: true, ViewAuditLogs: true}},
		{role: orgmodel.RoleMember, allowed: map[Permission]bool{ReadOrganization: true, CreateUpdateTasks: true, CommentTasks: true}},
		{role: orgmodel.RoleViewer, allowed: map[Permission]bool{ReadOrganization: true}},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			userID := uuid.New()
			guard := New(fakeMembershipReader{roles: map[uuid.UUID]orgmodel.Role{userID: tt.role}}, nil, 0)
			for _, permission := range permissions {
				role, err := guard.RequireOrganizationPermission(context.Background(), userID, organizationID, permission)
				if tt.allowed[permission] {
					require.NoError(t, err, permission)
					require.Equal(t, tt.role, role)
				} else {
					require.ErrorIs(t, err, apperror.ErrForbidden, permission)
				}
			}
		})
	}
}

func TestOrganizationMembershipAuthorization(t *testing.T) {
	t.Parallel()
	organizationID := uuid.New()
	memberID := uuid.New()
	guard := New(fakeMembershipReader{roles: map[uuid.UUID]orgmodel.Role{memberID: orgmodel.RoleMember}}, nil, 0)

	role, err := guard.RequireOrganizationMember(context.Background(), memberID, organizationID)
	require.NoError(t, err)
	require.Equal(t, orgmodel.RoleMember, role)

	_, err = guard.RequireOrganizationMember(context.Background(), uuid.New(), organizationID)
	require.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestUnknownPermissionFailsClosed(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	guard := New(fakeMembershipReader{roles: map[uuid.UUID]orgmodel.Role{userID: orgmodel.RoleOwner}}, nil, 0)
	_, err := guard.RequireOrganizationPermission(context.Background(), userID, uuid.New(), Permission("unknown"))
	require.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestOrganizationAuthorizationPropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()
	want := errors.New("database unavailable")
	guard := New(fakeMembershipReader{err: want}, nil, 0)
	_, err := guard.RequireOrganizationMember(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, want)
}

func TestMembershipCacheAsideHitMissAndInvalidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	organizationID, userID := uuid.New(), uuid.New()
	reader := &countingMembershipReader{role: orgmodel.RoleMember}
	store := &memoryStore{values: make(map[string][]byte)}
	guard := New(reader, store, time.Minute)

	for range 2 {
		role, err := guard.RequireOrganizationMember(ctx, userID, organizationID)
		require.NoError(t, err)
		require.Equal(t, orgmodel.RoleMember, role)
	}
	require.Equal(t, 1, reader.calls, "the second lookup should be served from cache")

	guard.InvalidateMembership(ctx, organizationID, userID)
	_, err := guard.RequireOrganizationMember(ctx, userID, organizationID)
	require.NoError(t, err)
	require.Equal(t, 2, reader.calls, "invalidation should force a repository lookup")
}

func TestMembershipCacheFailureFallsBackToRepository(t *testing.T) {
	t.Parallel()
	reader := &countingMembershipReader{role: orgmodel.RoleAdmin}
	store := &memoryStore{
		values: make(map[string][]byte),
		getErr: errors.New("redis unavailable"),
		setErr: errors.New("redis unavailable"),
	}
	guard := New(reader, store, time.Minute)

	role, err := guard.RequireOrganizationMember(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, orgmodel.RoleAdmin, role)
	require.Equal(t, 1, reader.calls)
}
