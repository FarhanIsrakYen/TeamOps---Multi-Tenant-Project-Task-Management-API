UPDATE tasks SET status = 'IN_PROGRESS' WHERE status = 'IN_REVIEW';

ALTER TABLE tasks ALTER COLUMN status DROP DEFAULT;
ALTER TYPE task_status RENAME TO task_status_with_review;
CREATE TYPE task_status AS ENUM ('TODO', 'IN_PROGRESS', 'DONE', 'CANCELLED');
ALTER TABLE tasks
  ALTER COLUMN status TYPE task_status
  USING status::text::task_status;
ALTER TABLE tasks ALTER COLUMN status SET DEFAULT 'TODO';
DROP TYPE task_status_with_review;
