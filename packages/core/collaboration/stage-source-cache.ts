import type { QueryClient, QueryKey } from "@tanstack/react-query";
import { collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";
import type { CollaborationStageCodeCandidate, CollaborationStageRead } from "./stage-schema";

const generations = new WeakMap<QueryClient, Map<string, number>>();
function state(client: QueryClient) {
  let entries = generations.get(client);
  if (!entries) { entries = new Map(); generations.set(client, entries); }
  return entries;
}
const sourceKey = (scope: CollaborationScope, workspaceId: string) => JSON.stringify([...collaborationKeys.scope(scope), workspaceId.toLowerCase()]);
const superseded = () => new DOMException("Stage source authority changed", "AbortError");

/** Source membership is workspace-wide. Preserve issue identity/history and scrub private R4/T4 entries. */
export function eraseCollaborationStageSource(client: QueryClient, scope: CollaborationScope, workspaceId: string, currentKey?: QueryKey): void {
  const entries = state(client), key = sourceKey(scope, workspaceId);
  entries.set(key, (entries.get(key) ?? 0) + 1);
  const root = collaborationKeys.scope(scope);
  const owned = (query: { queryKey: QueryKey }) => root.every((value, index) => query.queryKey[index] === value) &&
    ((query.queryKey.at(-3) === "code-candidate" && query.queryKey.at(-2) === workspaceId.toLowerCase()) ||
      (["test-reports", "completion"].includes(String(query.queryKey[6])) && query.queryKey[7] === workspaceId.toLowerCase()));
  const current = (key: QueryKey) => currentKey !== undefined && JSON.stringify(key) === JSON.stringify(currentKey);
  const queries = client.getQueryCache().findAll({ predicate: owned }).map((query) => ({ query, previous: query.state }));
  void client.cancelQueries({ predicate: (query) => owned(query) && !current(query.queryKey) }, { revert: false });
  for (const { query, previous } of queries) {
    const read = previous.data as CollaborationStageRead<CollaborationStageCodeCandidate> | undefined;
    const preserved = query.queryKey.at(-3) === "code-candidate" && read?.state === "available" && read.value.candidate.baseline === null ? read : undefined;
    // Keep a server-issued anonymous identity and its original read/error state;
    // another denial is not a new positive read or a reason to discard recovery UI.
    query.setData(preserved, { manual: true, updatedAt: preserved ? previous.dataUpdatedAt : 0 });
    query.setState({ data: preserved, dataUpdatedAt: preserved ? previous.dataUpdatedAt : 0,
      error: preserved ? previous.error : null, status: preserved ? previous.status : "pending" });
  }
}

export async function protectCollaborationStageSource<T>(client: QueryClient, scope: CollaborationScope, issueId: string | null, workspaceId: string,
  request: () => Promise<T>): Promise<T> {
  const entries = state(client), key = sourceKey(scope, workspaceId), generation = entries.get(key) ?? 0;
  const result = await protectCollaborationRead(client, scope, issueId, async () => {
    try {
      const value = await request();
      if ((entries.get(key) ?? 0) !== generation) throw superseded();
      return value;
    } catch (error) {
      // A stale source request must not turn an old 404 into fresh issue-access loss.
      if ((entries.get(key) ?? 0) !== generation) throw superseded();
      throw error;
    }
  });
  if ((entries.get(key) ?? 0) !== generation) throw superseded();
  return result;
}

export async function readCollaborationStageSource(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string, queryKey: QueryKey,
  request: () => Promise<CollaborationStageRead<CollaborationStageCodeCandidate>>): Promise<CollaborationStageRead<CollaborationStageCodeCandidate>> {
  const result = await protectCollaborationStageSource(client, scope, issueId, workspaceId, request);
  if (result.state === "available" && result.value.candidate.baseline === null) eraseCollaborationStageSource(client, scope, workspaceId, queryKey);
  return result;
}
