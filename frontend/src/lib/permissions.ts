import type { Role } from "../types";

export const canManageOrganization = (role?: Role) =>
  role === "OWNER" || role === "ADMIN";
export const canDeleteOrganization = (role?: Role) => role === "OWNER";
export const canManageMembers = canManageOrganization;
export const canManageProjects = canManageOrganization;
export const canCreateOrUpdateTasks = (role?: Role) =>
  role === "OWNER" || role === "ADMIN" || role === "MEMBER";
export const canDeleteTasks = canManageOrganization;
export const canReadAudit = canManageOrganization;
