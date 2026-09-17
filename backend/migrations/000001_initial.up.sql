CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE organization_role AS ENUM ('OWNER', 'ADMIN', 'MEMBER', 'VIEWER');
CREATE TYPE task_status AS ENUM ('TODO', 'IN_PROGRESS', 'DONE', 'CANCELLED');
CREATE TYPE task_priority AS ENUM ('LOW', 'MEDIUM', 'HIGH', 'URGENT');

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  password_hash text NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_email_normalized CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
  CONSTRAINT users_name_not_blank CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  CONSTRAINT users_password_hash_not_blank CHECK (length(password_hash) > 0)
);

CREATE TABLE organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  slug text NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT organizations_slug_unique UNIQUE (slug),
  CONSTRAINT organizations_name_not_blank CHECK (length(btrim(name)) BETWEEN 1 AND 120),
  CONSTRAINT organizations_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  CONSTRAINT organizations_version_positive CHECK (version > 0)
);
CREATE INDEX organizations_created_by_idx ON organizations(created_by);

CREATE TABLE organization_members (
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role organization_role NOT NULL DEFAULT 'MEMBER',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, user_id)
);
CREATE INDEX organization_members_user_lookup_idx ON organization_members(user_id, organization_id);
CREATE INDEX organization_members_org_role_idx ON organization_members(organization_id, role);

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  archived boolean NOT NULL DEFAULT false,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT projects_id_organization_unique UNIQUE (id, organization_id),
  CONSTRAINT projects_org_name_unique UNIQUE (organization_id, name),
  CONSTRAINT projects_name_not_blank CHECK (length(btrim(name)) BETWEEN 1 AND 120),
  CONSTRAINT projects_version_positive CHECK (version > 0)
);
CREATE INDEX projects_organization_lookup_idx ON projects(organization_id, created_at DESC);
CREATE INDEX projects_active_organization_idx ON projects(organization_id, updated_at DESC) WHERE archived = false;

CREATE TABLE tasks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id uuid NOT NULL,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  status task_status NOT NULL DEFAULT 'TODO',
  priority task_priority NOT NULL DEFAULT 'MEDIUM',
  assignee_id uuid REFERENCES users(id) ON DELETE SET NULL,
  created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  due_date timestamptz,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tasks_id_organization_unique UNIQUE (id, organization_id),
  CONSTRAINT tasks_project_organization_fk FOREIGN KEY (project_id, organization_id) REFERENCES projects(id, organization_id) ON DELETE CASCADE,
  CONSTRAINT tasks_title_not_blank CHECK (length(btrim(title)) BETWEEN 1 AND 200),
  CONSTRAINT tasks_version_positive CHECK (version > 0)
);
CREATE INDEX tasks_project_lookup_idx ON tasks(project_id, created_at DESC);
CREATE INDEX tasks_project_status_idx ON tasks(project_id, status, updated_at DESC);
CREATE INDEX tasks_assignee_lookup_idx ON tasks(assignee_id, status, due_date) WHERE assignee_id IS NOT NULL;
CREATE INDEX tasks_created_by_idx ON tasks(created_by, created_at DESC);
CREATE INDEX tasks_organization_status_idx ON tasks(organization_id, status);

CREATE TABLE task_comments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  author_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  body text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT task_comments_body_not_blank CHECK (length(btrim(body)) BETWEEN 1 AND 5000)
);
CREATE INDEX task_comments_task_lookup_idx ON task_comments(task_id, created_at, id);
CREATE INDEX task_comments_author_idx ON task_comments(author_id, created_at DESC);

CREATE TABLE task_labels (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name text NOT NULL,
  color varchar(7) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT task_labels_id_organization_unique UNIQUE (id, organization_id),
  CONSTRAINT task_labels_org_name_unique UNIQUE (organization_id, name),
  CONSTRAINT task_labels_name_not_blank CHECK (length(btrim(name)) BETWEEN 1 AND 50),
  CONSTRAINT task_labels_color_hex CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);
CREATE INDEX task_labels_organization_lookup_idx ON task_labels(organization_id, name);

CREATE TABLE task_label_assignments (
  task_id uuid NOT NULL,
  task_label_id uuid NOT NULL,
  organization_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (task_id, task_label_id),
  CONSTRAINT task_label_assignments_task_fk FOREIGN KEY (task_id, organization_id) REFERENCES tasks(id, organization_id) ON DELETE CASCADE,
  CONSTRAINT task_label_assignments_label_fk FOREIGN KEY (task_label_id, organization_id) REFERENCES task_labels(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX task_label_assignments_label_lookup_idx ON task_label_assignments(task_label_id, task_id);

CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY,
  family_id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  replaced_by uuid REFERENCES refresh_tokens(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED,
  user_agent text NOT NULL DEFAULT '',
  ip_address inet,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT refresh_tokens_hash_unique UNIQUE (token_hash),
  CONSTRAINT refresh_tokens_hash_length CHECK (octet_length(token_hash) = 32),
  CONSTRAINT refresh_tokens_expiry_valid CHECK (expires_at > created_at),
  CONSTRAINT refresh_tokens_revocation_valid CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE INDEX refresh_tokens_user_lookup_idx ON refresh_tokens(user_id, created_at DESC);
CREATE INDEX refresh_tokens_family_lookup_idx ON refresh_tokens(family_id);
CREATE INDEX refresh_tokens_expiry_cleanup_idx ON refresh_tokens(expires_at);
CREATE INDEX refresh_tokens_replaced_by_idx ON refresh_tokens(replaced_by) WHERE replaced_by IS NOT NULL;

CREATE TABLE audit_logs (
  id bigserial PRIMARY KEY,
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}',
  request_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT audit_logs_action_not_blank CHECK (length(btrim(action)) > 0),
  CONSTRAINT audit_logs_resource_type_not_blank CHECK (length(btrim(resource_type)) > 0),
  CONSTRAINT audit_logs_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
);
CREATE INDEX audit_logs_organization_lookup_idx ON audit_logs(organization_id, created_at DESC, id DESC);
CREATE INDEX audit_logs_actor_lookup_idx ON audit_logs(actor_id, created_at DESC) WHERE actor_id IS NOT NULL;
CREATE INDEX audit_logs_resource_lookup_idx ON audit_logs(organization_id, resource_type, resource_id);
