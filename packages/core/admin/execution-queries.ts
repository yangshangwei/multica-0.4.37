"use client";

import { queryOptions, useQuery } from "@tanstack/react-query";
import { getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import { useAdminAccess } from "./use-admin-access";
function assertCurrentScope(scope: AdminScope) {
  if (scope.apiScope !== adminApiScope() || useAuthStore.getState().user?.id !== scope.userId)
    throw new Error("Admin session changed");
}
function normalizedParams(params: URLSearchParams) {
  const copy = new URLSearchParams(params);
  copy.sort();
  return copy;
}
const polling = {
  staleTime: 0, gcTime: 0, retry: false as const, refetchInterval: 15000, refetchIntervalInBackground: false
};
export function adminExecutionListOptions(scope: AdminScope, params: URLSearchParams) {
  const filters = normalizedParams(params);
  return queryOptions({
    queryKey: adminKeys.resource(scope, "tasks", { query: filters.toString() }), queryFn: async ({ signal }) => {
      const data = await getApi().getAdminTasks(filters, { signal });
      assertCurrentScope(scope);
      if (!data || data.scope !== scope.organizationId)
        throw new AdminUnsupportedError();
      return data;
    }, ...polling
  });
}
export function adminIssueListOptions(scope: AdminScope, params: URLSearchParams) {
  const filters = normalizedParams(params);
  return queryOptions({
    queryKey: adminKeys.resource(scope, "issues", { query: filters.toString() }), queryFn: async ({ signal }) => {
      const data = await getApi().getAdminIssues(filters, { signal });
      assertCurrentScope(scope);
      if (!data || data.scope !== scope.organizationId)
        throw new AdminUnsupportedError();
      return data;
    }, ...polling
  });
}
export function adminExecutionOptions(scope: AdminScope, id: string) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "task", { id }), queryFn: async ({ signal }) => {
      const data = await getApi().getAdminTask(id, { signal });
      assertCurrentScope(scope);
      if (!data || data.id !== id)
        throw new AdminUnsupportedError();
      return data;
    }, ...polling, refetchInterval: 5000
  });
}
function useExecutionScope() {
  const access = useAdminAccess();
  const userId = useAuthStore((s) => s.user?.id ?? "");
  return {
    scope: {
      apiScope: adminApiScope(), userId, organizationId: access.identity?.organizationId ?? null
    }, enabled: access.status === "ready"
  };
}
export function useAdminExecutions(params: URLSearchParams) {
  const { scope, enabled } = useExecutionScope();
  return useQuery({ ...adminExecutionListOptions(scope, params), enabled });
}
export function useAdminIssues(params: URLSearchParams) {
  const { scope, enabled } = useExecutionScope();
  return useQuery({ ...adminIssueListOptions(scope, params), enabled });
}
export function useAdminExecution(id: string) {
  const { scope, enabled } = useExecutionScope();
  return useQuery({ ...adminExecutionOptions(scope, id), enabled: enabled && id !== "" });
}
