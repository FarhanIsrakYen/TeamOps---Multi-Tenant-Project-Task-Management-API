export type Role = "OWNER" | "ADMIN" | "MEMBER" | "VIEWER";
export type TaskStatus =
  "TODO" | "IN_PROGRESS" | "IN_REVIEW" | "DONE" | "CANCELLED";
export type TaskPriority = "LOW" | "MEDIUM" | "HIGH" | "URGENT";

export interface User {
  id: string;
  email: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  createdBy: string;
  role?: Role;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface Membership {
  organizationId: string;
  userId: string;
  role: Role;
  user?: User;
  createdAt: string;
}

export interface Project {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  archived: boolean;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface Task {
  id: string;
  organizationId: string;
  projectId: string;
  assigneeId?: string;
  createdBy: string;
  title: string;
  description: string;
  status: TaskStatus;
  priority: TaskPriority;
  version: number;
  dueAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Comment {
  id: string;
  taskId: string;
  authorId: string;
  body: string;
  createdAt: string;
  updatedAt: string;
}

export interface Label {
  id: string;
  organizationId: string;
  name: string;
  color: string;
}

export interface AuditLog {
  id: number;
  organizationId?: string;
  actorUserId?: string;
  action: string;
  resourceType: string;
  resourceId: string;
  metadata: Record<string, unknown>;
  requestId: string;
  ipAddress?: string;
  userAgent?: string;
  createdAt: string;
}

export interface PageMeta {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
}

export interface Envelope<T> {
  data: T;
  meta?: PageMeta;
}

export interface AuthTokens {
  accessToken: string;
  accessTokenExpiresAt: string;
  refreshToken: string;
  user: User;
}

export interface ApiErrorBody {
  code: string;
  message: string;
  requestId?: string;
}
