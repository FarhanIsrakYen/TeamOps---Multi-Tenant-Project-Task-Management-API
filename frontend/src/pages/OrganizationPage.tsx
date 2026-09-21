import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Button } from "../components/Button";
import { ErrorState } from "../components/ErrorState";
import { Form } from "../components/Form";
import { Input } from "../components/Input";
import { Loading } from "../components/Loading";
import { Modal } from "../components/Modal";
import { OrganizationNav } from "../components/OrganizationNav";
import { useOrganization } from "../hooks/queries";
import { api, apiMessage } from "../lib/api";
import { formatDate } from "../lib/format";
import {
  canDeleteOrganization,
  canManageOrganization,
} from "../lib/permissions";
import type { Envelope, Organization } from "../types";

export function OrganizationPage() {
  const { organizationId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const organization = useOrganization(organizationId);
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  useEffect(() => {
    if (organization.data) {
      setName(organization.data.name);
      setSlug(organization.data.slug);
    }
  }, [organization.data]);
  const update = useMutation({
    mutationFn: async () =>
      (
        await api.patch<Envelope<Organization>>(
          `/organizations/${organizationId}`,
          { name: name.trim(), slug, version: organization.data?.version },
        )
      ).data.data,
    onSuccess: (updated) => {
      queryClient.setQueryData(["organization", organizationId], updated);
      setEditing(false);
      void queryClient.invalidateQueries({ queryKey: ["organizations"] });
    },
  });
  const remove = useMutation({
    mutationFn: () => api.delete(`/organizations/${organizationId}`),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["organization", organizationId] });
      void queryClient.invalidateQueries({ queryKey: ["organizations"] });
      navigate("/organizations", { replace: true });
    },
  });
  if (organization.isLoading) return <Loading label="Loading organization" />;
  if (organization.error || !organization.data)
    return (
      <ErrorState
        error={organization.error}
        retry={() => void organization.refetch()}
      />
    );
  const item = organization.data;
  return (
    <>
      <header className="page-header">
        <div>
          <Link className="breadcrumb" to="/organizations">
            Organizations /
          </Link>
          <h1>{item.name}</h1>
          <p>
            {item.slug} · Your role: {item.role}
          </p>
        </div>
        {canManageOrganization(item.role) && (
          <Button
            size="compact"
            variant="secondary"
            onClick={() => setEditing(true)}
          >
            Edit organization
          </Button>
        )}
      </header>
      <OrganizationNav organization={item} />
      <section className="detail-grid">
        <article className="panel">
          <p className="eyebrow">ORGANIZATION</p>
          <h2>Workspace details</h2>
          <dl className="details">
            <div>
              <dt>Name</dt>
              <dd>{item.name}</dd>
            </div>
            <div>
              <dt>Slug</dt>
              <dd>{item.slug}</dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd>{formatDate(item.createdAt)}</dd>
            </div>
            <div>
              <dt>Version</dt>
              <dd>{item.version}</dd>
            </div>
          </dl>
        </article>
        <article className="panel">
          <p className="eyebrow">QUICK LINKS</p>
          <h2>Manage workspace</h2>
          <div className="quick-links">
            <Link to={`/organizations/${item.id}/members`}>
              Members <span>→</span>
            </Link>
            <Link to={`/organizations/${item.id}/projects`}>
              Projects <span>→</span>
            </Link>
            {canManageOrganization(item.role) && (
              <Link to={`/organizations/${item.id}/audit-logs`}>
                Audit log <span>→</span>
              </Link>
            )}
          </div>
        </article>
      </section>
      {canDeleteOrganization(item.role) && (
        <section className="danger-zone">
          <div>
            <h2>Delete organization</h2>
            <p>Permanently deletes its projects, tasks, and memberships.</p>
          </div>
          <Button variant="danger" onClick={() => setConfirmDelete(true)}>
            Delete organization
          </Button>
        </section>
      )}
      <Modal
        open={editing}
        onClose={() => setEditing(false)}
        title="Edit organization"
      >
        <Form
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            update.mutate();
          }}
          error={update.error ? apiMessage(update.error) : undefined}
        >
          <Input
            label="Name"
            name="name"
            minLength={2}
            maxLength={120}
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <Input
            label="Slug"
            name="slug"
            required
            pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
          />
          <div className="modal-actions">
            <Button
              type="button"
              variant="secondary"
              onClick={() => setEditing(false)}
            >
              Cancel
            </Button>
            <Button busy={update.isPending}>Save changes</Button>
          </div>
        </Form>
      </Modal>
      <Modal
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        title="Delete organization?"
        description="This action cannot be undone."
      >
        <div className="modal-actions">
          <Button variant="secondary" onClick={() => setConfirmDelete(false)}>
            Cancel
          </Button>
          <Button
            variant="danger"
            busy={remove.isPending}
            onClick={() => remove.mutate()}
          >
            Delete permanently
          </Button>
        </div>
        {remove.error && (
          <div className="error-banner">{apiMessage(remove.error)}</div>
        )}
      </Modal>
    </>
  );
}
