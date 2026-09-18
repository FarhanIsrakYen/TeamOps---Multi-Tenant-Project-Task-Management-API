export type Role = "OWNER" | "ADMIN" | "MEMBER" | "VIEWER";
export interface User {
  id: string;
  email: string;
  name: string;
  createdAt: string;
}
export interface Organization {
  id: string;
  name: string;
  slug: string;
  role: Role;
  version: number;
  createdAt: string;
}
export interface Project {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  archived: boolean;
  version: number;
  updatedAt: string;
}
export type TaskStatus =
  "TODO" | "IN_PROGRESS" | "IN_REVIEW" | "DONE" | "CANCELLED";
export interface Task {
  id: string;
  projectId: string;
  title: string;
  description: string;
  status: TaskStatus;
  priority: "LOW" | "MEDIUM" | "HIGH" | "URGENT";
  assigneeId?: string;
  version: number;
  dueAt?: string;
  updatedAt: string;
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
