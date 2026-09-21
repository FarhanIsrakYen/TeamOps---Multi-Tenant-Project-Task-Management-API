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
import { Pagination } from "../components/Pagination";
import { Select } from "../components/Select";
import { Table, type Column } from "../components/Table";
import { useOrganization, useProject } from "../hooks/queries";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { api, apiMessage, queryString } from "../lib/api";
import { formatDate } from "../lib/format";
import { canCreateOrUpdateTasks } from "../lib/permissions";
import type { Envelope, Membership, Task, TaskPriority } from "../types";

const statusOptions = [
  "TODO",
  "IN_PROGRESS",
  "IN_REVIEW",
  "DONE",
  "CANCELLED",
].map((value) => ({ value, label: value.replaceAll("_", " ") }));
const priorityOptions = ["LOW", "MEDIUM", "HIGH", "URGENT"].map((value) => ({
  value,
  label: value,
}));

export function TasksPage() {
  const { projectId = "" } = useParams();
  const queryClient = useQueryClient();
  const project = useProject(projectId);
  const organization = useOrganization(project.data?.organizationId ?? "");
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search);
  const [status, setStatus] = useState("");
  const [priority, setPriority] = useState("");
  const [assigneeId, setAssigneeId] = useState("");
  const [sort, setSort] = useState("-created_at");
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [newPriority, setNewPriority] = useState<TaskPriority>("MEDIUM");
  const [newAssigneeId, setNewAssigneeId] = useState("");
  const [dueAt, setDueAt] = useState("");
  const tasks = useQuery({
    queryKey: [
      "tasks",
      projectId,
      { page, search: debouncedSearch, status, priority, assigneeId, sort },
    ],
    enabled: Boolean(projectId),
    placeholderData: keepPreviousData,
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Task[]>>(
          `/projects/${projectId}/tasks${queryString({ page, page_size: 20, search: debouncedSearch, status, priority, assignee_id: assigneeId, sort })}`,
          { signal },
        )
      ).data,
  });
  const members = useQuery({
    queryKey: ["members", project.data?.organizationId],
    enabled: Boolean(project.data?.organizationId),
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Membership[]>>(
          `/organizations/${project.data?.organizationId}/members`,
          { signal },
        )
      ).data.data,
  });
  const create = useMutation({
    mutationFn: () =>
      api.post(`/projects/${projectId}/tasks`, {
        title: title.trim(),
        description: description.trim(),
        status: "TODO",
        priority: newPriority,
        assigneeId: newAssigneeId || undefined,
        dueAt: dueAt ? new Date(dueAt).toISOString() : undefined,
      }),
    onSuccess: async () => {
      setOpen(false);
      setTitle("");
      setDescription("");
      setNewPriority("MEDIUM");
      setNewAssigneeId("");
      setDueAt("");
      await queryClient.invalidateQueries({ queryKey: ["tasks", projectId] });
    },
  });
  if (project.isLoading || (project.data && organization.isLoading))
    return <Loading label="Loading tasks" />;
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
  const memberName = new Map(
    members.data?.map((member) => [
      member.userId,
      member.user?.name ?? member.userId,
    ]),
  );
  const columns: Column<Task>[] = [
    {
      key: "task",
      header: "Task",
      render: (task) => (
        <Link className="table-link" to={`/tasks/${task.id}`}>
          <strong>{task.title}</strong>
          <small>{task.description || "No description"}</small>
        </Link>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (task) => (
        <span className={`status ${task.status.toLowerCase()}`}>
          {task.status.replaceAll("_", " ")}
        </span>
      ),
    },
    {
      key: "priority",
      header: "Priority",
      render: (task) => (
        <span className={`priority ${task.priority.toLowerCase()}`}>
          {task.priority}
        </span>
      ),
    },
    {
      key: "assignee",
      header: "Assignee",
      render: (task) =>
        task.assigneeId
          ? (memberName.get(task.assigneeId) ?? "Unknown member")
          : "Unassigned",
    },
    { key: "due", header: "Due", render: (task) => formatDate(task.dueAt) },
  ];
  const resetPage = () => setPage(1);
  const assigneeOptions = [
    { value: "", label: "Anyone" },
    ...(members.data ?? []).map((member) => ({
      value: member.userId,
      label: member.user?.name ?? member.userId,
    })),
  ];
  return (
    <>
      <header className="page-header">
        <div>
          <Link className="breadcrumb" to={`/projects/${projectId}`}>
            {project.data.name} /
          </Link>
          <h1>Tasks</h1>
          <p>Filter and prioritize work without losing the project context.</p>
        </div>
        {canCreateOrUpdateTasks(organization.data.role) && (
          <Button size="compact" onClick={() => setOpen(true)}>
            + Add task
          </Button>
        )}
      </header>
      <div className="toolbar filters task-filters">
        <Input
          label="Search"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            resetPage();
          }}
          placeholder="Title or description"
        />
        <Select
          label="Status"
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            resetPage();
          }}
          options={[{ value: "", label: "All statuses" }, ...statusOptions]}
        />
        <Select
          label="Priority"
          value={priority}
          onChange={(e) => {
            setPriority(e.target.value);
            resetPage();
          }}
          options={[{ value: "", label: "All priorities" }, ...priorityOptions]}
        />
        <Select
          label="Assignee"
          value={assigneeId}
          onChange={(e) => {
            setAssigneeId(e.target.value);
            resetPage();
          }}
          options={assigneeOptions}
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
            { value: "due_at", label: "Due date" },
            { value: "-priority", label: "Priority" },
            { value: "title", label: "Title" },
          ]}
        />
      </div>
      {tasks.isLoading && <Loading label="Loading tasks" />}
      {tasks.error && (
        <ErrorState error={tasks.error} retry={() => void tasks.refetch()} />
      )}
      {tasks.data?.data.length === 0 && (
        <EmptyState
          title="No tasks found"
          body={
            search || status || priority || assigneeId
              ? "Adjust the filters to see more results."
              : "Create the first task for this project."
          }
        />
      )}
      {tasks.data && tasks.data.data.length > 0 && (
        <Table
          columns={columns}
          rows={tasks.data.data}
          rowKey={(task) => task.id}
        />
      )}
      {tasks.data?.meta && (
        <Pagination {...tasks.data.meta} onPageChange={setPage} />
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="Add task">
        <Form
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            create.mutate();
          }}
          error={create.error ? apiMessage(create.error) : undefined}
        >
          <Input
            label="Title"
            name="title"
            minLength={2}
            maxLength={200}
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <TextArea
            label="Description"
            name="description"
            maxLength={10000}
            rows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <div className="form-grid">
            <Select
              label="Priority"
              value={newPriority}
              onChange={(e) => setNewPriority(e.target.value as TaskPriority)}
              options={priorityOptions}
            />
            <Select
              label="Assignee"
              value={newAssigneeId}
              onChange={(e) => setNewAssigneeId(e.target.value)}
              options={assigneeOptions}
            />
          </div>
          <Input
            label="Due date"
            type="datetime-local"
            value={dueAt}
            onChange={(e) => setDueAt(e.target.value)}
          />
          <div className="modal-actions">
            <Button
              type="button"
              variant="secondary"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button busy={create.isPending}>Add task</Button>
          </div>
        </Form>
      </Modal>
    </>
  );
}
