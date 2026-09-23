CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE refresh_tokens
  ADD CONSTRAINT refresh_tokens_user_agent_length CHECK (length(user_agent) <= 512);

-- Repair legacy rows before making organization membership a database-level
-- assignment invariant. New writes cannot race a membership removal.
UPDATE tasks t
SET assignee_id = NULL
WHERE assignee_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM organization_members m
    WHERE m.organization_id = t.organization_id
      AND m.user_id = t.assignee_id
  );

ALTER TABLE tasks
  ADD CONSTRAINT tasks_assignee_membership_fk
  FOREIGN KEY (organization_id, assignee_id)
  REFERENCES organization_members(organization_id, user_id)
  ON DELETE SET NULL (assignee_id);

CREATE INDEX projects_name_search_trgm_idx
  ON projects USING gin (name gin_trgm_ops);
CREATE INDEX projects_description_search_trgm_idx
  ON projects USING gin (description gin_trgm_ops);
CREATE INDEX tasks_title_search_trgm_idx
  ON tasks USING gin (title gin_trgm_ops);
CREATE INDEX tasks_description_search_trgm_idx
  ON tasks USING gin (description gin_trgm_ops);

DROP INDEX audit_logs_actor_user_lookup_idx;
CREATE INDEX audit_logs_actor_user_lookup_idx
  ON audit_logs(organization_id, actor_user_id, created_at DESC, id DESC)
  WHERE actor_user_id IS NOT NULL;

DROP INDEX audit_logs_resource_lookup_idx;
CREATE INDEX audit_logs_resource_lookup_idx
  ON audit_logs(organization_id, resource_type, resource_id, created_at DESC, id DESC);
