import type { QueryClient } from "@tanstack/react-query";
import { ApiError } from "../api";
import type { StorageAdapter } from "../types/storage";
import { collaborationAccessEpoch, collaborationKeys, isCollaborationDenied, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationCreationDrafts, clearCollaborationDeliveryDrafts, readCollaborationDeliveryRecords, type CollaborationDeliveryContext } from "./delivery-draft";

export function collaborationDeliveryKey(scope: CollaborationScope, context: CollaborationDeliveryContext) {
  return context.kind === "issue" ? collaborationKeys.issue(scope, context.issueId)
    : [...collaborationKeys.scope(scope), "creation", context.sourceWorkspaceId.toLowerCase()] as const;
}
type SourceAccess = { epoch: number; denied: boolean };
const states = new WeakMap<QueryClient, Map<string, SourceAccess>>();
function sourceState(client: QueryClient, scope: CollaborationScope, sourceId: string) {
  let entries = states.get(client);
  if (!entries) { entries = new Map(); states.set(client, entries); }
  const key = JSON.stringify([...collaborationKeys.scope(scope), sourceId.toLowerCase()]);
  let state = entries.get(key);
  if (!state) { state = { epoch: 0, denied: false }; entries.set(key, state); }
  return state;
}
export function clearCollaborationSourceAccess(client: QueryClient, scope: CollaborationScope, sourceId: string, storage?: StorageAdapter) {
  const state = sourceState(client, scope, sourceId);
  state.epoch += 1;
  state.denied = true;
  const queryKey = collaborationDeliveryKey(scope, { kind: "creation", sourceWorkspaceId: sourceId });
  void client.cancelQueries({ queryKey }, { revert: false });
  for (const query of client.getQueryCache().findAll({ queryKey })) {
    query.setState({ data: undefined, dataUpdatedAt: 0, status: "error", error: new ApiError("Source access denied", 403, "Forbidden") });
  }
  for (const mutation of client.getMutationCache().getAll()) {
    if (!queryKey.every((part, index) => mutation.options.mutationKey?.[index] === part)) continue;
    mutation.state = { ...mutation.state, data: undefined, variables: undefined, context: undefined };
    client.getMutationCache().remove(mutation);
  }
  clearCollaborationCreationDrafts(scope, sourceId, storage);
}

export function reconcileCollaborationCreationSources(client: QueryClient, scope: CollaborationScope, sourceIds: string[], storage?: StorageAdapter) {
  const valid = new Set(sourceIds.map((id) => id.toLowerCase()));
  const known = new Set(readCollaborationDeliveryRecords(scope, storage).flatMap((record) => record.scope.context.kind === "creation" ? [record.scope.context.sourceWorkspaceId] : []));
  for (const query of client.getQueryCache().findAll({ queryKey: collaborationKeys.scope(scope) })) {
    if (query.queryKey[3] === "creation" && typeof query.queryKey[4] === "string") known.add(query.queryKey[4]);
  }
  for (const id of known) if (!valid.has(id) && !sourceState(client, scope, id).denied) clearCollaborationSourceAccess(client, scope, id, storage);
}

export async function protectCollaborationDelivery<T>(client: QueryClient, scope: CollaborationScope, context: CollaborationDeliveryContext,
  request: (assertCurrent: () => void) => Promise<T | null>, allowRecovery = false, storage?: StorageAdapter): Promise<T> {
  if (context.kind === "issue") {
    const epoch = collaborationAccessEpoch(client, scope, context.issueId);
    const assertCurrent = () => {
      if (!isCollaborationScopeCurrent(scope) || collaborationAccessEpoch(client, scope, context.issueId) !== epoch) {
        throw new DOMException("Collaboration access changed", "AbortError");
      }
    };
    try { return await protectCollaborationRead(client, scope, context.issueId, () => request(assertCurrent), allowRecovery); }
    catch (error) {
      if (isCollaborationDenied(error)) clearCollaborationDeliveryDrafts(storage, { ...scope, issueId: context.issueId });
      throw error;
    }
  }
  const state = sourceState(client, scope, context.sourceWorkspaceId);
  const epoch = state.epoch;
  const assertCurrent = () => {
    if (!isCollaborationScopeCurrent(scope) || state.epoch !== epoch) throw new DOMException("Collaboration source changed", "AbortError");
  };
  if (state.denied && !allowRecovery) throw new ApiError("Source access denied", 403, "Forbidden");
  try {
    const value = await protectCollaborationRead(client, scope, null, () => request(assertCurrent));
    assertCurrent();
    if (allowRecovery) state.denied = false;
    return value;
  } catch (error) {
    assertCurrent();
    if (isCollaborationDenied(error)) clearCollaborationSourceAccess(client, scope, context.sourceWorkspaceId, storage);
    throw error;
  }
}
