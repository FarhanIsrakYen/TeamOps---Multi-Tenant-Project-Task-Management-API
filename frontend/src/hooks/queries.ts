import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import type { Envelope, Organization, Project, User } from "../types";

export function useOrganization(organizationId: string) {
  return useQuery({
    queryKey: ["organization", organizationId],
    enabled: Boolean(organizationId),
    queryFn: async ({ signal }) =>
      (
        await api.get<Envelope<Organization>>(
          `/organizations/${organizationId}`,
          { signal },
        )
      ).data.data,
  });
}

export function useProject(projectId: string) {
  return useQuery({
    queryKey: ["project", projectId],
    enabled: Boolean(projectId),
    queryFn: async ({ signal }) =>
      (await api.get<Envelope<Project>>(`/projects/${projectId}`, { signal }))
        .data.data,
  });
}

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async ({ signal }) =>
      (await api.get<Envelope<User>>("/me", { signal })).data.data,
  });
}
