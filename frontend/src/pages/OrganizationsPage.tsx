import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { EmptyState } from "../components/EmptyState";
import { api, apiMessage } from "../lib/api";
import type { Envelope, Organization } from "../types";

export function OrganizationsPage() {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const query = useQuery({
    queryKey: ["organizations"],
    queryFn: async () =>
      (await api.get<Envelope<Organization[]>>("/organizations")).data,
  });
  const create = useMutation({
    mutationFn: async () => api.post("/organizations", { name, slug }),
    onSuccess: async () => {
      setOpen(false);
      setName("");
      setSlug("");
      await qc.invalidateQueries({ queryKey: ["organizations"] });
    },
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }
  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">YOUR WORKSPACE</p>
          <h1>Organizations</h1>
          <p>Choose a team space or create a new one.</p>
        </div>
        <button className="primary compact" onClick={() => setOpen(true)}>
          + New organization
        </button>
      </header>
      {query.isLoading && <div className="loading">Loading organizations…</div>}
      {query.error && (
        <div className="error-banner">{apiMessage(query.error)}</div>
      )}
      {query.data?.data.length === 0 && (
        <EmptyState
          title="Start with an organization"
          body="Organizations keep membership, projects, and permissions isolated."
        />
      )}
      <div className="card-grid">
        {query.data?.data.map((org) => (
          <Link
            className="org-card"
            to={`/organizations/${org.id}`}
            key={org.id}
          >
            <div className="org-monogram">
              {org.name.slice(0, 2).toUpperCase()}
            </div>
            <span className="role">{org.role}</span>
            <h3>{org.name}</h3>
            <p>{org.slug}</p>
            <span className="open-link">Open workspace →</span>
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
            <h2>New organization</h2>
            <p>Create an isolated space for a team or company.</p>
            {create.error && (
              <div className="error-banner">{apiMessage(create.error)}</div>
            )}
            <label>
              Name
              <input
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  if (!slug)
                    setSlug(
                      e.target.value
                        .toLowerCase()
                        .replace(/[^a-z0-9]+/g, "-")
                        .replace(/^-|-$/g, ""),
                    );
                }}
                required
              />
            </label>
            <label>
              Slug
              <input
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                pattern="[a-z0-9-]+"
                required
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
              <button className="primary" disabled={create.isPending}>
                Create
              </button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
