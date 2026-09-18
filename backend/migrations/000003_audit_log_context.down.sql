DROP INDEX IF EXISTS audit_logs_created_at_idx;
DROP INDEX IF EXISTS audit_logs_action_lookup_idx;

ALTER INDEX audit_logs_actor_user_lookup_idx RENAME TO audit_logs_actor_lookup_idx;

ALTER TABLE audit_logs
  DROP CONSTRAINT audit_logs_user_agent_length,
  DROP CONSTRAINT audit_logs_resource_id_length,
  DROP CONSTRAINT audit_logs_resource_type_length,
  DROP CONSTRAINT audit_logs_action_length,
  DROP COLUMN user_agent,
  DROP COLUMN ip_address;

DELETE FROM audit_logs a
WHERE organization_id IS NULL
   OR NOT EXISTS (SELECT 1 FROM organizations o WHERE o.id=a.organization_id);

ALTER TABLE audit_logs
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT audit_logs_organization_id_fkey
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE audit_logs RENAME COLUMN actor_user_id TO actor_id;
