package guard

import (
	"context"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	taskmodel "github.com/example/teamops/backend/internal/modules/tasks/model"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
)

type OrganizationAuthorizer interface {
	RequireOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, orgguard.Permission) (orgmodel.Role, error)
}

type Guard struct{ organizations OrganizationAuthorizer }

func New(organizations OrganizationAuthorizer) *Guard {
	return &Guard{organizations: organizations}
}

// RequireTaskAccess derives the tenant from the persisted task, preventing a
// caller from pairing a task ID with an organization they control.
func (g *Guard) RequireTaskAccess(ctx context.Context, userID uuid.UUID, task taskmodel.Task, permission orgguard.Permission) error {
	if task.ID == uuid.Nil || task.OrganizationID == uuid.Nil {
		return apperror.ErrNotFound
	}
	_, err := g.organizations.RequireOrganizationPermission(ctx, userID, task.OrganizationID, permission)
	return err
}
