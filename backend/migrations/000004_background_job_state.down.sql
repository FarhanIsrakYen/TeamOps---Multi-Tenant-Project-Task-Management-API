ALTER TABLE projects
  DROP CONSTRAINT projects_overdue_task_count_nonnegative,
  DROP CONSTRAINT projects_completed_task_count_nonnegative,
  DROP CONSTRAINT projects_task_count_nonnegative,
  DROP COLUMN statistics_refreshed_at,
  DROP COLUMN overdue_task_count,
  DROP COLUMN completed_task_count,
  DROP COLUMN task_count;

DROP INDEX tasks_stale_detection_idx;

ALTER TABLE tasks
  DROP COLUMN stale_detected_at,
  DROP COLUMN is_stale;
