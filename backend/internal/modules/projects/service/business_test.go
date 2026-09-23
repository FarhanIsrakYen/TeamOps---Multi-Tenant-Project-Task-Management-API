package service

import (
	"context"
	"sync"
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

type lockingProjectRepository struct {
	mu      sync.Mutex
	project projectmodel.Project
}

func (r *lockingProjectRepository) Create(context.Context, uuid.UUID, string, string) (projectmodel.Project, error) {
	return projectmodel.Project{}, nil
}
func (r *lockingProjectRepository) Get(_ context.Context, id uuid.UUID) (projectmodel.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.project.ID != id {
		return projectmodel.Project{}, pgx.ErrNoRows
	}
	return r.project, nil
}
func (r *lockingProjectRepository) List(context.Context, uuid.UUID, pagination.Params, projectmodel.Filters) ([]projectmodel.Project, int64, error) {
	return nil, 0, nil
}
func (r *lockingProjectRepository) Update(_ context.Context, id uuid.UUID, name, description string, archived bool, version int) (projectmodel.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.project.ID != id || r.project.Version != version {
		return projectmodel.Project{}, pgx.ErrNoRows
	}
	r.project.Name = name
	r.project.Description = description
	r.project.Archived = archived
	r.project.Version++
	return r.project, nil
}
func (r *lockingProjectRepository) Delete(context.Context, uuid.UUID) error { return nil }

func TestConcurrentProjectUpdatesRejectStaleVersion(t *testing.T) {
	t.Parallel()
	organizationID := uuid.New()
	userID := uuid.New()
	projectID := uuid.New()
	repository := &lockingProjectRepository{project: projectmodel.Project{ID: projectID, OrganizationID: organizationID, Name: "Original", Description: "Preserved", Version: 1}}
	organizations := orgguard.New(projectMemberships{roles: map[uuid.UUID]orgmodel.Role{userID: orgmodel.RoleAdmin}})
	projectCache := cache.New("127.0.0.1:0", "")
	t.Cleanup(func() { _ = projectCache.Close() })
	service := New(repository, organizations, projectguard.New(organizations), projectCache, time.Minute, projectAuditor{})

	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for _, name := range []string{"First name", "Second name"} {
		name := name
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := service.Update(context.Background(), userID, projectID, UpdateInput{Name: &name, Version: 1}, "request")
			errorsChannel <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsChannel)

	var successes, conflicts int
	for err := range errorsChannel {
		if err == nil {
			successes++
			continue
		}
		var appErr *apperror.Error
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 409, appErr.Status)
		require.Equal(t, "version_conflict", appErr.Code)
		conflicts++
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	require.Equal(t, 2, repository.project.Version)
	require.Equal(t, "Preserved", repository.project.Description)
}
