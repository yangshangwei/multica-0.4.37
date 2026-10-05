import { create } from "zustand";
import type { QueryClient } from "@tanstack/react-query";
import { ApiError, errorCode } from "../api";
import { projectKeys } from "./queries";
import { clearProjectDescriptionDrafts, useProjectDescriptionDraftStore } from "./description-draft-store";
import { clearProjectProgressDrafts, useProjectProgressDraftStore } from "./progress-draft-store";

let sessionGeneration = 0;
export const projectSessionGeneration = () => sessionGeneration;

const accessKey = (wsId: string, projectId?: string) => JSON.stringify([wsId, projectId ?? "*"]);
const pendingDeletions = new Set<string>();
export const isProjectDeletePending = (wsId: string, projectId: string) => pendingDeletions.has(accessKey(wsId, projectId));
export function beginProjectDelete(wsId: string, projectId: string) {
  const key = accessKey(wsId, projectId); const generation = sessionGeneration;
  pendingDeletions.add(key); return () => { if (generation === sessionGeneration) pendingDeletions.delete(key); };
}
const localTextFlushers = new Map<string, Set<() => void>>();
export function registerProjectLocalTextFlush(wsId: string, projectId: string, flush: () => void) {
  const key = accessKey(wsId, projectId); const listeners = localTextFlushers.get(key) ?? new Set<() => void>();
  listeners.add(flush); localTextFlushers.set(key, listeners);
  return () => { listeners.delete(flush); if (listeners.size === 0 && localTextFlushers.get(key) === listeners) localTextFlushers.delete(key); };
}
export const useProjectAccessStore = create<{
  denied: Record<string, true>; epochs: Record<string, number>; deleted: Record<string, string[]>;
}>(() => ({ denied: {}, epochs: {}, deleted: {} }));
export const projectAccessEpoch = (wsId: string, projectId?: string) => {
  const state = useProjectAccessStore.getState();
  return `${sessionGeneration}:${state.epochs[accessKey(wsId)] ?? 0}:${state.epochs[accessKey(wsId, projectId)] ?? 0}`;
};
export function canAccessProject(wsId: string, projectId?: string, generation = sessionGeneration) {
  const state = useProjectAccessStore.getState();
  return generation === sessionGeneration && !state.denied[accessKey(wsId)] && !state.denied[accessKey(wsId, projectId)] && !state.deleted[accessKey(wsId, projectId)];
}
export function isProjectAccessLost(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 401 || (error.status === 404 && errorCode(error) === "workspace_access_denied") || (error.status === 403 && !["project_evidence_forbidden", "project_permission_denied", "project_updates_disabled"].includes(errorCode(error) ?? "")));
}
export function isProjectDeleted(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404 && errorCode(error) === "project_not_found";
}
function clearProjectCaches(qc: QueryClient, wsId: string, projectId?: string) {
  const filters = projectId ? { queryKey: projectKeys.detail(wsId, projectId) } : { predicate: (query: { queryKey: readonly unknown[] }) => query.queryKey[1] === wsId };
  void qc.cancelQueries(filters); qc.removeQueries(filters);
  qc.removeQueries({ queryKey: projectKeys.list(wsId) });
  // Mutations are not cancelled by cancelQueries. Remove exposed variables,
  // context, result and error immediately; protected request wrappers also
  // reject late completions before callbacks can refill these caches.
  for (const mutation of qc.getMutationCache().getAll()) {
    const mutationKey = mutation.options.mutationKey;
    if (mutationKey?.[0] !== "projects" || mutationKey[1] !== wsId) continue;
    mutation.state = { ...mutation.state, data: undefined, variables: undefined, context: undefined, error: null, failureReason: null };
    qc.getMutationCache().remove(mutation);
  }
}
/** Runs before draft teardown on logout, expiry and Desktop server reset. */
export function resetProjectAccessSession(qc: QueryClient) {
  sessionGeneration += 1;
  pendingDeletions.clear(); localTextFlushers.clear();
  useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  for (const mutation of qc.getMutationCache().getAll()) {
    if (mutation.options.mutationKey?.[0] !== "projects") continue;
    mutation.state = { ...mutation.state, data: undefined, variables: undefined, context: undefined, error: null, failureReason: null };
    qc.getMutationCache().remove(mutation);
  }
}
export function clearProtectedProjectContent(qc: QueryClient, wsId: string, projectId?: string) {
  const key = accessKey(wsId, projectId);
  useProjectAccessStore.setState((state) => ({
    denied: { ...state.denied, [key]: true }, epochs: { ...state.epochs, [key]: (state.epochs[key] ?? 0) + 1 },
    deleted: Object.fromEntries(Object.entries(state.deleted).filter(([entry]) => {
      const [workspace, project]: string[] = JSON.parse(entry);
      return workspace !== wsId || (!!projectId && project !== projectId);
    })),
  }));
  clearProjectCaches(qc, wsId, projectId);
  clearProjectProgressDrafts(wsId, projectId); clearProjectDescriptionDrafts(wsId, projectId);
}
export function markProjectDeleted(qc: QueryClient, wsId: string, projectId: string) {
  if (!canAccessProject(wsId, projectId)) { clearProjectCaches(qc, wsId, projectId); return; }
  for (const flush of localTextFlushers.get(accessKey(wsId, projectId)) ?? []) flush();
  const texts: string[] = [];
  const belongs = (key: string) => {
    try { const parts: unknown = JSON.parse(key); return Array.isArray(parts) && parts[1] === wsId && parts[2] === projectId; } catch { return false; }
  };
  for (const [key, value] of Object.entries(useProjectDescriptionDraftStore.getState().draft.entries)) if (belongs(key)) texts.push(value.body);
  for (const [key, value] of Object.entries(useProjectProgressDraftStore.getState().draft.entries)) if (belongs(key)) {
    texts.push(value.draft.body, value.draft.acceptance?.scope ?? "", value.draft.acceptance?.explanation ?? "", value.draft.correction_reason ?? "");
  }
  const key = accessKey(wsId, projectId);
  useProjectAccessStore.setState((state) => ({ deleted: { ...state.deleted, [key]: [...new Set(texts.filter((text) => text.trim()))] }, epochs: { ...state.epochs, [key]: (state.epochs[key] ?? 0) + 1 } }));
  clearProjectCaches(qc, wsId, projectId);
  clearProjectProgressDrafts(wsId, projectId); clearProjectDescriptionDrafts(wsId, projectId);
}
export function handleProjectAccessError(qc: QueryClient, wsId: string, projectId: string | undefined, error: unknown) {
  if (isProjectAccessLost(error)) clearProtectedProjectContent(qc, wsId);
  else if (projectId && isProjectDeleted(error)) markProjectDeleted(qc, wsId, projectId);
}
function projectAccessChangedError(wsId: string, projectId?: string) {
  if (projectId && useProjectAccessStore.getState().deleted[accessKey(wsId, projectId)]) return new ApiError("Project deleted", 404, "Not Found", { code: "project_not_found" });
  return new ApiError("Project access changed", 403, "Forbidden");
}
export async function protectProjectRequest<T>(qc: QueryClient, wsId: string, projectId: string | undefined, request: () => Promise<T>): Promise<T> {
  const epoch = projectAccessEpoch(wsId, projectId);
  const generation = sessionGeneration;
  if (!canAccessProject(wsId, projectId)) throw projectAccessChangedError(wsId, projectId);
  try {
    const result = await request();
    if (generation !== sessionGeneration) throw new ApiError("Project session changed", 409, "Conflict", { code: "project_session_changed" });
    if (epoch !== projectAccessEpoch(wsId, projectId) || !canAccessProject(wsId, projectId)) throw projectAccessChangedError(wsId, projectId);
    return result;
  } catch (error) {
    if (generation !== sessionGeneration) throw new ApiError("Project session changed", 409, "Conflict", { code: "project_session_changed" });
    // A deleted project's local text remains copyable; do not reinterpret our
    // stale-result rejection as a newly discovered workspace revocation.
    if (epoch === projectAccessEpoch(wsId, projectId)) handleProjectAccessError(qc, wsId, projectId, error);
    if (error instanceof ApiError && isProjectAccessLost(error)) throw new ApiError("Project access changed", error.status, error.statusText, error.status === 404 ? { code: "workspace_access_denied" } : undefined);
    throw error;
  }
}
