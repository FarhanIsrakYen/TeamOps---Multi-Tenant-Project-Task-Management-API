//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/teamops/backend/internal/database"
	orgrepo "github.com/example/teamops/backend/internal/modules/organizations/repository"
	usersrepo "github.com/example/teamops/backend/internal/modules/users/repository"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestOrganizationCreationIsTransactionalAndTenantScoped(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url, 5)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`, pgx.QueryExecModeSimpleProtocol)
	require.NoError(t, err)
	migrationPath := filepath.Join("..", "..", "migrations", "000001_initial.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration), pgx.QueryExecModeSimpleProtocol)
	require.NoError(t, err)
	users := usersrepo.New(pool)
	orgs := orgrepo.New(pool)
	tx := database.NewTransactor(pool)
	rollbackErr := errors.New("force rollback")
	err = tx.WithinTransaction(ctx, func(txCtx context.Context) error {
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
}
