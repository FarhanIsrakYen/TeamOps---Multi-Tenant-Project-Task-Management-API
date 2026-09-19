package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/example/teamops/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

const (
	SessionCleanupJobName           = "session_cleanup"
	StaleTaskDetectionJobName       = "stale_task_detection"
	ProjectStatisticsRefreshJobName = "project_statistics_refresh"
)

type SessionCleaner interface {
	DeleteExpiredSessions(context.Context) (int64, error)
}

type MaintenanceStore interface {
	MarkStaleTasks(context.Context, time.Time) (int64, error)
	RefreshProjectStatistics(context.Context) (int64, error)
}

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) MarkStaleTasks(ctx context.Context, staleBefore time.Time) (int64, error) {
	tag, err := database.Executor(ctx, r.db).Exec(ctx, `
		UPDATE tasks
		SET is_stale=true, stale_detected_at=COALESCE(stale_detected_at, now())
		WHERE status NOT IN ('DONE','CANCELLED')
		  AND updated_at < $1
		  AND is_stale=false`, staleBefore)
	if err != nil {
		return 0, fmt.Errorf("mark stale tasks: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) RefreshProjectStatistics(ctx context.Context) (int64, error) {
	tag, err := database.Executor(ctx, r.db).Exec(ctx, `
		WITH statistics AS (
			SELECT p.id,
			       count(t.id)::integer AS task_count,
			       count(t.id) FILTER (WHERE t.status='DONE')::integer AS completed_task_count,
			       count(t.id) FILTER (WHERE t.due_date < now() AND t.status NOT IN ('DONE','CANCELLED'))::integer AS overdue_task_count
			FROM projects p
			LEFT JOIN tasks t ON t.project_id=p.id
			GROUP BY p.id
		)
		UPDATE projects p
		SET task_count=s.task_count,
		    completed_task_count=s.completed_task_count,
		    overdue_task_count=s.overdue_task_count,
		    statistics_refreshed_at=now()
		FROM statistics s
		WHERE p.id=s.id`)
	if err != nil {
		return 0, fmt.Errorf("refresh project statistics: %w", err)
	}
	return tag.RowsAffected(), nil
}

func SessionCleanupJob(repo SessionCleaner, log zerolog.Logger) Job {
	return JobFunc{JobName: SessionCleanupJobName, Handler: func(ctx context.Context) error {
		count, err := repo.DeleteExpiredSessions(ctx)
		if err == nil && count > 0 {
			log.Info().Int64("deleted", count).Msg("expired sessions cleaned")
		}
		return err
	}}
}

func StaleTaskDetectionJob(store MaintenanceStore, staleAfter time.Duration, now func() time.Time, log zerolog.Logger) Job {
	return JobFunc{JobName: StaleTaskDetectionJobName, Handler: func(ctx context.Context) error {
		count, err := store.MarkStaleTasks(ctx, now().Add(-staleAfter))
		if err == nil && count > 0 {
			log.Info().Int64("tasks_marked", count).Msg("stale task detection completed")
		}
		return err
	}}
}

func ProjectStatisticsRefreshJob(store MaintenanceStore, log zerolog.Logger) Job {
	return JobFunc{JobName: ProjectStatisticsRefreshJobName, Handler: func(ctx context.Context) error {
		count, err := store.RefreshProjectStatistics(ctx)
		if err == nil {
			log.Debug().Int64("projects_refreshed", count).Msg("project statistics refreshed")
		}
		return err
	}}
}
