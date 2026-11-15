import { useEffect, useRef } from "react";
import { infiniteQueryOptions, queryOptions, replaceEqualDeep, useInfiniteQuery, useQuery, useQueryClient, type InfiniteData, type QueryClient } from "@tanstack/react-query";
import { memberListOptions } from "../workspace/queries";
import { getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import { eraseCollaborationStageSource, protectCollaborationStageSource, readCollaborationStageSource } from "./stage-source-cache";
import { collaborationStageSourceDenied } from "./stage-presentation";
import type { CollaborationIssue } from "./schema";
import type { CollaborationStageProposalPage, CollaborationStageRead } from "./stage-schema";

/**
 * Stage data is server state: React Query only, keyed under the collaboration
 * issue key so the existing issue invalidation and access-loss cleanup cover it.
 * No realtime channel exists for stage data (contract section 9).
 */
export const collaborationStageKeys = {
  root: (scope: CollaborationScope, issueId: string) => [...collaborationKeys.issue(scope, issueId), "stage"] as const,
  view: (scope: CollaborationScope, issueId: string) => [...collaborationStageKeys.root(scope, issueId), "view"] as const,
  proposals: (scope: CollaborationScope, issueId: string) => [...collaborationStageKeys.root(scope, issueId), "proposals"] as const,
  proposal: (scope: CollaborationScope, issueId: string, proposalId: string) => [...collaborationStageKeys.root(scope, issueId), "proposal", proposalId.toLowerCase()] as const,
  codeCandidate: (scope: CollaborationScope, issueId: string, candidateId: string, workspaceId: string) => [...collaborationStageKeys.root(scope, issueId), "code-candidate", workspaceId.toLowerCase(), candidateId.toLowerCase()] as const,
  // Workspace-scoped data names its workspace; it stays under the issue whose panel reads it.
  policy: (scope: CollaborationScope, issueId: string, workspaceId: string, projectId: string) =>
    [...collaborationStageKeys.root(scope, issueId), "policy", workspaceId.toLowerCase(), projectId.toLowerCase()] as const,
};

const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const };
const VIEW_REFRESH_MS = 15_000;

/** R1. The read state is data: an older server is a stable "unsupported", not an error to retry. */
export function collaborationStageViewOptions(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string) {
  return queryOptions({
    queryKey: collaborationStageKeys.view(scope, issueId), ...freshness,
    // A server without the route will not grow one between polls.
    refetchInterval: (query: { state: { data?: unknown } }) => (query.state.data as { state?: string } | undefined)?.state === "unsupported" ? false : VIEW_REFRESH_MS,
    refetchIntervalInBackground: false,
    queryFn: async ({ signal }) => {
      if (!workspaceId) throw new CollaborationUnavailableError();
      const result = await protectCollaborationStageSource(client, scope, issueId, workspaceId, () => getApi().getCollaborationStage(issueId, { signal }));
      // Clear private secondary projections before observers receive the new
      // R1 denial. Its public child identity and original requests stay intact.
      if (result.state === "available" && collaborationStageSourceDenied(result.value)) eraseCollaborationStageSource(client, scope, workspaceId);
      return result;
    },
  });
}

type ProposalPages = InfiniteData<CollaborationStageRead<CollaborationStageProposalPage>, string | undefined>;
/** Pages never repeat a proposal and a cursor never loops; anything else is not a list to trust. */
function shareProposalPages(previous: unknown, next: unknown): unknown {
  if (next === undefined) return undefined;
  const data = next as ProposalPages;
  const cursors = new Set<string>(), proposals = new Set<string>();
  for (const [index, read] of data.pages.entries()) {
    const cursor = data.pageParams[index];
    if (cursor !== undefined) cursors.add(cursor);
    if (read.state !== "available") {
      // Only the first page may be a non-available state; a later one breaks the chain.
      if (index > 0) throw new CollaborationUnavailableError();
      continue;
    }
    if (read.value.next_cursor !== null && cursors.has(read.value.next_cursor)) throw new CollaborationUnavailableError();
    for (const item of read.value.items) {
      if (proposals.has(item.proposal_id)) throw new CollaborationUnavailableError();
      proposals.add(item.proposal_id);
    }
  }
  return replaceEqualDeep(previous, data);
}

/** R2. */
export function collaborationStageProposalsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: collaborationStageKeys.proposals(scope, issueId), ...freshness,
    structuralSharing: shareProposalPages, initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId,
      () => getApi().getCollaborationStageProposals(issueId, { limit: 20, cursor: pageParam }, { signal })),
    getNextPageParam: (page, _pages, _param, params) => page.state === "available" && page.value.next_cursor !== null && !params.includes(page.value.next_cursor)
      ? page.value.next_cursor : undefined,
  });
}

/** R3. The materialized child mapping is read here under current authority. */
export function collaborationStageProposalOptions(client: QueryClient, scope: CollaborationScope, issueId: string, proposalId: string) {
  return queryOptions({
    queryKey: collaborationStageKeys.proposal(scope, issueId, proposalId), ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationStageProposal(issueId, proposalId, { signal })),
  });
}

/** P1. Source membership is independent of collaboration access; a member refusal is data, not issue denial. */
export function collaborationStagePolicyOptions(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string, projectId: string) {
  return queryOptions({
    queryKey: collaborationStageKeys.policy(scope, issueId, workspaceId, projectId), ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getProjectStagePolicy(workspaceId, projectId, { signal })),
  });
}

/**
 * The stage view of an authorized collaboration detail. When the detail has
 * moved to another revision or lifecycle than the cached view observed, the
 * view is re-read once for that detail version: actions stay disabled until
 * both describe the same state (see `deriveCollaborationStageState`).
 */
export function useCollaborationStageView(scope: CollaborationScope, issue: CollaborationIssue) {
  const client = useQueryClient();
  const query = useQuery({ ...collaborationStageViewOptions(client, scope, issue.id, issue.source_workspace.id), enabled: scope.userId !== "" && issue.capabilities.read === true });
  const { data, isFetching, refetch } = query;
  const requested = useRef<string | null>(null);
  useEffect(() => {
    if (data?.state !== "available" || isFetching) return;
    const version = `${issue.collaboration_revision}:${issue.lifecycle_epoch}`;
    if (data.value.collaboration_revision === issue.collaboration_revision && data.value.lifecycle_epoch === issue.lifecycle_epoch) { requested.current = null; return; }
    if (requested.current === version) return;
    requested.current = version;
    void refetch({ cancelRefetch: false });
  }, [data, isFetching, refetch, issue.collaboration_revision, issue.lifecycle_epoch]);
  return query;
}

export function useCollaborationStageProposals(scope: CollaborationScope, issueId: string, enabled: boolean) {
  const client = useQueryClient();
  return useInfiniteQuery({ ...collaborationStageProposalsOptions(client, scope, issueId), enabled: enabled && scope.userId !== "" });
}

export function useCollaborationStageProposal(scope: CollaborationScope, issueId: string, proposalId: string | null) {
  const client = useQueryClient();
  return useQuery({ ...collaborationStageProposalOptions(client, scope, issueId, proposalId ?? ""), enabled: proposalId !== null && scope.userId !== "" });
}

export function useCollaborationStagePolicy(scope: CollaborationScope, issueId: string, workspaceId: string, projectId: string | null, enabled: boolean) {
  const client = useQueryClient();
  return useQuery({ ...collaborationStagePolicyOptions(client, scope, issueId, workspaceId, projectId ?? ""), enabled: enabled && projectId !== null && scope.userId !== "" });
}

/** R4 uses the same issue prefix, abort signal and access fence as every private stage read. */
export function collaborationStageCodeCandidateOptions(client: QueryClient, scope: CollaborationScope, issueId: string, candidateId: string, workspaceId: string) {
  const queryKey = collaborationStageKeys.codeCandidate(scope, issueId, candidateId, workspaceId);
  return queryOptions({ queryKey, ...freshness,
    refetchInterval: VIEW_REFRESH_MS, refetchIntervalInBackground: false,
    queryFn: ({ signal }) => readCollaborationStageSource(client, scope, issueId, workspaceId, queryKey, () => getApi().getCollaborationStageCodeCandidate(issueId, candidateId, { signal })),
  });
}
export function useCollaborationStageCodeCandidate(scope: CollaborationScope, issueId: string, candidateId: string | null, workspaceId: string,
  discovery?: import("./stage-schema").CollaborationStageView["code_candidates"]) {
  const client = useQueryClient();
  const query = useQuery({ ...collaborationStageCodeCandidateOptions(client, scope, issueId, candidateId ?? "", workspaceId), enabled: candidateId !== null && scope.userId !== "" });
  const entry = discovery?.items.find((candidate) => candidate.candidate_id === candidateId);
  const identity = JSON.stringify([candidateId, entry?.version, entry?.content_digest, entry?.state]);
  const previous = useRef(identity);
  const { refetch } = query;
  useEffect(() => {
    if (previous.current === identity) return;
    previous.current = identity;
    if (candidateId !== null) void refetch({ cancelRefetch: false });
  }, [identity, candidateId, refetch]);
  return query;
}

export interface CollaborationStagePolicyAuthority { workspaceId: string; userId: string; role: "owner" | "admin" | "member" | null }
/** Reuse the explicit source-workspace roster read, retaining only this viewer's role under the account/session fence. */
export function collaborationStagePolicyAuthorityOptions(client: QueryClient, scope: CollaborationScope, workspaceId: string) {
  const members = memberListOptions(workspaceId);
  return queryOptions({ queryKey: [...collaborationKeys.scope(scope), "stage-policy-authority", workspaceId.toLowerCase()], ...freshness,
    refetchInterval: VIEW_REFRESH_MS,
    queryFn: (context): Promise<CollaborationStagePolicyAuthority> => protectCollaborationRead(client, scope, null, async () => {
      if (typeof members.queryFn !== "function") throw new CollaborationUnavailableError();
      const rows = await members.queryFn(context);
      const own = rows.filter((member) => member.workspace_id.toLowerCase() === workspaceId.toLowerCase() && member.user_id.toLowerCase() === scope.userId.toLowerCase());
      const role = own.length === 1 ? own[0]!.role : null;
      return { workspaceId: workspaceId.toLowerCase(), userId: scope.userId.toLowerCase(), role: role === "owner" || role === "admin" || role === "member" ? role : null };
    }),
  });
}
export function useCollaborationStagePolicyAuthority(scope: CollaborationScope, workspaceId: string) {
  const client = useQueryClient();
  return useQuery(collaborationStagePolicyAuthorityOptions(client, scope, workspaceId));
}
