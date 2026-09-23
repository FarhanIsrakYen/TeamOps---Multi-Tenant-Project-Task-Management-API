package service

import (
	"context"
	"sync"
	"testing"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectguard "github.com/example/teamops/backend/internal/modules/projects/guard"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	taskguard "github.com/example/teamops/backend/internal/modules/tasks/guard"
	taskmodel "github.com/example/teamops/backend/internal/modules/tasks/model"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type taskMemberships struct {
	roles map[[2]uuid.UUID]orgmodel.Role
}

func (m taskMemberships) Role(_ context.Context, organizationID, userID uuid.UUID) (orgmodel.Role, error) {
	role, ok := m.roles[[2]uuid.UUID{organizationID, userID}]
	if !ok {
		return "", pgx.ErrNoRows
	}
	return role, nil
}

type taskRepository struct {
	mu           sync.Mutex
	task         taskmodel.Task
	deleted      bool
	commented    bool
	updated      bool
	setLabelsErr error
}

func (r *taskRepository) Create(_ context.Context, task taskmodel.Task) (taskmodel.Task, error) {
	task.ID = uuid.New()
	return task, nil
}
func (r *taskRepository) Get(_ context.Context, id uuid.UUID) (taskmodel.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task.ID != id {
		return taskmodel.Task{}, pgx.ErrNoRows
	}
	return r.task, nil
}
func (r *taskRepository) List(context.Context, uuid.UUID, pagination.Params, taskmodel.Filters) ([]taskmodel.Task, int64, error) {
	return nil, 0, nil
}
func (r *taskRepository) Update(_ context.Context, task taskmodel.Task) (taskmodel.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.Version != r.task.Version {
		return taskmodel.Task{}, pgx.ErrNoRows
	}
	r.updated = true
	task.Version++
	r.task = task
	return task, nil
}
func (r *taskRepository) Delete(context.Context, uuid.UUID) error {
	r.deleted = true
	return nil
}
func (r *taskRepository) AddComment(_ context.Context, taskID, authorID uuid.UUID, body string) (taskmodel.Comment, error) {
	r.commented = true
	return taskmodel.Comment{ID: uuid.New(), TaskID: taskID, AuthorID: authorID, Body: body}, nil
}
func (r *taskRepository) Comments(context.Context, uuid.UUID) ([]taskmodel.Comment, error) {
	return nil, nil
}
func (r *taskRepository) CreateLabel(_ context.Context, organizationID uuid.UUID, name, color string) (taskmodel.Label, error) {
	return taskmodel.Label{ID: uuid.New(), OrganizationID: organizationID, Name: name, Color: color}, nil
}
func (r *taskRepository) Labels(context.Context, uuid.UUID) ([]taskmodel.Label, error) {
	return nil, nil
}
func (r *taskRepository) SetLabels(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) error {
	return r.setLabelsErr
}

type taskProjects struct{ project projectmodel.Project }

func (p taskProjects) Get(_ context.Context, id uuid.UUID) (projectmodel.Project, error) {
	if p.project.ID != id {
		return projectmodel.Project{}, pgx.ErrNoRows
	}
	return p.project, nil
}

type taskAuditor struct{}

func (taskAuditor) Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error {
	return nil
}

func taskServiceForRole(role orgmodel.Role) (*Service, *taskRepository, uuid.UUID, uuid.UUID) {
	organizationID := uuid.New()
	userID := uuid.New()
	taskID := uuid.New()
	projectID := uuid.New()
	repository := &taskRepository{task: taskmodel.Task{ID: taskID, OrganizationID: organizationID, ProjectID: projectID, Title: "Task", Status: taskmodel.StatusTODO, Priority: taskmodel.PriorityMedium, Version: 1}}
	organizations := orgguard.New(taskMemberships{roles: map[[2]uuid.UUID]orgmodel.Role{{organizationID, userID}: role}})
	projects := taskProjects{project: projectmodel.Project{ID: projectID, OrganizationID: organizationID}}
	service := New(repository, repository, repository, projects, organizations, projectguard.New(organizations), taskguard.New(organizations), taskAuditor{})
	return service, repository, userID, taskID
}

func TestAllRolesCanReadTasks(t *testing.T) {
	t.Parallel()
	for _, role := range []orgmodel.Role{orgmodel.RoleOwner, orgmodel.RoleAdmin, orgmodel.RoleMember, orgmodel.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			service, _, userID, taskID := taskServiceForRole(role)
			_, err := service.Get(context.Background(), userID, taskID)
			require.NoError(t, err)
		})
	}
}

func TestMemberCanUpdateAndCommentButCannotDeleteTask(t *testing.T) {
	t.Parallel()
	service, repository, userID, taskID := taskServiceForRole(orgmodel.RoleMember)
	title := "Updated task"
	_, err := service.Update(context.Background(), userID, taskID, UpdateInput{Title: &title, Version: 1}, "request")
	require.NoError(t, err)
	require.True(t, repository.updated)
	_, err = service.AddComment(context.Background(), userID, taskID, "comment", "request")
	require.NoError(t, err)
	require.True(t, repository.commented)
	require.ErrorIs(t, service.Delete(context.Background(), userID, taskID, "request"), apperror.ErrForbidden)
	require.False(t, repository.deleted)
}

func TestViewerIsReadOnlyAndAdminCanDeleteTask(t *testing.T) {
	t.Parallel()
	viewerService, viewerRepository, viewerID, viewerTaskID := taskServiceForRole(orgmodel.RoleViewer)
	_, err := viewerService.Update(context.Background(), viewerID, viewerTaskID, UpdateInput{Version: 1}, "request")
	require.ErrorIs(t, err, apperror.ErrForbidden)
	_, err = viewerService.AddComment(context.Background(), viewerID, viewerTaskID, "comment", "request")
	require.ErrorIs(t, err, apperror.ErrForbidden)
	require.False(t, viewerRepository.updated)
	require.False(t, viewerRepository.commented)

	adminService, adminRepository, adminID, adminTaskID := taskServiceForRole(orgmodel.RoleAdmin)
	require.NoError(t, adminService.Delete(context.Background(), adminID, adminTaskID, "request"))
	require.True(t, adminRepository.deleted)
}

func TestTaskAccessRejectsNonMemberAndMissingResource(t *testing.T) {
	t.Parallel()
	service, _, _, taskID := taskServiceForRole(orgmodel.RoleOwner)
	_, err := service.Get(context.Background(), uuid.New(), taskID)
	require.ErrorIs(t, err, apperror.ErrNotFound)
	_, err = service.Get(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, apperror.ErrNotFound)
}
