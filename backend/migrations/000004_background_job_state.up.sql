ALTER TABLE tasks
  ADD COLUMN is_stale boolean NOT NULL DEFAULT false,
  ADD COLUMN stale_detected_at timestamptz;

CREATE INDEX tasks_stale_detection_idx
  ON tasks(updated_at)
  WHERE status NOT IN ('DONE', 'CANCELLED') AND is_stale = false;

ALTER TABLE projects
  ADD COLUMN task_count integer NOT NULL DEFAULT 0,
  ADD COLUMN completed_task_count integer NOT NULL DEFAULT 0,
  ADD COLUMN overdue_task_count integer NOT NULL DEFAULT 0,
  ADD COLUMN statistics_refreshed_at timestamptz,
  ADD CONSTRAINT projects_task_count_nonnegative CHECK (task_count >= 0),
  ADD CONSTRAINT projects_completed_task_count_nonnegative CHECK (completed_task_count >= 0),
  ADD CONSTRAINT projects_overdue_task_count_nonnegative CHECK (overdue_task_count >= 0);
