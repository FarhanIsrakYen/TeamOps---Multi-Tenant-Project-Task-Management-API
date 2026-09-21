import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { ErrorState } from "../components/ErrorState";
import { Form } from "../components/Form";
import { Input } from "../components/Input";
import { Loading } from "../components/Loading";
import { Modal } from "../components/Modal";
import { Pagination } from "../components/Pagination";
import { Select } from "../components/Select";
import { api, apiMessage, queryString } from "../lib/api";
import { initials } from "../lib/format";
import type { Envelope, Organization } from "../types";

export function OrganizationsPage() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [sort, setSort] = useState("-created_at");
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const organizations = useQuery({
    queryKey: ["organizations", { page, pageSize: 12, sort }],
    placeholderData: keepPreviousData,
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Organization[]>>(
          `/organizations${queryString({ page, page_size: 12, sort })}`,
          { signal },
        )
      ).data,
  });
  const create = useMutation({
    mutationFn: async () =>
      (
        await api.post<Envelope<Organization>>("/organizations", {
          name: name.trim(),
          slug,
        })
      ).data.data,
    onSuccess: async () => {
      setOpen(false);
      setName("");
      setSlug("");
      setPage(1);
      await queryClient.invalidateQueries({ queryKey: ["organizations"] });
    },
  });
  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate();
  }
  function updateName(value: string) {
    setName(value);
    setSlug(
      value
        .toLowerCase()
        .trim()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-|-$/g, ""),
    );
  }
  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">YOUR WORKSPACE</p>
          <h1>Organizations</h1>
          <p>
            Membership, projects, and permissions stay isolated per
            organization.
          </p>
        </div>
        <Button size="compact" onClick={() => setOpen(true)}>
          + New organization
        </Button>
      </header>
      <div className="toolbar">
        <Select
          label="Sort organizations"
          value={sort}
          onChange={(e) => {
            setSort(e.target.value);
            setPage(1);
          }}
          options={[
            { value: "-created_at", label: "Newest" },
            { value: "created_at", label: "Oldest" },
            { value: "name", label: "Name A–Z" },
            { value: "-name", label: "Name Z–A" },
          ]}
        />
      </div>
      {organizations.isLoading && <Loading label="Loading organizations" />}
      {organizations.error && (
        <ErrorState
          error={organizations.error}
          retry={() => void organizations.refetch()}
        />
      )}
      {organizations.data?.data.length === 0 && (
        <EmptyState
          title="Start with an organization"
          body="Create an isolated workspace for your team."
        />
      )}
      <div className="card-grid">
        {organizations.data?.data.map((organization) => (
          <Link
            className="org-card"
            to={`/organizations/${organization.id}`}
            key={organization.id}
          >
            <div className="org-monogram">{initials(organization.name)}</div>
            <span className="role">{organization.role}</span>
            <h3>{organization.name}</h3>
            <p>{organization.slug}</p>
            <span className="open-link">Open workspace →</span>
          </Link>
        ))}
      </div>
      {organizations.data?.meta && (
        <Pagination
          page={organizations.data.meta.page}
          totalPages={organizations.data.meta.totalPages}
          total={organizations.data.meta.total}
          onPageChange={setPage}
        />
      )}
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title="New organization"
        description="Create an isolated space for a team or company."
      >
        <Form
          onSubmit={submit}
          error={create.error ? apiMessage(create.error) : undefined}
        >
          <Input
            label="Name"
            name="name"
            minLength={2}
            maxLength={120}
            required
            value={name}
            onChange={(e) => updateName(e.target.value)}
          />
          <Input
            label="Slug"
            name="slug"
            minLength={2}
            maxLength={80}
            pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
            required
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            hint="Lowercase letters, numbers, and single hyphens."
          />
          <div className="modal-actions">
            <Button
              type="button"
              variant="secondary"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button busy={create.isPending}>Create organization</Button>
          </div>
        </Form>
      </Modal>
    </>
  );
}
