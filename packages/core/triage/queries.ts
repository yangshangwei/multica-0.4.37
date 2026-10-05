import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { TriageHistoryParams, TriageListParams } from "../types/triage";

export const triageKeys = {
  all: (wsId: string) => ["triage", wsId] as const,
  settings: (wsId: string) => [...triageKeys.all(wsId), "settings"] as const,
  counts: (wsId: string) => [...triageKeys.all(wsId), "counts"] as const,
  lists: (wsId: string) => [...triageKeys.all(wsId), "list"] as const,
  list: (wsId: string, params: TriageListParams = {}) => [...triageKeys.lists(wsId), params] as const,
  detail: (wsId: string, id: string) => [...triageKeys.all(wsId), "detail", id] as const,
  itemHistory: (wsId: string, id: string) => [...triageKeys.all(wsId), "item-history", id] as const,
  history: (wsId: string, params: TriageHistoryParams = {}) => [...triageKeys.all(wsId), "history", params] as const,
  import: (wsId: string, id: string) => [...triageKeys.all(wsId), "import", id] as const,
};

export function triageSettingsOptions(wsId: string) {
  return queryOptions({ queryKey: triageKeys.settings(wsId), queryFn: ({ signal }) => api.getTriageSettings(wsId, { signal }), enabled: wsId.length > 0, staleTime: 30_000 });
}
export function triageListOptions(wsId: string, params: TriageListParams = {}) {
  return queryOptions({
    queryKey: triageKeys.list(wsId, params), queryFn: ({ signal }) => api.listTriageItems(wsId, params, { signal }), enabled: wsId.length > 0,
    // Due snoozes are computed by the server from stored time, even after restart.
    staleTime: 15_000, refetchInterval: 30_000, refetchOnWindowFocus: true,
  });
}
export function triageCountsOptions(wsId: string) {
  return queryOptions({
    queryKey: triageKeys.counts(wsId), queryFn: async ({ signal }) => (await api.listTriageItems(wsId, { view: "ready", limit: 1 }, { signal })).counts,
    enabled: wsId.length > 0, staleTime: 15_000, refetchInterval: 30_000, refetchOnWindowFocus: true,
  });
}
export function triageDetailOptions(wsId: string, id: string) {
  return queryOptions({ queryKey: triageKeys.detail(wsId, id), queryFn: ({ signal }) => api.getTriageItem(wsId, id, { signal }), enabled: wsId.length > 0 && id.length > 0 });
}
export function triageItemHistoryOptions(wsId: string, id: string) {
  return queryOptions({ queryKey: triageKeys.itemHistory(wsId, id), queryFn: ({ signal }) => api.getTriageItemHistory(wsId, id, { signal }), enabled: wsId.length > 0 && id.length > 0 });
}
export function triageHistoryOptions(wsId: string, params: TriageHistoryParams = {}) {
  return queryOptions({ queryKey: triageKeys.history(wsId, params), queryFn: ({ signal }) => api.listTriageHistory(wsId, params, { signal }), enabled: wsId.length > 0 });
}
export function triageImportOptions(wsId: string, id: string) {
  return queryOptions({ queryKey: triageKeys.import(wsId, id), queryFn: ({ signal }) => api.getTriageImport(wsId, id, { signal }), enabled: wsId.length > 0 && id.length > 0 });
}
