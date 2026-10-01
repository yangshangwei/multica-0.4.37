"use client";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import { useAdminAccess } from "./use-admin-access";

function assertScope(scope: AdminScope) {
  if (scope.apiScope !== adminApiScope() || scope.userId !== useAuthStore.getState().user?.id) throw new Error("Admin session changed");
}
function scopedRead<T extends { scope: string }>(scope: AdminScope, resource: string, params: URLSearchParams,
  read: (params: URLSearchParams, signal: AbortSignal) => Promise<T | null>) {
  const filters = new URLSearchParams(params); filters.sort();
  return queryOptions({
    queryKey: adminKeys.resource(scope, resource, { query: filters.toString() }),
    queryFn: async ({ signal }) => {
      assertScope(scope);
      const result = await read(filters, signal);
      assertScope(scope);
      if (!result || result.scope !== scope.organizationId) throw new AdminUnsupportedError();
      return result;
    },
    retry: false, staleTime: 0, gcTime: 0, refetchInterval: 15_000, refetchIntervalInBackground: false,
  });
}
export function adminOverviewOptions(scope: AdminScope, params: URLSearchParams) {
  return scopedRead(scope, "overview", params, (filters, signal) => getApi().getAdminOverview(filters, { signal }));
}
export function adminAlertsOptions(scope: AdminScope, params: URLSearchParams) {
  return scopedRead(scope, "alerts", params, (filters, signal) => getApi().getAdminAlerts(filters, { signal }));
}
export function adminWorkspacesOptions(scope: AdminScope, params: URLSearchParams) {
  return scopedRead(scope, "workspaces", params, (filters, signal) => getApi().getAdminWorkspaces(filters, { signal }));
}
export function adminAuditOptions(scope: AdminScope, params: URLSearchParams) {
  return scopedRead(scope, "audit", params, (filters, signal) => getApi().getAdminAudit(filters, { signal }));
}
export function adminHealthOptions(scope: AdminScope) {
  return scopedRead(scope, "health", new URLSearchParams(), (_filters, signal) => getApi().getAdminHealth({ signal }));
}
export function adminSettingsOptions(scope: AdminScope) {
  return scopedRead(scope, "settings", new URLSearchParams(), (_filters, signal) => getApi().getAdminSettings({ signal }));
}
export function adminAlertOptions(scope: AdminScope, id: string) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "alert", { id }),
    queryFn: async ({ signal }) => {
      assertScope(scope);
      const result = await getApi().getAdminAlert(id, { signal });
      assertScope(scope);
      if (!result || result.id !== id || result.organizationId !== scope.organizationId) throw new AdminUnsupportedError();
      return result;
    }, retry: false, staleTime: 0, gcTime: 0, refetchInterval: 5_000, refetchIntervalInBackground: false,
  });
}
export function useAdminObservationScope() {
  const access = useAdminAccess();
  return { scope: { apiScope: adminApiScope(), userId: access.identity?.userId ?? "", organizationId: access.identity?.organizationId ?? null }, enabled: access.status === "ready", identity: access.identity };
}
export function useAdminOverview(params: URLSearchParams) { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminOverviewOptions(scope, params), enabled }); }
export function useAdminAlerts(params: URLSearchParams) { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminAlertsOptions(scope, params), enabled }); }
export function useAdminWorkspaces(params: URLSearchParams) { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminWorkspacesOptions(scope, params), enabled }); }
export function useAdminAudit(params: URLSearchParams) { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminAuditOptions(scope, params), enabled }); }
export function useAdminHealth() { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminHealthOptions(scope), enabled }); }
export function useAdminSettings() { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminSettingsOptions(scope), enabled }); }
export function useAdminAlert(id: string) { const { scope, enabled } = useAdminObservationScope(); return useQuery({ ...adminAlertOptions(scope, id), enabled: enabled && !!id }); }
