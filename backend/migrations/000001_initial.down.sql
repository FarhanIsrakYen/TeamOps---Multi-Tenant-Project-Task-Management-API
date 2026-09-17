DROP TABLE IF EXISTS
  audit_logs,
  refresh_tokens,
  task_label_assignments,
  task_labels,
  task_comments,
  tasks,
  projects,
  organization_members,
  organizations,
  users
CASCADE;

DROP TYPE IF EXISTS task_priority, task_status, organization_role;

