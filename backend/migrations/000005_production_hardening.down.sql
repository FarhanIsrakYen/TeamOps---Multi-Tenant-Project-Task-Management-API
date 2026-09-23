DROP INDEX audit_logs_resource_lookup_idx;
CREATE INDEX audit_logs_resource_lookup_idx
  ON audit_logs(organization_id, resource_type, resource_id);

DROP INDEX audit_logs_actor_user_lookup_idx;
CREATE INDEX audit_logs_actor_user_lookup_idx
  ON audit_logs(actor_user_id, created_at DESC)
  WHERE actor_user_id IS NOT NULL;

DROP INDEX tasks_description_search_trgm_idx;
DROP INDEX tasks_title_search_trgm_idx;
DROP INDEX projects_description_search_trgm_idx;
DROP INDEX projects_name_search_trgm_idx;

ALTER TABLE tasks DROP CONSTRAINT tasks_assignee_membership_fk;

ALTER TABLE refresh_tokens DROP CONSTRAINT refresh_tokens_user_agent_length;
