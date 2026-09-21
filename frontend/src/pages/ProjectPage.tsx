import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Button } from "../components/Button";
import { ErrorState } from "../components/ErrorState";
import { Form } from "../components/Form";
import { Input, TextArea } from "../components/Input";
import { Loading } from "../components/Loading";
import { Modal } from "../components/Modal";
import { useOrganization, useProject } from "../hooks/queries";
import { api, apiMessage } from "../lib/api";
import { formatDate } from "../lib/format";
import { canManageProjects } from "../lib/permissions";
import type { Envelope, Project } from "../types";

export function ProjectPage() {
  const { projectId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const project = useProject(projectId);
  const organization = useOrganization(project.data?.organizationId ?? "");
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [archived, setArchived] = useState(false);
  useEffect(() => {
    if (project.data) {
      setName(project.data.name);
      setDescription(project.data.description);
      setArchived(project.data.archived);
    }
  }, [project.data]);
  const update = useMutation({
    mutationFn: async () =>
      (
        await api.patch<Envelope<Project>>(`/projects/${projectId}`, {
          name: name.trim(),
          description: description.trim(),
          archived,
          version: project.data?.version,
        })
      ).data.data,
    onSuccess: (updated) => {
      queryClient.setQueryData(["project", projectId], updated);
      setEditing(false);
      void queryClient.invalidateQueries({
        queryKey: ["projects", updated.organizationId],
      });
    },
  });
  const remove = useMutation({
    mutationFn: () => api.delete(`/projects/${projectId}`),
    onSuccess: () => {
      if (project.data)
        void queryClient.invalidateQueries({
          queryKey: ["projects", project.data.organizationId],
        });
      navigate(
        project.data
          ? `/organizations/${project.data.organizationId}/projects`
          : "/organizations",
        { replace: true },
      );
    },
  });
  if (project.isLoading || (project.data && organization.isLoading))
    return <Loading label="Loading project" />;
  if (project.error || !project.data)
    return (
      <ErrorState error={project.error} retry={() => void project.refetch()} />
    );
  if (organization.error || !organization.data)
    return (
      <ErrorState
        error={organization.error}
        retry={() => void organization.refetch()}
      />
    );
  const manageable = canManageProjects(organization.data.role);
  return (
    <>
      <header className="page-header">
        <div>
          <Link
            className="breadcrumb"
            to={`/organizations/${project.data.organizationId}/projects`}
          >
            {organization.data.name} / Projects /
          </Link>
          <h1>{project.data.name}</h1>
          <p>{project.data.description || "No description"}</p>
        </div>
        {manageable && (
          <Button
            size="compact"
            variant="secondary"
            onClick={() => setEditing(true)}
          >
            Edit project
          </Button>
        )}
      </header>
      <section className="detail-grid">
        <article className="panel">
          <p className="eyebrow">PROJECT</p>
          <h2>Details</h2>
          <dl className="details">
            <div>
              <dt>Status</dt>
              <dd>
                <span
                  className={`badge static ${project.data.archived ? "muted" : ""}`}
                >
                  {project.data.archived ? "Archived" : "Active"}
                </span>
              </dd>
            </div>
            <div>
              <dt>Updated</dt>
              <dd>{formatDate(project.data.updatedAt)}</dd>
            </div>
            <div>
              <dt>Version</dt>
              <dd>{project.data.version}</dd>
            </div>
          </dl>
        </article>
        <article className="panel action-panel">
          <p className="eyebrow">WORK</p>
          <h2>Tasks</h2>
          <p>View, filter, prioritize, and assign this project’s tasks.</p>
          <Link
            className="button primary compact"
            to={`/projects/${projectId}/tasks`}
          >
            Open task list
          </Link>
        </article>
      </section>
      {manageable && (
        <section className="danger-zone">
          <div>
            <h2>Delete project</h2>
            <p>Permanently removes the project and its tasks.</p>
          </div>
          <Button variant="danger" onClick={() => setConfirmDelete(true)}>
            Delete project
          </Button>
        </section>
      )}
      <Modal
        open={editing}
        onClose={() => setEditing(false)}
        title="Edit project"
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
          <TextArea
            label="Description"
            name="description"
            maxLength={5000}
            rows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <label className="check-field">
            <input
              type="checkbox"
              checked={archived}
              onChange={(e) => setArchived(e.target.checked)}
            />{" "}
            Archived
          </label>
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
        title="Delete project?"
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
