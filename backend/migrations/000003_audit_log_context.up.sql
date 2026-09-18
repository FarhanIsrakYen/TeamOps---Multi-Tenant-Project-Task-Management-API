ALTER TABLE audit_logs RENAME COLUMN actor_id TO actor_user_id;

ALTER TABLE audit_logs
  ALTER COLUMN organization_id DROP NOT NULL,
  DROP CONSTRAINT audit_logs_organization_id_fkey,
  ADD COLUMN ip_address inet,
  ADD COLUMN user_agent text NOT NULL DEFAULT '';

ALTER TABLE audit_logs
  ADD CONSTRAINT audit_logs_action_length CHECK (length(action) <= 100),
  ADD CONSTRAINT audit_logs_resource_type_length CHECK (length(resource_type) <= 100),
  ADD CONSTRAINT audit_logs_resource_id_length CHECK (length(resource_id) <= 255),
  ADD CONSTRAINT audit_logs_user_agent_length CHECK (length(user_agent) <= 512);

ALTER INDEX audit_logs_actor_lookup_idx RENAME TO audit_logs_actor_user_lookup_idx;

CREATE INDEX audit_logs_action_lookup_idx
  ON audit_logs(organization_id, action, created_at DESC, id DESC);

CREATE INDEX audit_logs_created_at_idx
  ON audit_logs(created_at DESC, id DESC);
