import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { EmptyState } from "../components/EmptyState";
import { ErrorState } from "../components/ErrorState";
import { Loading } from "../components/Loading";
import { api } from "../lib/api";
import { initials } from "../lib/format";
import type { Envelope, Organization } from "../types";

export function DashboardPage() {
  const { user } = useAuth();
  const organizations = useQuery({
    queryKey: ["organizations", { page: 1, pageSize: 6, sort: "-created_at" }],
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Organization[]>>(
          "/organizations?page=1&page_size=6&sort=-created_at",
          { signal },
        )
      ).data,
  });
  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">OVERVIEW</p>
          <h1>Welcome back, {user?.name.split(" ")[0]}</h1>
          <p>Your organizations and active workspaces at a glance.</p>
        </div>
        <Link className="button primary compact" to="/organizations">
          View all organizations
        </Link>
      </header>
      {organizations.isLoading && <Loading label="Loading your workspace" />}
      {organizations.error && (
        <ErrorState
          error={organizations.error}
          retry={() => void organizations.refetch()}
        />
      )}
      {organizations.data?.data.length === 0 && (
        <EmptyState
          title="Your workspace is ready"
          body="Create an organization to start planning projects and tasks."
          action={
            <Link className="button primary compact" to="/organizations">
              Create an organization
            </Link>
          }
        />
      )}
      {organizations.data && organizations.data.data.length > 0 && (
        <section>
          <div className="section-title">
            <div>
              <p className="eyebrow">RECENT</p>
              <h2>Organizations</h2>
            </div>
            <span>
              {organizations.data.meta?.total ?? organizations.data.data.length}{" "}
              total
            </span>
          </div>
          <div className="card-grid">
            {organizations.data.data.map((organization) => (
              <Link
                className="org-card"
                to={`/organizations/${organization.id}`}
                key={organization.id}
              >
                <div className="org-monogram">
                  {initials(organization.name)}
                </div>
                <span className="role">{organization.role}</span>
                <h3>{organization.name}</h3>
                <p>{organization.slug}</p>
                <span className="open-link">Open workspace →</span>
              </Link>
            ))}
          </div>
        </section>
      )}
    </>
  );
}
