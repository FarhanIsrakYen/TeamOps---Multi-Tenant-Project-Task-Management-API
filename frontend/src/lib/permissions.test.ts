import { describe, expect, it } from "vitest";
import {
  canCreateOrUpdateTasks,
  canDeleteOrganization,
  canDeleteTasks,
  canManageMembers,
  canReadAudit,
} from "./permissions";
import type { Role } from "../types";

describe("role-based UI hints", () => {
  it.each<[Role, boolean, boolean, boolean]>([
    ["OWNER", true, true, true],
    ["ADMIN", true, false, true],
    ["MEMBER", false, false, true],
    ["VIEWER", false, false, false],
  ])(
    "maps %s without treating UI checks as API authorization",
    (role, manages, deletesOrganization, editsTasks) => {
      expect(canManageMembers(role)).toBe(manages);
      expect(canReadAudit(role)).toBe(manages);
      expect(canDeleteTasks(role)).toBe(manages);
      expect(canDeleteOrganization(role)).toBe(deletesOrganization);
      expect(canCreateOrUpdateTasks(role)).toBe(editsTasks);
    },
  );
});
