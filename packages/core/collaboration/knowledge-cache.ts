import { focusManager, onlineManager, type QueryClient, type QueryKey } from "@tanstack/react-query";
import { collaborationKeys, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { KnowledgeProjectionPolicy, knowledgeProjectionExpired, type KnowledgeProjectionBoundary } from "./knowledge-lease";
import type { CollaborationKnowledgeProjection } from "./knowledge-schema";

export interface AuthorizedKnowledgeProjection<T> { value: T; boundary: KnowledgeProjectionBoundary }
interface ProjectionState {
  policy: KnowledgeProjectionPolicy;
  timers: Map<string, ReturnType<typeof setTimeout>>;
  lifecycle?: { count: number; dispose: () => void };
}
const scopes = new WeakMap<QueryClient, Map<string, ProjectionState>>();
export const knowledgeScopeKey = (scope: CollaborationScope) => [...collaborationKeys.scope(scope), "knowledge"] as const;
const prefix = (key: QueryKey, root: QueryKey) => root.every((part, index) => key[index] === part);

function state(client: QueryClient, scope: CollaborationScope) {
  let entries = scopes.get(client);
  if (!entries) { entries = new Map(); scopes.set(client, entries); }
  const key = JSON.stringify(knowledgeScopeKey(scope));
  let entry = entries.get(key);
  if (!entry) { entry = { policy: new KnowledgeProjectionPolicy(), timers: new Map() }; entries.set(key, entry); }
  return entry;
}

/** Keep mounted Query observers connected, but erase their payloads and cancellation rollback. */
function erase(client: QueryClient, root: QueryKey, current?: QueryKey): Promise<void> {
  const isCurrent = (key: QueryKey) => current !== undefined && JSON.stringify(key) === JSON.stringify(current);
  const cancelled = client.cancelQueries({ predicate: (query) => prefix(query.queryKey, root) && !isCurrent(query.queryKey) }, { revert: false });
  for (const query of client.getQueryCache().findAll({ predicate: (entry) => prefix(entry.queryKey, root) })) {
    // A manual update replaces TanStack's saved cancellation rollback snapshot too.
    query.setData(undefined, { manual: true, updatedAt: 0 });
    query.setState({ data: undefined, dataUpdatedAt: 0, error: null, status: "pending" });
    if (!query.getObserversCount() && !isCurrent(query.queryKey)) client.removeQueries({ queryKey: query.queryKey, exact: true });
  }
  return cancelled;
}

/** A source relationship can be restricted or not loaded yet; clear the whole private knowledge scope. */
export function expireKnowledgeProjections(client: QueryClient, scope: CollaborationScope, refetch = true): void {
  const entry = state(client, scope);
  entry.policy.invalidate();
  for (const timer of entry.timers.values()) clearTimeout(timer);
  entry.timers.clear();
  void erase(client, knowledgeScopeKey(scope)).then(() => {
    if (refetch && isCollaborationScopeCurrent(scope) && onlineManager.isOnline() && focusManager.isFocused()) {
      void client.refetchQueries({ queryKey: knowledgeScopeKey(scope), type: "active" });
    }
  });
}

export async function readKnowledgeProjection<T extends CollaborationKnowledgeProjection>(
  client: QueryClient, scope: CollaborationScope, groupKey: QueryKey, queryKey: QueryKey,
  request: () => Promise<T | null>, parent?: KnowledgeProjectionBoundary, issueId: string | null = null,
): Promise<AuthorizedKnowledgeProjection<T>> {
  if (!isCollaborationScopeCurrent(scope) || !onlineManager.isOnline()) throw knowledgeProjectionExpired();
  const entry = state(client, scope);
  const group = JSON.stringify(groupKey);
  let token;
  try { token = entry.policy.begin(group, parent); }
  catch (error) { void erase(client, queryKey, queryKey); throw error; }
  if (!parent) {
    clearTimeout(entry.timers.get(group)); entry.timers.delete(group);
    // Cancellation is synchronous; do not await the current Query's own promise.
    void erase(client, groupKey, queryKey);
  }
  try {
    const value = await protectCollaborationRead(client, scope, issueId, request);
    if (!isCollaborationScopeCurrent(scope)) throw knowledgeProjectionExpired();
    const boundary = entry.policy.accept(token, value);
    clearTimeout(entry.timers.get(group));
    entry.timers.set(group, setTimeout(() => {
      entry.policy.invalidate(group);
      entry.timers.delete(group);
      void erase(client, groupKey).then(() => {
        if (isCollaborationScopeCurrent(scope) && onlineManager.isOnline() && focusManager.isFocused()) void client.refetchQueries({ queryKey: groupKey, type: "active" });
      });
    }, Math.max(0, boundary.deadline - performance.now())));
    return { value, boundary };
  } catch (error) {
    if (!entry.policy.isCurrent(token)) throw knowledgeProjectionExpired();
    entry.policy.invalidate(group);
    clearTimeout(entry.timers.get(group)); entry.timers.delete(group);
    void erase(client, groupKey, queryKey);
    throw error;
  }
}

/** UI lifecycle is optional plumbing; lease enforcement also runs for standalone QueryClient reads. */
export function bindKnowledgeProjectionLifecycle(client: QueryClient, scope: CollaborationScope): () => void {
  const entry = state(client, scope);
  if (!entry.lifecycle) {
    const focus = focusManager.subscribe(() => expireKnowledgeProjections(client, scope));
    const online = onlineManager.subscribe(() => expireKnowledgeProjections(client, scope));
    entry.lifecycle = { count: 0, dispose: () => { focus(); online(); expireKnowledgeProjections(client, scope, false); } };
  }
  const lifecycle = entry.lifecycle;
  lifecycle.count++;
  let disposed = false;
  return () => {
    if (disposed) return;
    disposed = true;
    if (--lifecycle.count === 0) { lifecycle.dispose(); entry.lifecycle = undefined; }
  };
}
