import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { EmptyState } from "../components/EmptyState";
import { ErrorState } from "../components/ErrorState";
import { Input } from "../components/Input";
import { Loading } from "../components/Loading";
import { OrganizationNav } from "../components/OrganizationNav";
import { Pagination } from "../components/Pagination";
import { Select } from "../components/Select";
import { Table, type Column } from "../components/Table";
import { useOrganization } from "../hooks/queries";
import { api, queryString } from "../lib/api";
import { formatDate } from "../lib/format";
import type { AuditLog, Envelope } from "../types";

export function AuditLogsPage() {
  const { organizationId = "" } = useParams();
  const organization = useOrganization(organizationId);
  const [page, setPage] = useState(1);
  const [actor, setActor] = useState("");
  const [action, setAction] = useState("");
  const [resourceType, setResourceType] = useState("");
  const [resourceId, setResourceId] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [sort, setSort] = useState("-created_at");
  const logs = useQuery({
    queryKey: [
      "audit-logs",
      organizationId,
      { page, actor, action, resourceType, resourceId, from, to, sort },
    ],
    enabled: Boolean(organizationId),
    placeholderData: keepPreviousData,
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<AuditLog[]>>(
          `/organizations/${organizationId}/audit-logs${queryString({ page, page_size: 20, actor_id: actor, action, resource_type: resourceType, resource_id: resourceId, from: from ? new Date(`${from}T00:00:00`).toISOString() : "", to: to ? new Date(`${to}T23:59:59`).toISOString() : "", sort })}`,
          { signal },
        )
      ).data,
  });
  if (organization.isLoading) return <Loading label="Loading audit log" />;
  if (organization.error || !organization.data)
    return (
      <ErrorState
        error={organization.error}
        retry={() => void organization.refetch()}
      />
    );
  const reset = () => setPage(1);
  const columns: Column<AuditLog>[] = [
    { key: "time", header: "Time", render: (log) => formatDate(log.createdAt) },
    {
      key: "action",
      header: "Action",
      render: (log) => <strong>{log.action}</strong>,
    },
    {
      key: "actor",
      header: "Actor",
      render: (log) => <code>{log.actorUserId?.slice(0, 8) ?? "system"}</code>,
    },
    {
      key: "resource",
      header: "Resource",
      render: (log) => (
        <div>
          <span>{log.resourceType}</span>
          <small className="subtle">
            <code>{log.resourceId}</code>
          </small>
        </div>
      ),
    },
    {
      key: "request",
      header: "Request",
      render: (log) => (
        <code title={log.requestId}>{log.requestId?.slice(0, 8) || "—"}</code>
      ),
    },
  ];
  return (
    <>
      <header className="page-header">
        <div>
          <Link className="breadcrumb" to={`/organizations/${organizationId}`}>
            {organization.data.name} /
          </Link>
          <h1>Audit logs</h1>
          <p>
            Security and business-sensitive activity. Available to owners and
            admins.
          </p>
        </div>
      </header>
      <OrganizationNav organization={organization.data} />
      <div className="toolbar filters audit-filters">
        <Input
          label="Actor ID"
          value={actor}
          onChange={(e) => {
            setActor(e.target.value);
            reset();
          }}
          placeholder="UUID"
        />
        <Input
          label="Action"
          value={action}
          onChange={(e) => {
            setAction(e.target.value);
            reset();
          }}
          placeholder="task.status_changed"
        />
        <Input
          label="Resource type"
          value={resourceType}
          onChange={(e) => {
            setResourceType(e.target.value);
            reset();
          }}
        />
        <Input
          label="Resource ID"
          value={resourceId}
          onChange={(e) => {
            setResourceId(e.target.value);
            reset();
          }}
        />
        <Input
          label="From"
          type="date"
          value={from}
          onChange={(e) => {
            setFrom(e.target.value);
            reset();
          }}
        />
        <Input
          label="To"
          type="date"
          value={to}
          onChange={(e) => {
            setTo(e.target.value);
            reset();
          }}
        />
        <Select
          label="Sort"
          value={sort}
          onChange={(e) => {
            setSort(e.target.value);
            reset();
          }}
          options={[
            { value: "-created_at", label: "Newest" },
            { value: "created_at", label: "Oldest" },
          ]}
        />
      </div>
      {logs.isLoading && <Loading label="Loading audit events" />}
      {logs.error && (
        <ErrorState error={logs.error} retry={() => void logs.refetch()} />
      )}
      {logs.data?.data.length === 0 && (
        <EmptyState
          title="No audit events found"
          body="No events match the current filters."
        />
      )}
      {logs.data && logs.data.data.length > 0 && (
        <Table
          columns={columns}
          rows={logs.data.data}
          rowKey={(log) => log.id}
        />
      )}
      {logs.data?.meta && (
        <Pagination {...logs.data.meta} onPageChange={setPage} />
      )}
    </>
  );
}
