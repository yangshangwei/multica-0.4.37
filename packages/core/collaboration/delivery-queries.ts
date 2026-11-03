import { infiniteQueryOptions, replaceEqualDeep, type InfiniteData, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import type { CollaborationDeliveryHistory } from "./delivery-schema";

function shareDeliveryPages(previous: unknown, next: unknown): unknown {
  if (next === undefined) return undefined;
  const data = next as InfiniteData<CollaborationDeliveryHistory, string | undefined>;
  const cursors = new Set<string>();
  const submissions = new Map<string, string>();
  let issueId: string | undefined;
  let sequence: bigint | undefined;
  for (const [index, page] of data.pages.entries()) {
    if (issueId !== undefined && page.issue_id !== issueId) throw new CollaborationUnavailableError();
    issueId = page.issue_id;
    const cursor = data.pageParams[index];
    if (cursor !== undefined) cursors.add(cursor);
    if (page.next_cursor !== null && cursors.has(page.next_cursor)) throw new CollaborationUnavailableError();
    for (const submission of page.items) {
      const encoded = JSON.stringify(submission);
      const old = submissions.get(submission.id);
      if (old !== undefined && old !== encoded || old === undefined && sequence !== undefined && BigInt(submission.event_seq) >= sequence) throw new CollaborationUnavailableError();
      if (old === undefined) sequence = BigInt(submission.event_seq);
      submissions.set(submission.id, encoded);
    }
  }
  return replaceEqualDeep(previous, data);
}

export function collaborationDeliveryHistoryOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "delivery-history"],
    staleTime: 0, retry: false, refetchOnMount: "always", refetchOnWindowFocus: "always", refetchOnReconnect: "always",
    structuralSharing: shareDeliveryPages, initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      const api = getApi();
      const page = await api.getCollaborationDeliveryHistory(issueId, { limit: 50, cursor: pageParam }, { signal });
      if (page === null) await api.getCollaborationIssue(issueId, { signal });
      return page;
    }),
    getNextPageParam: (page, _pages, _param, params) => page.next_cursor !== null && !params.includes(page.next_cursor) ? page.next_cursor : undefined,
  });
}
