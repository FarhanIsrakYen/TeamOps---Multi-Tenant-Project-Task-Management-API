//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/database"
	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	auditrepo "github.com/example/teamops/backend/internal/modules/audit/repository"
	orgrepo "github.com/example/teamops/backend/internal/modules/organizations/repository"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	projectrepo "github.com/example/teamops/backend/internal/modules/projects/repository"
	taskmodel "github.com/example/teamops/backend/internal/modules/tasks/model"
	taskrepo "github.com/example/teamops/backend/internal/modules/tasks/repository"
	usersrepo "github.com/example/teamops/backend/internal/modules/users/repository"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestOrganizationCreationIsTransactionalAndTenantScoped(t *testing.T) {
	ctx := context.Background()
	pool := resetPostgres(t)
	users := usersrepo.New(pool)
	orgs := orgrepo.New(pool)
	tx := database.NewTransactor(pool)
	rollbackErr := errors.New("force rollback")
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		rolledBackUser, createErr := users.Create(txCtx, "rollback@example.com", "Rollback", "hash")
		if createErr != nil {
			return createErr
		}
		if _, createErr = orgs.Create(txCtx, rolledBackUser.ID, "Rolled Back", "rolled-back"); createErr != nil {
			return createErr
		}
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)
	_, err = users.ByEmail(ctx, "rollback@example.com")
	require.ErrorIs(t, err, pgx.ErrNoRows)

	user, err := users.Create(ctx, "owner@example.com", "Owner", "hash")
	require.NoError(t, err)
	created, err := orgs.Create(ctx, user.ID, "Acme", "acme")
	require.NoError(t, err)
	require.Equal(t, "Acme", created.Name)
	items, total, err := orgs.ListForUser(ctx, user.ID, pagination.Params{Page: 1, PageSize: 20, SortBy: "createdAt", Order: "desc"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "OWNER", string(items[0].Role))

	projects := projectrepo.New(pool)
	project, err := projects.Create(ctx, created.ID, "Roadmap", "")
	require.NoError(t, err)
	projectErrors := runConcurrentUpdates(func(name string) error {
		_, updateErr := projects.Update(ctx, project.ID, name, "", false, project.Version)
		return updateErr
	})
	requireConcurrentOptimisticLock(t, projectErrors)
	projectPage, projectTotal, err := projects.List(ctx, created.ID, pagination.Params{Page: 2, PageSize: 20, Offset: 20, SortBy: "created_at", Order: "desc"}, projectmodel.Filters{})
	require.NoError(t, err)
	require.Empty(t, projectPage)
	require.EqualValues(t, 1, projectTotal)

	tasks := taskrepo.New(pool)
	task, err := tasks.Create(ctx, taskmodel.Task{OrganizationID: created.ID, ProjectID: project.ID, CreatedBy: user.ID, Title: "Ship feature", Status: taskmodel.StatusTODO, Priority: taskmodel.PriorityHigh})
	require.NoError(t, err)
	taskErrors := runConcurrentUpdates(func(title string) error {
		candidate := task
		candidate.Title = title
		_, updateErr := tasks.Update(ctx, candidate)
		return updateErr
	})
	requireConcurrentOptimisticLock(t, taskErrors)
	taskPage, taskTotal, err := tasks.List(ctx, project.ID, pagination.Params{Page: 2, PageSize: 20, Offset: 20, SortBy: "created_at", Order: "desc"}, taskmodel.Filters{Status: taskmodel.StatusTODO, Priority: taskmodel.PriorityHigh})
	require.NoError(t, err)
	require.Empty(t, taskPage)
	require.EqualValues(t, 1, taskTotal)

	audits := auditrepo.New(pool)
	organizationID, actorID := created.ID, user.ID
	require.NoError(t, audits.Insert(ctx, auditmodel.Entry{
		OrganizationID: &organizationID,
		ActorUserID:    &actorID,
		Action:         "task.status_changed",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Metadata:       map[string]any{"oldStatus": "TODO", "newStatus": "IN_PROGRESS"},
		RequestID:      "integration-request",
		IPAddress:      "192.0.2.10",
		UserAgent:      "integration-test",
	}))
	from := time.Now().Add(-time.Minute)
	logs, auditTotal, err := audits.List(ctx, created.ID, pagination.Params{Page: 1, PageSize: 20}, auditmodel.Filters{
		ActorUserID: &actorID, Action: "task.status_changed", ResourceType: "task", ResourceID: task.ID.String(), From: &from,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, auditTotal)
	require.Len(t, logs, 1)
	require.Equal(t, "192.0.2.10", logs[0].IPAddress)
	require.Equal(t, "integration-test", logs[0].UserAgent)
	require.Equal(t, "IN_PROGRESS", logs[0].Metadata["newStatus"])
}

func runConcurrentUpdates(update func(string) error) []error {
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for _, value := range []string{"concurrent-a", "concurrent-b"} {
		value := value
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errorsChannel <- update(value)
		}()
	}
	close(start)
	wait.Wait()
	close(errorsChannel)
	out := make([]error, 0, 2)
	for err := range errorsChannel {
		out = append(out, err)
	}
	return out
}

func requireConcurrentOptimisticLock(t *testing.T, updateErrors []error) {
	t.Helper()
	var successes, conflicts int
	for _, err := range updateErrors {
		if err == nil {
			successes++
		} else if errors.Is(err, pgx.ErrNoRows) {
			conflicts++
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}
