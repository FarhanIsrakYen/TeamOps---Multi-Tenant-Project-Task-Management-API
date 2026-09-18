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

func TestTaskReviewStatusMigrationIsReversible(t *testing.T) {
	t.Parallel()
	up, err := os.ReadFile("000002_task_in_review.up.sql")
	require.NoError(t, err)
	require.Contains(t, string(up), "'IN_REVIEW'")
	down, err := os.ReadFile("000002_task_in_review.down.sql")
	require.NoError(t, err)
	require.Contains(t, string(down), "UPDATE tasks SET status = 'IN_PROGRESS'")
	require.Contains(t, string(down), "CREATE TYPE task_status AS ENUM")
}

func TestAuditContextMigrationIsReversible(t *testing.T) {
	t.Parallel()
	up, err := os.ReadFile("000003_audit_log_context.up.sql")
	require.NoError(t, err)
	upSQL := string(up)
	for _, fragment := range []string{"actor_user_id", "ip_address inet", "user_agent text", "DROP CONSTRAINT audit_logs_organization_id_fkey", "audit_logs_action_lookup_idx"} {
		require.Contains(t, upSQL, fragment)
	}

	down, err := os.ReadFile("000003_audit_log_context.down.sql")
	require.NoError(t, err)
	require.Contains(t, string(down), "RENAME COLUMN actor_user_id TO actor_id")
	require.Contains(t, string(down), "ON DELETE CASCADE")
}
