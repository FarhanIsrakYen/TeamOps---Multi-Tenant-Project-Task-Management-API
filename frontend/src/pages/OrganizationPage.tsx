import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { EmptyState } from "../components/EmptyState";
import { api, apiMessage } from "../lib/api";
import type { Envelope, Organization, Project } from "../types";

export function OrganizationPage() {
  const { organizationId = "" } = useParams();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const org = useQuery({
    queryKey: ["organization", organizationId],
    queryFn: async () =>
      (
        await api.get<Envelope<Organization>>(
          `/organizations/${organizationId}`,
        )
      ).data.data,
  });
  const projects = useQuery({
    queryKey: ["projects", organizationId],
    queryFn: async () =>
      (
        await api.get<Envelope<Project[]>>(
          `/organizations/${organizationId}/projects`,
        )
      ).data.data,
  });
  const create = useMutation({
    mutationFn: () =>
      api.post(`/organizations/${organizationId}/projects`, {
        name,
        description,
      }),
    onSuccess: async () => {
      setOpen(false);
      setName("");
      setDescription("");
      await qc.invalidateQueries({ queryKey: ["projects", organizationId] });
    },
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }
  if (org.error)
    return <div className="error-banner">{apiMessage(org.error)}</div>;
  return (
    <>
      <header className="page-header">
        <div>
          <Link className="breadcrumb" to="/organizations">
            Organizations /
          </Link>
          <h1>{org.data?.name ?? "Loading…"}</h1>
          <p>
            {org.data?.slug} · Your role: {org.data?.role}
          </p>
        </div>
        <button
          className="primary compact"
          disabled={org.data?.role === "VIEWER"}
          onClick={() => setOpen(true)}
        >
          + New project
        </button>
      </header>
      <div className="section-title">
        <h2>Projects</h2>
        <span>{projects.data?.length ?? 0} total</span>
      </div>
      {projects.data?.length === 0 && (
        <EmptyState
          title="No projects yet"
          body="Create a project to organize a stream of work."
        />
      )}
      <div className="project-list">
        {projects.data?.map((p) => (
          <Link key={p.id} to={`/projects/${p.id}`} className="project-row">
            <div className="project-icon">{p.name.slice(0, 1)}</div>
            <div>
              <h3>{p.name}</h3>
              <p>{p.description || "No description"}</p>
            </div>
            <span className={p.archived ? "badge muted" : "badge"}>
              {p.archived ? "Archived" : "Active"}
            </span>
            <span>→</span>
          </Link>
        ))}
      </div>
      {open && (
        <div className="modal-backdrop" onMouseDown={() => setOpen(false)}>
          <form
            className="modal"
            onMouseDown={(e) => e.stopPropagation()}
            onSubmit={submit}
          >
            <h2>New project</h2>
            {create.error && (
              <div className="error-banner">{apiMessage(create.error)}</div>
            )}
            <label>
              Name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </label>
            <label>
              Description
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={4}
              />
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setOpen(false)}
              >
                Cancel
              </button>
              <button className="primary">Create project</button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
