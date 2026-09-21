import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { ErrorState } from "../components/ErrorState";
import { Form } from "../components/Form";
import { Input, TextArea } from "../components/Input";
import { Loading } from "../components/Loading";
import { Modal } from "../components/Modal";
import { OrganizationNav } from "../components/OrganizationNav";
import { Pagination } from "../components/Pagination";
import { Select } from "../components/Select";
import { Table, type Column } from "../components/Table";
import { useOrganization } from "../hooks/queries";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { api, apiMessage, queryString } from "../lib/api";
import { formatDate } from "../lib/format";
import { canManageProjects } from "../lib/permissions";
import type { Envelope, Project } from "../types";

export function ProjectsPage() {
  const { organizationId = "" } = useParams();
  const queryClient = useQueryClient();
  const organization = useOrganization(organizationId);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search);
  const [archived, setArchived] = useState("");
  const [sort, setSort] = useState("-created_at");
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const projects = useQuery({
    queryKey: [
      "projects",
      organizationId,
      { page, search: debouncedSearch, archived, sort },
    ],
    enabled: Boolean(organizationId),
    placeholderData: keepPreviousData,
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Project[]>>(
          `/organizations/${organizationId}/projects${queryString({ page, page_size: 20, search: debouncedSearch, archived, sort })}`,
          { signal },
        )
      ).data,
  });
  const create = useMutation({
    mutationFn: async () =>
      (
        await api.post<Envelope<Project>>(
          `/organizations/${organizationId}/projects`,
          { name: name.trim(), description: description.trim() },
        )
      ).data.data,
    onSuccess: async () => {
      setOpen(false);
      setName("");
      setDescription("");
      await queryClient.invalidateQueries({
        queryKey: ["projects", organizationId],
      });
    },
  });
  if (organization.isLoading) return <Loading label="Loading projects" />;
  if (organization.error || !organization.data)
    return (
      <ErrorState
        error={organization.error}
        retry={() => void organization.refetch()}
      />
    );
  const columns: Column<Project>[] = [
    {
      key: "project",
      header: "Project",
      render: (project) => (
        <Link className="table-link" to={`/projects/${project.id}`}>
          <strong>{project.name}</strong>
          <small>{project.description || "No description"}</small>
        </Link>
      ),
    },
    {
      key: "state",
      header: "State",
      render: (project) => (
        <span className={`badge static ${project.archived ? "muted" : ""}`}>
          {project.archived ? "Archived" : "Active"}
        </span>
      ),
    },
    {
      key: "updated",
      header: "Updated",
      render: (project) => formatDate(project.updatedAt),
    },
    {
      key: "version",
      header: "Version",
      render: (project) => `v${project.version}`,
    },
  ];
  const resetPage = () => setPage(1);
  return (
    <>
      <header className="page-header">
        <div>
          <Link className="breadcrumb" to={`/organizations/${organizationId}`}>
            {organization.data.name} /
          </Link>
          <h1>Projects</h1>
          <p>Plan distinct streams of work inside this organization.</p>
        </div>
        {canManageProjects(organization.data.role) && (
          <Button size="compact" onClick={() => setOpen(true)}>
            + New project
          </Button>
        )}
      </header>
      <OrganizationNav organization={organization.data} />
      <div className="toolbar filters">
        <Input
          aria-label="Search projects"
          label="Search"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            resetPage();
          }}
          placeholder="Name or description"
        />
        <Select
          label="State"
          value={archived}
          onChange={(e) => {
            setArchived(e.target.value);
            resetPage();
          }}
          options={[
            { value: "", label: "All" },
            { value: "false", label: "Active" },
            { value: "true", label: "Archived" },
          ]}
        />
        <Select
          label="Sort"
          value={sort}
          onChange={(e) => {
            setSort(e.target.value);
            resetPage();
          }}
          options={[
            { value: "-created_at", label: "Newest" },
            { value: "-updated_at", label: "Recently updated" },
            { value: "name", label: "Name A–Z" },
          ]}
        />
      </div>
      {projects.isLoading && <Loading label="Loading projects" />}
      {projects.error && (
        <ErrorState
          error={projects.error}
          retry={() => void projects.refetch()}
        />
      )}
      {projects.data?.data.length === 0 && (
        <EmptyState
          title="No projects found"
          body={
            search || archived
              ? "Adjust the filters to see more results."
              : "Create a project to organize a stream of work."
          }
        />
      )}
      {projects.data && projects.data.data.length > 0 && (
        <Table
          columns={columns}
          rows={projects.data.data}
          rowKey={(project) => project.id}
        />
      )}
      {projects.data?.meta && (
        <Pagination {...projects.data.meta} onPageChange={setPage} />
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="New project">
        <Form
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            create.mutate();
          }}
          error={create.error ? apiMessage(create.error) : undefined}
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
          <div className="modal-actions">
            <Button
              type="button"
              variant="secondary"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button busy={create.isPending}>Create project</Button>
          </div>
        </Form>
      </Modal>
    </>
  );
}
