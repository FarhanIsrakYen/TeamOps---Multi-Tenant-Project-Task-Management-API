package guard

import (
	"context"
	"testing"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
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

func TestRequireProjectAccessPreventsCrossOrganizationAccess(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	memberOrganizationID := uuid.New()
	projectOrganizationID := uuid.New()
	organizations := orgguard.New(memberships{roles: map[[2]uuid.UUID]orgmodel.Role{
		{memberOrganizationID, userID}: orgmodel.RoleOwner,
	}})
	guard := New(organizations)
	project := projectmodel.Project{ID: uuid.New(), OrganizationID: projectOrganizationID}

	err := guard.RequireProjectAccess(context.Background(), userID, project, orgguard.ReadOrganization)
	require.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestRequireProjectAccessHandlesAllowedAndMissingResources(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	organizationID := uuid.New()
	organizations := orgguard.New(memberships{roles: map[[2]uuid.UUID]orgmodel.Role{
		{organizationID, userID}: orgmodel.RoleViewer,
	}})
	guard := New(organizations)

	err := guard.RequireProjectAccess(context.Background(), userID, projectmodel.Project{ID: uuid.New(), OrganizationID: organizationID}, orgguard.ReadOrganization)
	require.NoError(t, err)
	err = guard.RequireProjectAccess(context.Background(), userID, projectmodel.Project{}, orgguard.ReadOrganization)
	require.ErrorIs(t, err, apperror.ErrNotFound)
}
