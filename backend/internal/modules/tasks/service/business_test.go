package service

import (
	"context"
	"sync"
	"testing"
	"time"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectguard "github.com/example/teamops/backend/internal/modules/projects/guard"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	taskguard "github.com/example/teamops/backend/internal/modules/tasks/guard"
	taskmodel "github.com/example/teamops/backend/internal/modules/tasks/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type recordingTaskAuditor struct{ actions []string }

func (a *recordingTaskAuditor) Record(_ context.Context, _, _ uuid.UUID, action, _, _, _ string, _ map[string]any) error {
	a.actions = append(a.actions, action)
	return nil
}

func TestUpdateRejectsInvalidStatusTransition(t *testing.T) {
	t.Parallel()
	service, repository, userID, taskID := taskServiceForRole("MEMBER")
	next := taskmodel.StatusDone
	_, err := service.Update(context.Background(), userID, taskID, UpdateInput{Status: &next, Version: 1}, "request")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 409, appErr.Status)
	require.Equal(t, "invalid_status_transition", appErr.Code)
	require.False(t, repository.updated)
}

func TestCreateRequiresTodoInitialStatus(t *testing.T) {
	t.Parallel()
	service, repository, userID, _ := taskServiceForRole("MEMBER")
	_, err := service.Create(context.Background(), userID, repository.task.ProjectID, taskmodel.Task{Title: "Already done", Status: taskmodel.StatusDone, Priority: taskmodel.PriorityMedium}, "request")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.Status)
	require.Equal(t, "invalid_initial_status", appErr.Code)
}

func TestTaskAssignmentPriorityStatusAndDueDatePatch(t *testing.T) {
	t.Parallel()
	service, repository, userID, taskID := taskServiceForRole("MEMBER")
	dueAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	status := taskmodel.StatusInProgress
	priority := taskmodel.PriorityUrgent
	updated, err := service.Update(context.Background(), userID, taskID, UpdateInput{
		AssigneeSet: true,
		AssigneeID:  &userID,
		Status:      &status,
		Priority:    &priority,
		DueAtSet:    true,
		DueAt:       &dueAt,
		Version:     1,
	}, "request")
	require.NoError(t, err)
	require.Equal(t, &userID, updated.AssigneeID)
	require.Equal(t, taskmodel.StatusInProgress, updated.Status)
	require.Equal(t, taskmodel.PriorityUrgent, updated.Priority)
	require.Equal(t, &dueAt, updated.DueAt)

	title := "Still assigned"
	updated, err = service.Update(context.Background(), userID, taskID, UpdateInput{
		Title:       &title,
		AssigneeSet: true,
		AssigneeID:  nil,
		DueAtSet:    true,
		DueAt:       nil,
		Version:     2,
	}, "request")
	require.NoError(t, err)
	require.Nil(t, updated.AssigneeID)
	require.Nil(t, updated.DueAt)
	require.Equal(t, 3, repository.task.Version)
}

func TestTaskAssignmentRejectsNonMember(t *testing.T) {
	t.Parallel()
	service, repository, userID, taskID := taskServiceForRole("MEMBER")
	nonMemberID := uuid.New()
	_, err := service.Update(context.Background(), userID, taskID, UpdateInput{AssigneeSet: true, AssigneeID: &nonMemberID, Version: 1}, "request")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "invalid_assignee", appErr.Code)
	require.False(t, repository.updated)
}

func TestConcurrentTaskUpdatesRejectStaleVersion(t *testing.T) {
	t.Parallel()
	service, repository, userID, taskID := taskServiceForRole("MEMBER")
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for _, title := range []string{"First update", "Second update"} {
		title := title
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := service.Update(context.Background(), userID, taskID, UpdateInput{Title: &title, Version: 1}, "request")
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
	require.Equal(t, 2, repository.task.Version)
}

func TestCommentAndLabelValidationIsEnforcedInService(t *testing.T) {
	t.Parallel()
	memberService, repository, memberID, taskID := taskServiceForRole("MEMBER")
	_, err := memberService.AddComment(context.Background(), memberID, taskID, "   ", "request")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "invalid_comment", appErr.Code)
	require.False(t, repository.commented)

	adminService, adminRepository, adminID, _ := taskServiceForRole("ADMIN")
	_, err = adminService.CreateLabel(context.Background(), adminID, adminRepository.task.OrganizationID, "Security", "not-a-color", "request")
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "invalid_label", appErr.Code)

	labelID := uuid.New()
	err = memberService.SetLabels(context.Background(), memberID, taskID, []uuid.UUID{labelID, labelID}, "request")
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "duplicate_label", appErr.Code)
}

func TestAssignmentAndStatusChangesEmitGranularAuditEvents(t *testing.T) {
	t.Parallel()
	organizationID, userID, taskID, projectID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repository := &taskRepository{task: taskmodel.Task{ID: taskID, OrganizationID: organizationID, ProjectID: projectID, Title: "Task", Status: taskmodel.StatusTODO, Priority: taskmodel.PriorityMedium, Version: 1}}
	organizations := orgguard.New(taskMemberships{roles: map[[2]uuid.UUID]orgmodel.Role{{organizationID, userID}: orgmodel.RoleMember}}, nil, 0)
	projects := taskProjects{project: projectmodel.Project{ID: projectID, OrganizationID: organizationID}}
	auditor := &recordingTaskAuditor{}
	service := New(repository, repository, repository, projects, organizations, projectguard.New(organizations), taskguard.New(organizations), auditor)
	status := taskmodel.StatusInProgress

	_, err := service.Update(context.Background(), userID, taskID, UpdateInput{AssigneeSet: true, AssigneeID: &userID, Status: &status, Version: 1}, "request")
	require.NoError(t, err)
	require.Equal(t, []string{"task.updated", "task.assignment_changed", "task.status_changed"}, auditor.actions)
}
