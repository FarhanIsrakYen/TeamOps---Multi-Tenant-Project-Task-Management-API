import type { Role } from "../types";

export const canManageOrganization = (role?: Role) => role === "OWNER";
export const canDeleteOrganization = (role?: Role) => role === "OWNER";
const isAdministrator = (role?: Role) => role === "OWNER" || role === "ADMIN";
export const canManageMembers = isAdministrator;
export const canManageProjects = isAdministrator;
export const canCreateOrUpdateTasks = (role?: Role) =>
  role === "OWNER" || role === "ADMIN" || role === "MEMBER";
export const canDeleteTasks = isAdministrator;
export const canReadAudit = isAdministrator;
