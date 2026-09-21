import { NavLink } from "react-router-dom";
import { canReadAudit } from "../lib/permissions";
import type { Organization } from "../types";

export function OrganizationNav({
  organization,
}: {
  organization: Organization;
}) {
  const base = `/organizations/${organization.id}`;
  return (
    <nav className="tabs" aria-label="Organization sections">
      <NavLink end to={base}>
        Overview
      </NavLink>
      <NavLink to={`${base}/members`}>Members</NavLink>
      <NavLink to={`${base}/projects`}>Projects</NavLink>
      {canReadAudit(organization.role) && (
        <NavLink to={`${base}/audit-logs`}>Audit log</NavLink>
      )}
    </nav>
  );
}
