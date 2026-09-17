package service

import (
	"context"
	"testing"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type authorizationMemberships struct{ roles map[uuid.UUID]orgmodel.Role }

func (m authorizationMemberships) Role(_ context.Context, _ uuid.UUID, userID uuid.UUID) (orgmodel.Role, error) {
	role, ok := m.roles[userID]
	if !ok {
		return "", pgx.ErrNoRows
	}
	return role, nil
}

func TestMemberCannotManageOrganizationMembership(t *testing.T) {
	t.Parallel()
	memberID := uuid.New()
	guard := orgguard.New(authorizationMemberships{roles: map[uuid.UUID]orgmodel.Role{memberID: orgmodel.RoleMember}})
	service := New(nil, nil, nil, guard)

	_, err := service.AddMember(context.Background(), memberID, uuid.New(), "new@example.com", orgmodel.RoleViewer, "request")
	require.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestRoleAssignmentPreventsOwnerPrivilegeEscalation(t *testing.T) {
	t.Parallel()
	adminID := uuid.New()
	guard := orgguard.New(authorizationMemberships{roles: map[uuid.UUID]orgmodel.Role{adminID: orgmodel.RoleAdmin}})
	service := New(nil, nil, nil, guard)

	_, err := service.AddMember(context.Background(), adminID, uuid.New(), "new@example.com", orgmodel.RoleOwner, "request")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 403, appErr.Status)
	require.Equal(t, "owner_assignment_forbidden", appErr.Code)
}

func TestViewerCannotMutateOrganizationAndAdminCannotDeleteIt(t *testing.T) {
	t.Parallel()
	viewerID := uuid.New()
	adminID := uuid.New()
	guard := orgguard.New(authorizationMemberships{roles: map[uuid.UUID]orgmodel.Role{
		viewerID: orgmodel.RoleViewer,
		adminID:  orgmodel.RoleAdmin,
	}})
	service := New(nil, nil, nil, guard)
	organizationID := uuid.New()

	_, err := service.Update(context.Background(), viewerID, organizationID, "Changed", "changed", 1, "request")
	require.ErrorIs(t, err, apperror.ErrForbidden)
	require.ErrorIs(t, service.Delete(context.Background(), adminID, organizationID), apperror.ErrForbidden)
}
