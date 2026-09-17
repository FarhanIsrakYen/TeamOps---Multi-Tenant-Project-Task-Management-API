package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitialMigrationContainsDomainTables(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("000001_initial.up.sql")
	require.NoError(t, err)
	sql := string(raw)

	for _, table := range []string{
		"users",
		"organizations",
		"organization_members",
		"projects",
		"tasks",
		"task_comments",
		"task_labels",
		"task_label_assignments",
		"refresh_tokens",
		"audit_logs",
	} {
		require.Contains(t, sql, "CREATE TABLE "+table+" (", "missing table %s", table)
	}

	for _, legacyName := range []string{"organization_memberships", "refresh_sessions", "CREATE TABLE labels ("} {
		require.False(t, strings.Contains(sql, legacyName), "legacy table name remains: %s", legacyName)
	}
}
