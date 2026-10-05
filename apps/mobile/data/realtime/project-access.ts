import type { QueryClient } from "@tanstack/react-query";

// Access epochs contain no server data. They prevent already-running HTTP/WS
// work from restoring protected data after membership is revoked.
const epochs = new Map<string, number>();
const denied = new Set<string>();
const listeners = new Set<(workspaceId: string, status: 401 | 403) => void>();
const cacheCleanups = new Set<(workspaceId: string) => void>();

export const projectAccessEpoch = (workspaceId: string) => epochs.get(workspaceId) ?? 0;
export const isProjectAccessDenied = (workspaceId: string) => denied.has(workspaceId);
export const allowProjectAccess = (workspaceId: string) => denied.delete(workspaceId);

export function markProjectAccessDenied(workspaceId: string, status: 401 | 403 = 403) {
  const alreadyDenied = denied.has(workspaceId);
  epochs.set(workspaceId, projectAccessEpoch(workspaceId) + 1);
  denied.add(workspaceId);
  for (const cleanup of cacheCleanups) cleanup(workspaceId);
  if (alreadyDenied) return;
  for (const listener of listeners) {
    try { listener(workspaceId, status); }
    catch { console.warn("[projects] Access revoked; navigation unavailable"); }
  }
}

export function onProjectAccessDenied(listener: (workspaceId: string, status: 401 | 403) => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

function clearWorkspaceQueries(qc: QueryClient, workspaceId: string) {
  const filters = { predicate: (query: { queryKey: readonly unknown[] }) => query.queryKey[1] === workspaceId };
  // Both operations begin synchronously, before any navigation/storage await.
  void qc.cancelQueries(filters);
  qc.removeQueries(filters);
}

export function revokeProjectAccess(qc: QueryClient, workspaceId: string) {
  markProjectAccessDenied(workspaceId);
  clearWorkspaceQueries(qc, workspaceId);
}

export function observeProjectAccess(qc: QueryClient) {
  const clear = (workspaceId: string) => clearWorkspaceQueries(qc, workspaceId);
  cacheCleanups.add(clear);
  return () => { cacheCleanups.delete(clear); };
}

export function isProjectAccessError(error: unknown): boolean {
  if (typeof error !== "object" || error === null || !("status" in error)) return false;
  if (error.status === 401) return true;
  if (error.status !== 403) return false;
  // ApiError preserves the server code in body. Losing permission for an
  // operation or one evidence source does not revoke workspace membership.
  // Mirrors core/projects/access without importing the Web hook/store layer.
  const body = "body" in error ? error.body : undefined;
  const code = typeof body === "object" && body !== null && "code" in body ? body.code : undefined;
  return code !== "project_permission_denied" && code !== "project_evidence_forbidden" && code !== "project_updates_disabled";
}

export async function protectProjectQuery<T>(qc: QueryClient, workspaceId: string | null, read: () => Promise<T>): Promise<T> {
  if (!workspaceId) throw new Error("A workspace is required");
  const epoch = projectAccessEpoch(workspaceId);
  try {
    const value = await read();
    if (epoch !== projectAccessEpoch(workspaceId)) throw Object.assign(new Error("Project access changed"), { status: 403 });
    return value;
  } catch (error) {
    if (isProjectAccessError(error)) revokeProjectAccess(qc, workspaceId);
    throw error;
  }
}
