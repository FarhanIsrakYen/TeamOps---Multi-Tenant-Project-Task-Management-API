package guard

import (
	"context"
	"errors"
	"testing"

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
			guard := New(fakeMembershipReader{roles: map[uuid.UUID]orgmodel.Role{userID: tt.role}})
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
	guard := New(fakeMembershipReader{roles: map[uuid.UUID]orgmodel.Role{memberID: orgmodel.RoleMember}})

	role, err := guard.RequireOrganizationMember(context.Background(), memberID, organizationID)
	require.NoError(t, err)
	require.Equal(t, orgmodel.RoleMember, role)

	_, err = guard.RequireOrganizationMember(context.Background(), uuid.New(), organizationID)
	require.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestUnknownPermissionFailsClosed(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	guard := New(fakeMembershipReader{roles: map[uuid.UUID]orgmodel.Role{userID: orgmodel.RoleOwner}})
	_, err := guard.RequireOrganizationPermission(context.Background(), userID, uuid.New(), Permission("unknown"))
	require.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestOrganizationAuthorizationPropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()
	want := errors.New("database unavailable")
	guard := New(fakeMembershipReader{err: want})
	_, err := guard.RequireOrganizationMember(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, want)
}
