import type { QueryClient, QueryKey } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import { useAuthStore } from "../auth";
import type { AccountView } from "../paths/account";
import { clearCollaborationDrafts, retainCollaborationDraftScope } from "./draft";

export interface CollaborationScope { server: string; apiScope: string; userId: string }
const instances = new WeakMap<object, number>();
let nextInstance = 0;

export function collaborationScope(userId: string): CollaborationScope {
  const api = getApi();
  let instance = instances.get(api);
  if (instance === undefined) { instance = ++nextInstance; instances.set(api, instance); }
  const server = api.getBaseUrl();
  return { server, userId, apiScope: JSON.stringify([server, instance, api.getSessionScope()]) };
}

export const collaborationKeys = {
  scope: (scope: CollaborationScope) => ["collaboration", scope.apiScope, scope.userId] as const,
  issue: (scope: CollaborationScope, issueId: string) => [...collaborationKeys.scope(scope), "issue", issueId.toLowerCase()] as const,
  workbench: (scope: CollaborationScope, view: AccountView) => [...collaborationKeys.scope(scope), "workbench", view] as const,
  notifications: (scope: CollaborationScope) => [...collaborationKeys.scope(scope), "notifications"] as const,
};

interface Access { epoch: number; denied: boolean; denialError: ApiError | null }
const access = new WeakMap<QueryClient, Map<string, Access>>();
const accessKey = (scope: CollaborationScope, issueId: string | null) => JSON.stringify([...collaborationKeys.scope(scope), issueId?.toLowerCase() ?? null]);
function state(client: QueryClient, scope: CollaborationScope, issueId: string | null): Access {
  let entries = access.get(client);
  if (!entries) { entries = new Map(); access.set(client, entries); }
  const key = accessKey(scope, issueId);
  let current = entries.get(key);
  if (!current) { current = { epoch: 0, denied: false, denialError: null }; entries.set(key, current); }
  return current;
}

export function collaborationAccessEpoch(client: QueryClient, scope: CollaborationScope, issueId: string): number {
  return state(client, scope, issueId).epoch;
}
export function collaborationIssueAccessDenied(client: QueryClient, scope: CollaborationScope, issueId: string): boolean {
  return state(client, scope, issueId).denied;
}
export function isCollaborationScopeCurrent(scope: CollaborationScope): boolean {
  return useAuthStore.getState().user?.id === scope.userId && collaborationScope(scope.userId).apiScope === scope.apiScope;
}
export function isCollaborationDenied(error: unknown): boolean {
  return error instanceof ApiError && !error.isStaleAuthResponse && [401, 403, 404, 410].includes(error.status);
}
export class CollaborationUnavailableError extends Error {
  constructor() { super("Collaboration response is unavailable"); this.name = "CollaborationUnavailableError"; }
}
const superseded = () => new DOMException("Collaboration access changed", "AbortError");
const denied = () => new ApiError("Collaboration access denied", 403, "Forbidden");
function ownedKey(key: QueryKey | undefined, scope: CollaborationScope): boolean {
  return key?.[0] === "collaboration" && key[1] === scope.apiScope && key[2] === scope.userId;
}

/** Advance the fence before cancellation so neither fetch nor rollback restores private data. */
export function clearCollaborationAccess(client: QueryClient, scope: CollaborationScope, issueId: string): void {
  const current = state(client, scope, issueId);
  current.epoch += 1;
  current.denied = true;
  current.denialError = denied();
  state(client, scope, null).epoch += 1;
  clearCollaborationDrafts(undefined, { ...scope, issueId });
  const predicate = (entry: { queryKey: QueryKey }) => ownedKey(entry.queryKey, scope) &&
    (entry.queryKey[3] !== "issue" || entry.queryKey[4] === issueId.toLowerCase());
  void client.cancelQueries({ predicate }, { revert: false });
  for (const query of client.getQueryCache().findAll({ predicate })) {
    query.setData(undefined, { manual: true, updatedAt: 0 });
    query.setState({ data: undefined, dataUpdatedAt: 0, error: current.denialError, status: "error" });
  }
  for (const mutation of client.getMutationCache().getAll()) {
    const key = mutation.options.mutationKey;
    if (!ownedKey(key, scope) || key?.[4] !== issueId.toLowerCase()) continue;
    mutation.state = { ...mutation.state, data: undefined, variables: undefined, context: undefined };
    client.getMutationCache().remove(mutation);
  }
}

/** Source membership changed, but independent collaboration access may survive.
 * Hide the old projection and fence its responses without deleting original
 * command drafts. An authoritative failed re-read performs the final cleanup.
 */
export function clearCollaborationProjection(client: QueryClient, scope: CollaborationScope, issueId: string): void {
  state(client, scope, issueId).epoch += 1;
  const queryKey = collaborationKeys.issue(scope, issueId);
  void client.cancelQueries({ queryKey }, { revert: false });
  for (const query of client.getQueryCache().findAll({ queryKey })) {
    query.setState({ data: undefined, dataUpdatedAt: 0, error: null, status: "pending" });
  }
  for (const mutation of client.getMutationCache().getAll()) {
    const key = mutation.options.mutationKey;
    if (!ownedKey(key, scope) || key?.[4] !== issueId.toLowerCase()) continue;
    mutation.state = { ...mutation.state, data: undefined, variables: undefined, context: undefined };
    client.getMutationCache().remove(mutation);
  }
}

export async function protectCollaborationRead<T>(
  client: QueryClient, scope: CollaborationScope, issueId: string | null,
  request: () => Promise<T | null>, allowRecovery = false,
): Promise<T> {
  if (!isCollaborationScopeCurrent(scope)) throw superseded();
  const before = { ...state(client, scope, issueId) };
  if (before.denied && !allowRecovery) throw before.denialError ?? denied();
  try {
    const value = await request();
    if (!isCollaborationScopeCurrent(scope) || state(client, scope, issueId).epoch !== before.epoch) throw superseded();
    if (value === null) throw new CollaborationUnavailableError();
    if (allowRecovery) {
      const current = state(client, scope, issueId);
      current.denied = false;
      current.denialError = null;
      if (before.denialError) {
        // Clear only this denial's propagated errors, including disabled reads.
        // Query reset() could restore data from before access was revoked.
        for (const query of client.getQueryCache().findAll({ predicate: (entry) => ownedKey(entry.queryKey, scope) && entry.state.error === before.denialError })) {
          query.setState({ data: undefined, dataUpdatedAt: 0, error: null, errorUpdatedAt: 0, status: "pending", fetchFailureCount: 0, fetchFailureReason: null });
        }
      }
    }
    return value;
  } catch (error) {
    if (!isCollaborationScopeCurrent(scope) || state(client, scope, issueId).epoch !== before.epoch) throw superseded();
    if (issueId && isCollaborationDenied(error) && !state(client, scope, issueId).denied) clearCollaborationAccess(client, scope, issueId);
    throw error;
  }
}

/** Session changes remove both cached data and retained mutation variables. */
export function retainCollaborationScope(client: QueryClient, scope: CollaborationScope): void {
  retainCollaborationDraftScope(scope);
  const stale = (key: QueryKey | undefined) => key?.[0] === "collaboration" && !ownedKey(key, scope);
  const predicate = (entry: { queryKey: QueryKey }) => stale(entry.queryKey);
  void client.cancelQueries({ predicate }, { revert: false });
  client.removeQueries({ predicate });
  for (const mutation of client.getMutationCache().getAll()) {
    if (stale(mutation.options.mutationKey)) {
      mutation.state = { ...mutation.state, data: undefined, variables: undefined, context: undefined };
      client.getMutationCache().remove(mutation);
    }
  }
}
