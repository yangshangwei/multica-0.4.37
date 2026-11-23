import { useEffect } from "react";
import { queryOptions, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { useKnowledgeRealtime } from "../realtime/knowledge";
import { CollaborationUnavailableError, collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";
import { bindKnowledgeProjectionLifecycle, knowledgeScopeKey, readKnowledgeProjection } from "./knowledge-cache";
import type { KnowledgeProjectionBoundary } from "./knowledge-lease";
import { KNOWLEDGE_REFRESH_MS, type CollaborationKnowledgePageParams, type CollaborationKnowledgeTarget } from "./knowledge-schema";

export const collaborationKnowledgeKeys = {
  root: knowledgeScopeKey,
  source: (scope: CollaborationScope, issueId: string) => [...knowledgeScopeKey(scope), "source", issueId.toLowerCase()] as const,
  candidate: (scope: CollaborationScope, candidateId: string) => [...knowledgeScopeKey(scope), "candidate", candidateId.toLowerCase()] as const,
  target: (scope: CollaborationScope, target: CollaborationKnowledgeTarget) => [...knowledgeScopeKey(scope), "target", target.workspace_id.toLowerCase(), target.kind,
    (target.kind === "project" ? target.project_id : target.workspace_id).toLowerCase()] as const,
  // Original-actor receipts have no private source projection and no source authorization prerequisite.
  operation: (scope: CollaborationScope, candidateId: string, requestId: string) => [...collaborationKeys.scope(scope), "knowledge-operation", candidateId.toLowerCase(), requestId.toLowerCase()] as const,
};
export interface KnowledgePageOptions extends CollaborationKnowledgePageParams { boundary?: KnowledgeProjectionBoundary }
const freshness = {
  staleTime: 0, gcTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const,
  refetchOnReconnect: "always" as const,
  refetchInterval: (query: { state: { error: Error | null } }) => query.state.error instanceof CollaborationUnavailableError ? false : KNOWLEDGE_REFRESH_MS,
  refetchIntervalInBackground: false,
};
function pageKey(root: readonly string[], page: KnowledgePageOptions) {
  if ((page.cursor === undefined) !== (page.boundary === undefined)) throw new TypeError("A knowledge page requires its first page authorization boundary");
  return [...root, "page", page.cursor ?? null, page.limit ?? 20, page.boundary ?? null] as const;
}

/** Explicit pages carry the first-page lease; fetching a later page cannot refresh older content. */
export function collaborationKnowledgeCandidatesOptions(client: QueryClient, scope: CollaborationScope, issueId: string, page: KnowledgePageOptions = {}) {
  const root = collaborationKnowledgeKeys.source(scope, issueId);
  const key = pageKey(root, page);
  return queryOptions({
    queryKey: key, ...freshness,
    queryFn: ({ signal }) => readKnowledgeProjection(client, scope, root, key,
      () => getApi().getCollaborationKnowledgeCandidates(issueId, { cursor: page.cursor, limit: page.limit }, { signal }), page.boundary, issueId),
  });
}

export function collaborationKnowledgeCandidateOptions(client: QueryClient, scope: CollaborationScope, candidateId: string) {
  const key = collaborationKnowledgeKeys.candidate(scope, candidateId);
  return queryOptions({ queryKey: key, ...freshness,
    queryFn: ({ signal }) => readKnowledgeProjection(client, scope, key, key, () => getApi().getCollaborationKnowledgeCandidate(candidateId, { signal })),
  });
}

export function collaborationKnowledgeTargetOptions(client: QueryClient, scope: CollaborationScope, target: CollaborationKnowledgeTarget, page: KnowledgePageOptions = {}) {
  const root = collaborationKnowledgeKeys.target(scope, target);
  const key = pageKey(root, page);
  return queryOptions({ queryKey: key, ...freshness,
    queryFn: ({ signal }) => readKnowledgeProjection(client, scope, root, key,
      () => getApi().getCollaborationKnowledgeTarget(target, { cursor: page.cursor, limit: page.limit }, { signal }), page.boundary),
  });
}

export function collaborationKnowledgeOperationOptions(client: QueryClient, scope: CollaborationScope, candidateId: string, requestId: string) {
  return queryOptions({ queryKey: collaborationKnowledgeKeys.operation(scope, candidateId, requestId), retry: false, staleTime: 0, gcTime: 0,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, null, () => getApi().getCollaborationKnowledgeOperation(candidateId, requestId, { signal })),
  });
}

function useKnowledgeLifecycle(scope: CollaborationScope) {
  const client = useQueryClient();
  useKnowledgeRealtime(scope);
  useEffect(() => bindKnowledgeProjectionLifecycle(client, scope), [client, scope]);
  return client;
}
export function useCollaborationKnowledgeCandidates(scope: CollaborationScope, issueId: string, page: KnowledgePageOptions = {}) {
  const client = useKnowledgeLifecycle(scope);
  return useQuery({ ...collaborationKnowledgeCandidatesOptions(client, scope, issueId, page), enabled: scope.userId !== "" });
}
export function useCollaborationKnowledgeCandidate(scope: CollaborationScope, candidateId: string) {
  const client = useKnowledgeLifecycle(scope);
  return useQuery({ ...collaborationKnowledgeCandidateOptions(client, scope, candidateId), enabled: scope.userId !== "" });
}
export function useCollaborationKnowledgeTarget(scope: CollaborationScope, target: CollaborationKnowledgeTarget, page: KnowledgePageOptions = {}) {
  const client = useKnowledgeLifecycle(scope);
  return useQuery({ ...collaborationKnowledgeTargetOptions(client, scope, target, page), enabled: scope.userId !== "" });
}
export function useCollaborationKnowledgeOperation(scope: CollaborationScope, candidateId: string, requestId: string) {
  const client = useQueryClient();
  return useQuery({ ...collaborationKnowledgeOperationOptions(client, scope, candidateId, requestId), enabled: scope.userId !== "" });
}
