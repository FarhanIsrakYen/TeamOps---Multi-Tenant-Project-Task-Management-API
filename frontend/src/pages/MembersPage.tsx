import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { ErrorState } from "../components/ErrorState";
import { Form } from "../components/Form";
import { Input } from "../components/Input";
import { Loading } from "../components/Loading";
import { Modal } from "../components/Modal";
import { OrganizationNav } from "../components/OrganizationNav";
import { Select } from "../components/Select";
import { Table, type Column } from "../components/Table";
import { useOrganization } from "../hooks/queries";
import { api, apiMessage } from "../lib/api";
import { formatDate } from "../lib/format";
import { canManageMembers } from "../lib/permissions";
import type { Envelope, Membership, Role } from "../types";

export function MembersPage() {
  const { organizationId = "" } = useParams();
  const queryClient = useQueryClient();
  const organization = useOrganization(organizationId);
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("MEMBER");
  const members = useQuery({
    queryKey: ["members", organizationId],
    enabled: Boolean(organizationId),
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Membership[]>>(
          `/organizations/${organizationId}/members`,
          { signal },
        )
      ).data.data,
  });
  const upsert = useMutation({
    mutationFn: () =>
      api.post(`/organizations/${organizationId}/members`, {
        email: email.trim().toLowerCase(),
        role,
      }),
    onSuccess: async () => {
      setOpen(false);
      setEmail("");
      setRole("MEMBER");
      await queryClient.invalidateQueries({
        queryKey: ["members", organizationId],
      });
    },
  });
  const remove = useMutation({
    mutationFn: (userId: string) =>
      api.delete(`/organizations/${organizationId}/members/${userId}`),
    onSuccess: async () =>
      queryClient.invalidateQueries({ queryKey: ["members", organizationId] }),
  });
  if (organization.isLoading || members.isLoading)
    return <Loading label="Loading members" />;
  if (organization.error || !organization.data)
    return (
      <ErrorState
        error={organization.error}
        retry={() => void organization.refetch()}
      />
    );
  if (members.error)
    return (
      <ErrorState error={members.error} retry={() => void members.refetch()} />
    );
  const manageable = canManageMembers(organization.data.role);
  const columns: Column<Membership>[] = [
    {
      key: "member",
      header: "Member",
      render: (member) => (
        <div>
          <strong>{member.user?.name ?? "Unknown user"}</strong>
          <small className="subtle">{member.user?.email}</small>
        </div>
      ),
    },
    {
      key: "role",
      header: "Role",
      render: (member) => <span className="badge static">{member.role}</span>,
    },
    {
      key: "joined",
      header: "Joined",
      render: (member) => formatDate(member.createdAt),
    },
    {
      key: "actions",
      header: "",
      className: "actions",
      render: (member) =>
        manageable && member.role !== "OWNER" ? (
          <Button
            variant="ghost"
            size="compact"
            busy={remove.isPending && remove.variables === member.userId}
            onClick={() => remove.mutate(member.userId)}
          >
            Remove
          </Button>
        ) : null,
    },
  ];
  return (
    <>
      <header className="page-header">
        <div>
          <Link className="breadcrumb" to={`/organizations/${organizationId}`}>
            {organization.data.name} /
          </Link>
          <h1>Members</h1>
          <p>
            Roles shown here are hints for the interface; the API enforces every
            permission.
          </p>
        </div>
        {manageable && (
          <Button size="compact" onClick={() => setOpen(true)}>
            + Add member
          </Button>
        )}
      </header>
      <OrganizationNav organization={organization.data} />
      {remove.error && (
        <div className="error-banner">{apiMessage(remove.error)}</div>
      )}
      {members.data?.length === 0 ? (
        <EmptyState
          title="No members"
          body="Invite a teammate by their registered email address."
        />
      ) : (
        <Table
          columns={columns}
          rows={members.data ?? []}
          rowKey={(member) => member.userId}
        />
      )}
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title="Add or update member"
        description="The user must already have a TeamOps account."
      >
        <Form
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            upsert.mutate();
          }}
          error={upsert.error ? apiMessage(upsert.error) : undefined}
        >
          <Input
            label="Email"
            name="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <Select
            label="Role"
            value={role}
            onChange={(e) => setRole(e.target.value as Role)}
            options={[
              { value: "ADMIN", label: "Admin" },
              { value: "MEMBER", label: "Member" },
              { value: "VIEWER", label: "Viewer" },
            ]}
          />
          <p className="form-note">
            Owner cannot be assigned through this form.
          </p>
          <div className="modal-actions">
            <Button
              type="button"
              variant="secondary"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button busy={upsert.isPending}>Save member</Button>
          </div>
        </Form>
      </Modal>
    </>
  );
}
