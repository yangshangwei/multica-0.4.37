"use client";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import { useAdminAccess } from "./use-admin-access";
function assertCurrentInstallationScope(scope: AdminScope) {
  if (scope.apiScope !== adminApiScope() || scope.userId !== useAuthStore.getState().user?.id)
    throw new Error("Admin session changed");
}
const polling = {
  retry: false as const, staleTime: 0, gcTime: 0, refetchInterval: 15000, refetchIntervalInBackground: false
};
function sortedParams(input: URLSearchParams) {
  const params = new URLSearchParams(input);
  params.sort();
  return params;
}
export function adminInstallationsOptions(scope: AdminScope, input: URLSearchParams) {
  const params = sortedParams(input);
  return queryOptions({
    queryKey: adminKeys.resource(scope, "installations", { query: params.toString() }), queryFn: async ({ signal }) => {
      const data = await getApi().getAdminInstallations(params, { signal });
      assertCurrentInstallationScope(scope);
      if (!data || data.scope !== scope.organizationId || data.items.some(item => item.organizationId !== scope.organizationId))
        throw new AdminUnsupportedError();
      return data;
    }, ...polling
  });
}
export function adminInstallationOptions(scope: AdminScope, id: string) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "installation", { id }), queryFn: async ({ signal }) => {
      const data = await getApi().getAdminInstallation(id, { signal });
      assertCurrentInstallationScope(scope);
      if (!data || data.scope !== scope.organizationId || data.installation.organizationId !== scope.organizationId || data.installation.id !== id)
        throw new AdminUnsupportedError();
      return data;
    }, ...polling, refetchInterval: 5000
  });
}
export function adminUnassociatedRuntimesOptions(scope: AdminScope, input: URLSearchParams) {
  const params = sortedParams(input);
  return queryOptions({
    queryKey: adminKeys.resource(scope, "unassociated-runtimes", { query: params.toString() }), queryFn: async ({ signal }) => {
      const data = await getApi().getAdminUnassociatedRuntimes(params, { signal });
      assertCurrentInstallationScope(scope);
      if (!data || data.scope !== scope.organizationId)
        throw new AdminUnsupportedError();
      return data;
    }, ...polling
  });
}
function useInstallationScope() {
  const access = useAdminAccess();
  return {
    scope: {
      apiScope: adminApiScope(), userId: access.identity?.userId ?? "", organizationId: access.identity?.organizationId ?? null
    }, enabled: access.status === "ready"
  };
}
export function useAdminInstallations(params: URLSearchParams) {
  const { scope, enabled } = useInstallationScope();
  return useQuery({ ...adminInstallationsOptions(scope, params), enabled });
}
export function useAdminInstallation(id: string) {
  const { scope, enabled } = useInstallationScope();
  return useQuery({ ...adminInstallationOptions(scope, id), enabled: enabled && id !== "" });
}
export function useAdminUnassociatedRuntimes(params: URLSearchParams) {
  const { scope, enabled } = useInstallationScope();
  return useQuery({ ...adminUnassociatedRuntimesOptions(scope, params), enabled });
}
