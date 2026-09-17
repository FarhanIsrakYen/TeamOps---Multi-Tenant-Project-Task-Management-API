package guard

import (
	"context"
	"testing"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	taskmodel "github.com/example/teamops/backend/internal/modules/tasks/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type memberships struct {
	roles map[[2]uuid.UUID]orgmodel.Role
}

func (m memberships) Role(_ context.Context, organizationID, userID uuid.UUID) (orgmodel.Role, error) {
	role, ok := m.roles[[2]uuid.UUID{organizationID, userID}]
	if !ok {
		return "", pgx.ErrNoRows
	}
	return role, nil
}

func TestRequireTaskAccessUsesPersistedOrganization(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	organizationID := uuid.New()
	otherOrganizationID := uuid.New()
	organizations := orgguard.New(memberships{roles: map[[2]uuid.UUID]orgmodel.Role{
		{organizationID, userID}: orgmodel.RoleAdmin,
	}})
	guard := New(organizations)

	require.NoError(t, guard.RequireTaskAccess(context.Background(), userID, taskmodel.Task{ID: uuid.New(), OrganizationID: organizationID}, orgguard.DeleteTasks))
	err := guard.RequireTaskAccess(context.Background(), userID, taskmodel.Task{ID: uuid.New(), OrganizationID: otherOrganizationID}, orgguard.ReadOrganization)
	require.ErrorIs(t, err, apperror.ErrForbidden)
	err = guard.RequireTaskAccess(context.Background(), userID, taskmodel.Task{}, orgguard.ReadOrganization)
	require.ErrorIs(t, err, apperror.ErrNotFound)
}
