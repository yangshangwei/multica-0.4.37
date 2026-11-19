import { infiniteQueryOptions, queryOptions, replaceEqualDeep, type InfiniteData, type QueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import { compareCollaborationInformationTuple, type CollaborationInformationAffected, type CollaborationInformationHistory } from "./information-schema";
import type { CollaborationIssue } from "./schema";

type Observation = Pick<CollaborationIssue, "collaboration_revision" | "lifecycle_epoch" | "information_workflow">;
const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const };
const observationKey = (value?: Observation) => [value?.collaboration_revision ?? null, value?.lifecycle_epoch ?? null, value?.information_workflow?.context_version ?? null];
function matchesObservation(value: { collaboration_revision: string; lifecycle_epoch: string }, expected?: Observation) {
  return expected === undefined || value.collaboration_revision === expected.collaboration_revision && value.lifecycle_epoch === expected.lifecycle_epoch;
}

/** A missing individual handoff is not proof that its parent became unreadable. */
async function readInformation<T>(issueId: string, signal: AbortSignal, request: () => Promise<T | null>): Promise<T | null> {
  try {
    const value = await request();
    if (value === null) await getApi().getCollaborationIssue(issueId, { signal });
    return value;
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 404 || error.isStaleAuthResponse) throw error;
    await getApi().getCollaborationIssue(issueId, { signal });
    return null;
  }
}

function pageBinding(pages: readonly { issue_id: string; collaboration_revision: string; lifecycle_epoch: string; upper_handoff: unknown; next_cursor: string | null }[], params: readonly (string | undefined)[]) {
  if (pages.length !== params.length) throw new CollaborationUnavailableError();
  const head = pages[0];
  const cursors = new Set<string>();
  pages.forEach((page, index) => {
    if (head && (page.issue_id !== head.issue_id || page.collaboration_revision !== head.collaboration_revision ||
      page.lifecycle_epoch !== head.lifecycle_epoch || JSON.stringify(page.upper_handoff) !== JSON.stringify(head.upper_handoff))) throw new CollaborationUnavailableError();
    const cursor = params[index];
    if (cursor !== undefined) {
      if (cursors.has(cursor) || index === 0 || pages[index - 1]?.next_cursor !== cursor) throw new CollaborationUnavailableError();
      cursors.add(cursor);
    } else if (index !== 0) throw new CollaborationUnavailableError();
    if (page.next_cursor !== null && cursors.has(page.next_cursor)) throw new CollaborationUnavailableError();
  });
}

export function shareCollaborationInformationPages(previous: unknown, next: unknown): unknown {
  if (next === undefined) return undefined;
  const data = next as InfiniteData<CollaborationInformationHistory, string | undefined>;
  if (!Array.isArray(data.pages) || !Array.isArray(data.pageParams)) throw new CollaborationUnavailableError();
  pageBinding(data.pages, data.pageParams);
  const ids = new Set<string>();
  let last: CollaborationInformationHistory["items"][number] | undefined;
  for (const page of data.pages) {
    for (const item of page.items) {
      if (ids.has(item.handoff_id) || last && compareCollaborationInformationTuple(last, item) <= 0) throw new CollaborationUnavailableError();
      ids.add(item.handoff_id); last = item;
    }
  }
  return replaceEqualDeep(previous, data);
}

export function shareCollaborationInformationAffectedPages(previous: unknown, next: unknown): unknown {
  if (next === undefined) return undefined;
  const data = next as InfiniteData<CollaborationInformationAffected, string | undefined>;
  if (!Array.isArray(data.pages) || !Array.isArray(data.pageParams)) throw new CollaborationUnavailableError();
  pageBinding(data.pages, data.pageParams);
  const head = data.pages[0], ids = new Set<string>();
  for (const page of data.pages) {
    if (head && (page.repair_handoff_id !== head.repair_handoff_id || page.superseded_request_count !== head.superseded_request_count)) throw new CollaborationUnavailableError();
    for (const item of page.items) {
      if (ids.has(item.request_id)) throw new CollaborationUnavailableError();
      ids.add(item.request_id);
    }
  }
  if (head && (BigInt(ids.size) > BigInt(head.superseded_request_count) ||
    data.pages.at(-1)?.next_cursor === null && BigInt(ids.size) !== BigInt(head.superseded_request_count))) throw new CollaborationUnavailableError();
  return replaceEqualDeep(previous, data);
}

export function collaborationInformationHistoryOptions(client: QueryClient, scope: CollaborationScope, issueId: string, observation?: Observation) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "information-history", ...observationKey(observation)], ...freshness,
    structuralSharing: shareCollaborationInformationPages, initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      const value = await readInformation(issueId, signal, () => getApi().getCollaborationInformationHistory(issueId, { limit: 50, cursor: pageParam }, { signal }));
      if (value !== null && !matchesObservation(value, observation)) throw new CollaborationUnavailableError();
      return value;
    }),
    getNextPageParam: (page, _pages, _param, params) => {
      if (page.next_cursor !== null && params.includes(page.next_cursor)) throw new CollaborationUnavailableError();
      return page.next_cursor ?? undefined;
    },
  });
}

export function collaborationInformationHandoffOptions(client: QueryClient, scope: CollaborationScope, issueId: string, handoffId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "information-handoff", handoffId], ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId,
      () => readInformation(issueId, signal, () => getApi().getCollaborationInformationHandoff(issueId, handoffId, { signal }))),
  });
}

export function collaborationInformationAffectedOptions(client: QueryClient, scope: CollaborationScope, issueId: string, repairId: string, observation?: Observation, expectedCount?: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "information-affected", repairId, ...observationKey(observation), expectedCount ?? null], ...freshness,
    structuralSharing: shareCollaborationInformationAffectedPages, initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      const value = await readInformation(issueId, signal, () => getApi().getCollaborationInformationRepairAffected(issueId, repairId, { limit: 50, cursor: pageParam }, { signal }));
      if (value !== null && (!matchesObservation(value, observation) || expectedCount !== undefined && value.superseded_request_count !== expectedCount)) throw new CollaborationUnavailableError();
      return value;
    }),
    getNextPageParam: (page, _pages, _param, params) => {
      if (page.next_cursor !== null && params.includes(page.next_cursor)) throw new CollaborationUnavailableError();
      return page.next_cursor ?? undefined;
    },
  });
}
