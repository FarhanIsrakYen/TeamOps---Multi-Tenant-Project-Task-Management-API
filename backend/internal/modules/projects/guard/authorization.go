package guard

import (
	"context"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
)

type OrganizationAuthorizer interface {
	RequireOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, orgguard.Permission) (orgmodel.Role, error)
}

type Guard struct{ organizations OrganizationAuthorizer }

func New(organizations OrganizationAuthorizer) *Guard {
	return &Guard{organizations: organizations}
}

// RequireProjectAccess authorizes a project that was loaded from trusted
// persistence. Callers cannot supply the organization boundary independently.
func (g *Guard) RequireProjectAccess(ctx context.Context, userID uuid.UUID, project projectmodel.Project, permission orgguard.Permission) error {
	if project.ID == uuid.Nil || project.OrganizationID == uuid.Nil {
		return apperror.ErrNotFound
	}
	_, err := g.organizations.RequireOrganizationPermission(ctx, userID, project.OrganizationID, permission)
	return err
}
