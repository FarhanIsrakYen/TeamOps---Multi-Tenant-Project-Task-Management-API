import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { ErrorState } from "../components/ErrorState";
import { Form } from "../components/Form";
import { Input, TextArea } from "../components/Input";
import { Loading } from "../components/Loading";
import { Modal } from "../components/Modal";
import { Select } from "../components/Select";
import { useOrganization, useProject } from "../hooks/queries";
import { api, apiMessage } from "../lib/api";
import { formatDate } from "../lib/format";
import { canCreateOrUpdateTasks, canDeleteTasks } from "../lib/permissions";
import type {
  Comment,
  Envelope,
  Membership,
  Task,
  TaskPriority,
  TaskStatus,
} from "../types";

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

export function TaskPage() {
  const { taskId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const task = useQuery({
    queryKey: ["task", taskId],
    queryFn: async ({ signal }) =>
      (await api.get<Envelope<Task>>(`/tasks/${taskId}`, { signal })).data.data,
  });
  const project = useProject(task.data?.projectId ?? "");
  const organization = useOrganization(task.data?.organizationId ?? "");
  const members = useQuery({
    queryKey: ["members", task.data?.organizationId],
    enabled: Boolean(task.data?.organizationId),
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Membership[]>>(
          `/organizations/${task.data?.organizationId}/members`,
          { signal },
        )
      ).data.data,
  });
  const comments = useQuery({
    queryKey: ["comments", taskId],
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Comment[]>>(`/tasks/${taskId}/comments`, {
          signal,
        })
      ).data.data,
  });
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<TaskStatus>("TODO");
  const [priority, setPriority] = useState<TaskPriority>("MEDIUM");
  const [assigneeId, setAssigneeId] = useState("");
  const [dueAt, setDueAt] = useState("");
  const [comment, setComment] = useState("");
  useEffect(() => {
    if (task.data) {
      setTitle(task.data.title);
      setDescription(task.data.description);
      setStatus(task.data.status);
      setPriority(task.data.priority);
      setAssigneeId(task.data.assigneeId ?? "");
      setDueAt(task.data.dueAt ? task.data.dueAt.slice(0, 16) : "");
    }
  }, [task.data]);
  const update = useMutation({
    mutationFn: async () =>
      (
        await api.patch<Envelope<Task>>(`/tasks/${taskId}`, {
          title: title.trim(),
          description: description.trim(),
          status,
          priority,
          assigneeId: assigneeId || null,
          dueAt: dueAt ? new Date(dueAt).toISOString() : null,
          version: task.data?.version,
        })
      ).data.data,
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ["task", taskId] });
      const previous = queryClient.getQueryData<Task>(["task", taskId]);
      if (previous)
        queryClient.setQueryData<Task>(["task", taskId], {
          ...previous,
          title,
          description,
          status,
          priority,
          assigneeId: assigneeId || undefined,
          dueAt: dueAt || undefined,
          version: previous.version + 1,
        });
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous)
        queryClient.setQueryData(["task", taskId], context.previous);
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(["task", taskId], updated);
      setEditing(false);
      void queryClient.invalidateQueries({
        queryKey: ["tasks", updated.projectId],
      });
    },
  });
  const remove = useMutation({
    mutationFn: () => api.delete(`/tasks/${taskId}`),
    onSuccess: () => {
      if (task.data)
        void queryClient.invalidateQueries({
          queryKey: ["tasks", task.data.projectId],
        });
      navigate(
        task.data ? `/projects/${task.data.projectId}/tasks` : "/organizations",
        { replace: true },
      );
    },
  });
  const addComment = useMutation({
    mutationFn: () =>
      api.post(`/tasks/${taskId}/comments`, { body: comment.trim() }),
    onSuccess: async () => {
      setComment("");
      await queryClient.invalidateQueries({ queryKey: ["comments", taskId] });
    },
  });
  if (
    task.isLoading ||
    (task.data && (project.isLoading || organization.isLoading))
  )
    return <Loading label="Loading task" />;
  if (task.error || !task.data)
    return <ErrorState error={task.error} retry={() => void task.refetch()} />;
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
  const canEdit = canCreateOrUpdateTasks(organization.data.role);
  const memberName = new Map(
    members.data?.map((member) => [
      member.userId,
      member.user?.name ?? member.userId,
    ]),
  );
  const assigneeOptions = [
    { value: "", label: "Unassigned" },
    ...(members.data ?? []).map((member) => ({
      value: member.userId,
      label: member.user?.name ?? member.userId,
    })),
  ];
  return (
    <>
      <header className="page-header">
        <div>
          <Link
            className="breadcrumb"
            to={`/projects/${task.data.projectId}/tasks`}
          >
            {project.data.name} / Tasks /
          </Link>
          <h1>{task.data.title}</h1>
          <p>
            Version {task.data.version} · Updated{" "}
            {formatDate(task.data.updatedAt)}
          </p>
        </div>
        {canEdit && (
          <Button size="compact" onClick={() => setEditing(true)}>
            Edit task
          </Button>
        )}
      </header>
      <section className="detail-grid">
        <article className="panel">
          <div className="task-heading">
            <span className={`status ${task.data.status.toLowerCase()}`}>
              {task.data.status.replaceAll("_", " ")}
            </span>
            <span className={`priority ${task.data.priority.toLowerCase()}`}>
              {task.data.priority}
            </span>
          </div>
          <h2>Description</h2>
          <p className="prose">
            {task.data.description || "No description provided."}
          </p>
        </article>
        <article className="panel">
          <p className="eyebrow">DETAILS</p>
          <dl className="details">
            <div>
              <dt>Assignee</dt>
              <dd>
                {task.data.assigneeId
                  ? (memberName.get(task.data.assigneeId) ?? "Unknown member")
                  : "Unassigned"}
              </dd>
            </div>
            <div>
              <dt>Due date</dt>
              <dd>{formatDate(task.data.dueAt)}</dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd>{formatDate(task.data.createdAt)}</dd>
            </div>
          </dl>
        </article>
      </section>
      <section className="panel comments">
        <div className="section-title">
          <h2>Comments</h2>
          <span>{comments.data?.length ?? 0}</span>
        </div>
        {comments.error && (
          <ErrorState
            error={comments.error}
            retry={() => void comments.refetch()}
          />
        )}
        {comments.data?.length === 0 && (
          <EmptyState
            title="No comments yet"
            body="Start the conversation around this task."
          />
        )}
        {comments.data?.map((item) => (
          <article className="comment" key={item.id}>
            <div>
              <strong>{memberName.get(item.authorId) ?? "Team member"}</strong>
              <time>{formatDate(item.createdAt)}</time>
            </div>
            <p>{item.body}</p>
          </article>
        ))}
        {canEdit && (
          <Form
            className="comment-form"
            onSubmit={(event: FormEvent) => {
              event.preventDefault();
              if (comment.trim()) addComment.mutate();
            }}
            error={addComment.error ? apiMessage(addComment.error) : undefined}
          >
            <TextArea
              label="Add a comment"
              rows={3}
              maxLength={5000}
              required
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
            <Button size="compact" busy={addComment.isPending}>
              Post comment
            </Button>
          </Form>
        )}
      </section>
      {canDeleteTasks(organization.data.role) && (
        <section className="danger-zone">
          <div>
            <h2>Delete task</h2>
            <p>Permanently removes this task and its comments.</p>
          </div>
          <Button variant="danger" onClick={() => setConfirmDelete(true)}>
            Delete task
          </Button>
        </section>
      )}
      <Modal open={editing} onClose={() => setEditing(false)} title="Edit task">
        <Form
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            update.mutate();
          }}
          error={update.error ? apiMessage(update.error) : undefined}
        >
          <Input
            label="Title"
            minLength={2}
            maxLength={200}
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <TextArea
            label="Description"
            rows={4}
            maxLength={10000}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <div className="form-grid">
            <Select
              label="Status"
              value={status}
              onChange={(e) => setStatus(e.target.value as TaskStatus)}
              options={statusOptions}
            />
            <Select
              label="Priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as TaskPriority)}
              options={priorityOptions}
            />
          </div>
          <Select
            label="Assignee"
            value={assigneeId}
            onChange={(e) => setAssigneeId(e.target.value)}
            options={assigneeOptions}
          />
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
        title="Delete task?"
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
