package service

import (
	"context"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectguard "github.com/example/teamops/backend/internal/modules/projects/guard"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type projectMemberships struct{ roles map[uuid.UUID]orgmodel.Role }

func (m projectMemberships) Role(_ context.Context, _ uuid.UUID, userID uuid.UUID) (orgmodel.Role, error) {
	role, ok := m.roles[userID]
	if !ok {
		return "", pgx.ErrNoRows
	}
	return role, nil
}

type projectRepository struct{ creates int }

func (r *projectRepository) Create(_ context.Context, organizationID uuid.UUID, name, description string) (projectmodel.Project, error) {
	r.creates++
	return projectmodel.Project{ID: uuid.New(), OrganizationID: organizationID, Name: name, Description: description}, nil
}
func (r *projectRepository) Get(context.Context, uuid.UUID) (projectmodel.Project, error) {
	return projectmodel.Project{}, pgx.ErrNoRows
}
func (r *projectRepository) List(context.Context, uuid.UUID, pagination.Params, projectmodel.Filters) ([]projectmodel.Project, int64, error) {
	return nil, 0, nil
}
func (r *projectRepository) Update(context.Context, uuid.UUID, string, string, bool, int) (projectmodel.Project, error) {
	return projectmodel.Project{}, nil
}
func (r *projectRepository) Delete(context.Context, uuid.UUID) error { return nil }

type projectAuditor struct{}

func (projectAuditor) Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error {
	return nil
}

func TestOnlyOwnerAndAdminCanManageProjects(t *testing.T) {
	t.Parallel()
	organizationID := uuid.New()
	tests := []struct {
		name    string
		role    orgmodel.Role
		allowed bool
	}{
		{name: "owner", role: orgmodel.RoleOwner, allowed: true},
		{name: "admin", role: orgmodel.RoleAdmin, allowed: true},
		{name: "member", role: orgmodel.RoleMember},
		{name: "viewer", role: orgmodel.RoleViewer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()
			repository := &projectRepository{}
			organizations := orgguard.New(projectMemberships{roles: map[uuid.UUID]orgmodel.Role{userID: tt.role}})
			access := projectguard.New(organizations)
			projectCache := cache.New("127.0.0.1:0", "")
			t.Cleanup(func() { _ = projectCache.Close() })
			service := New(repository, organizations, access, projectCache, time.Minute, projectAuditor{})

			_, err := service.Create(context.Background(), userID, organizationID, "Project", "", "request")
			if tt.allowed {
				require.NoError(t, err)
				require.Equal(t, 1, repository.creates)
			} else {
				require.ErrorIs(t, err, apperror.ErrForbidden)
				require.Zero(t, repository.creates)
			}
		})
	}
}
