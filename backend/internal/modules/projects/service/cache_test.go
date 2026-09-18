package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type projectCacheStore struct {
	mu        sync.Mutex
	values    map[string][]byte
	getErr    error
	setErr    error
	deleteErr error
	sets      int
	deletes   int
}

func (s *projectCacheStore) GetJSON(_ context.Context, key string, dst any) (bool, error) {
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

func (s *projectCacheStore) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets++
	if s.setErr != nil {
		return s.setErr
	}
	raw, err := json.Marshal(value)
	if err == nil {
		s.values[key] = raw
	}
	return err
}

func (s *projectCacheStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for _, key := range keys {
		delete(s.values, key)
	}
	return nil
}

type cacheProjectRepository struct {
	project   projectmodel.Project
	getCalls  int
	updates   int
	deletions int
}

func (r *cacheProjectRepository) Create(context.Context, uuid.UUID, string, string) (projectmodel.Project, error) {
	return projectmodel.Project{}, nil
}
func (r *cacheProjectRepository) Get(context.Context, uuid.UUID) (projectmodel.Project, error) {
	r.getCalls++
	return r.project, nil
}
func (r *cacheProjectRepository) List(context.Context, uuid.UUID, pagination.Params, projectmodel.Filters) ([]projectmodel.Project, int64, error) {
	return nil, 0, nil
}
func (r *cacheProjectRepository) Update(_ context.Context, _ uuid.UUID, name, description string, archived bool, version int) (projectmodel.Project, error) {
	r.updates++
	r.project.Name = name
	r.project.Description = description
	r.project.Archived = archived
	r.project.Version = version + 1
	return r.project, nil
}
func (r *cacheProjectRepository) Delete(context.Context, uuid.UUID) error {
	r.deletions++
	return nil
}

type allowProjectAccess struct{}

func (allowProjectAccess) RequireProjectAccess(context.Context, uuid.UUID, projectmodel.Project, orgguard.Permission) error {
	return nil
}

type unusedOrganizationAuthorizer struct{}

func (unusedOrganizationAuthorizer) RequireOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, orgguard.Permission) (orgmodel.Role, error) {
	return orgmodel.RoleAdmin, nil
}

func TestProjectCacheAsideHitMissAndInvalidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	projectID := uuid.New()
	repository := &cacheProjectRepository{project: projectmodel.Project{
		ID: projectID, OrganizationID: uuid.New(), Name: "Cached project", Version: 1,
	}}
	store := &projectCacheStore{values: make(map[string][]byte)}
	service := New(repository, unusedOrganizationAuthorizer{}, allowProjectAccess{}, store, time.Minute, projectAuditor{})

	for range 2 {
		project, err := service.Get(ctx, uuid.New(), projectID)
		require.NoError(t, err)
		require.Equal(t, "Cached project", project.Name)
	}
	require.Equal(t, 1, repository.getCalls)
	require.Equal(t, 1, store.sets)

	name := "Updated project"
	_, err := service.Update(ctx, uuid.New(), projectID, UpdateInput{Name: &name, Version: 1}, "request")
	require.NoError(t, err)
	require.Equal(t, 1, store.deletes)
	_, exists := store.values[cache.ProjectKey(projectID.String())]
	require.False(t, exists)

	_, err = service.Get(ctx, uuid.New(), projectID)
	require.NoError(t, err)
	require.Equal(t, 3, repository.getCalls, "update loads once and invalidation makes the next read miss")

	require.NoError(t, service.Delete(ctx, uuid.New(), projectID, "request"))
	require.Equal(t, 2, store.deletes)
}

func TestProjectCacheFailureDoesNotFailReadsOrWrites(t *testing.T) {
	t.Parallel()
	projectID := uuid.New()
	repository := &cacheProjectRepository{project: projectmodel.Project{
		ID: projectID, OrganizationID: uuid.New(), Name: "Available", Version: 1,
	}}
	store := &projectCacheStore{
		values: make(map[string][]byte), getErr: errors.New("redis unavailable"),
		setErr: errors.New("redis unavailable"), deleteErr: errors.New("redis unavailable"),
	}
	service := New(repository, unusedOrganizationAuthorizer{}, allowProjectAccess{}, store, time.Minute, projectAuditor{})

	project, err := service.Get(context.Background(), uuid.New(), projectID)
	require.NoError(t, err)
	require.Equal(t, "Available", project.Name)

	name := "Still available"
	_, err = service.Update(context.Background(), uuid.New(), projectID, UpdateInput{Name: &name, Version: 1}, "request")
	require.NoError(t, err)
}
