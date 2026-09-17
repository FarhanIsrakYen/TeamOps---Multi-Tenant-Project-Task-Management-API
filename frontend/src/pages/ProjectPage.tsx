import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { api, apiMessage } from "../lib/api";
import type { Envelope, Project, Task, TaskStatus } from "../types";
const columns: { status: TaskStatus; label: string }[] = [
  { status: "TODO", label: "To do" },
  { status: "IN_PROGRESS", label: "In progress" },
  { status: "DONE", label: "Done" },
  { status: "CANCELLED", label: "Cancelled" },
];
export function ProjectPage() {
  const { projectId = "" } = useParams();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("MEDIUM");
  const project = useQuery({
    queryKey: ["project", projectId],
    queryFn: async () =>
      (await api.get<Envelope<Project>>(`/projects/${projectId}`)).data.data,
  });
  const tasks = useQuery({
    queryKey: ["tasks", projectId],
    queryFn: async () =>
      (
        await api.get<Envelope<Task[]>>(
          `/projects/${projectId}/tasks?pageSize=100&sort=updatedAt`,
        )
      ).data.data,
  });
  const create = useMutation({
    mutationFn: () =>
      api.post(`/projects/${projectId}/tasks`, {
        title,
        description,
        priority,
        status: "TODO",
      }),
    onSuccess: async () => {
      setOpen(false);
      setTitle("");
      setDescription("");
      await qc.invalidateQueries({ queryKey: ["tasks", projectId] });
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
          <Link
            className="breadcrumb"
            to={
              project.data
                ? `/organizations/${project.data.organizationId}`
                : "/organizations"
            }
          >
            Projects /
          </Link>
          <h1>{project.data?.name ?? "Loading…"}</h1>
          <p>{project.data?.description}</p>
        </div>
        <button className="primary compact" onClick={() => setOpen(true)}>
          + Add task
        </button>
      </header>
      {tasks.error && (
        <div className="error-banner">{apiMessage(tasks.error)}</div>
      )}
      <div className="board">
        {columns.map((col) => (
          <section className="board-column" key={col.status}>
            <header>
              <h2>{col.label}</h2>
              <span>
                {tasks.data?.filter((t) => t.status === col.status).length ?? 0}
              </span>
            </header>
            <div className="task-stack">
              {tasks.data
                ?.filter((t) => t.status === col.status)
                .map((task) => (
                  <article className="task-card" key={task.id}>
                    <div className={`priority ${task.priority.toLowerCase()}`}>
                      {task.priority}
                    </div>
                    <h3>{task.title}</h3>
                    <p>{task.description || "No description"}</p>
                    <footer>
                      <span>v{task.version}</span>
                      {task.dueAt && (
                        <time>{new Date(task.dueAt).toLocaleDateString()}</time>
                      )}
                    </footer>
                  </article>
                ))}
            </div>
          </section>
        ))}
      </div>
      {open && (
        <div className="modal-backdrop" onMouseDown={() => setOpen(false)}>
          <form
            className="modal"
            onMouseDown={(e) => e.stopPropagation()}
            onSubmit={submit}
          >
            <h2>Add a task</h2>
            {create.error && (
              <div className="error-banner">{apiMessage(create.error)}</div>
            )}
            <label>
              Title
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
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
            <label>
              Priority
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
              >
                <option>LOW</option>
                <option>MEDIUM</option>
                <option>HIGH</option>
                <option>URGENT</option>
              </select>
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setOpen(false)}
              >
                Cancel
              </button>
              <button className="primary">Add task</button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
