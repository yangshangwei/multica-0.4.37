import { infiniteQueryOptions, replaceEqualDeep, type InfiniteData, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import type { CollaborationRunRecords } from "./run-records-schema";

function shareRecordedPages(previous: unknown, next: unknown): unknown {
  // Access cleanup must still be able to remove every page immediately.
  if (next === undefined) return undefined;
  // Each page has already crossed the API schema. Check TanStack's complete
  // new chain before caching it; an old snapshot may have different boundaries.
  const data = next as InfiniteData<CollaborationRunRecords, string | undefined>;
  const cursors = new Set<string>();
  const records = new Map<string, string>();
  for (const [index, page] of data.pages.entries()) {
    const cursor = data.pageParams[index];
    if (cursor !== undefined) cursors.add(cursor);
    if (page.next_cursor !== null && cursors.has(page.next_cursor)) throw new CollaborationUnavailableError();
    for (const item of page.items) {
      const recorded = JSON.stringify(item);
      const earlier = records.get(item.id);
      if (earlier !== undefined && earlier !== recorded) throw new CollaborationUnavailableError();
      records.set(item.id, recorded);
    }
  }
  return replaceEqualDeep(previous, data);
}

export function collaborationRunRecordsOptions(client: QueryClient, scope: CollaborationScope, issueId: string, runId: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "run", runId.toLowerCase(), "records"],
    staleTime: 0, retry: false, refetchOnMount: "always", refetchOnWindowFocus: "always", refetchOnReconnect: "always",
    refetchInterval: 15_000, refetchIntervalInBackground: false,
    structuralSharing: shareRecordedPages,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      const api = getApi();
      const page = await api.getCollaborationRunRecords(issueId, runId, { cursor: pageParam, limit: 50 }, { signal });
      // A missing optional route/run or invalid page does not establish loss of
      // issue authority. Recheck that authority inside the same response fence.
      if (page === null) await api.getCollaborationIssue(issueId, { signal });
      return page;
    }),
    // Stop a forged cycle even during a multi-page fetch. The cache check above
    // reports the invalid chain as unavailable rather than a completed history.
    getNextPageParam: (page, _pages, _param, params) => page.next_cursor !== null && !params.includes(page.next_cursor)
      ? page.next_cursor : undefined,
  });
}
