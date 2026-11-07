import { infiniteQueryOptions, replaceEqualDeep, type InfiniteData, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import { hasValidCollaborationCommandHistoryItems, type CollaborationCommandHistory } from "./command-history-schema";

function shareCommandHistoryPages(previous: unknown, next: unknown): unknown {
  if (next === undefined) return undefined;
  // API pages are already parsed. Validate TanStack's complete new chain,
  // allowing a full refresh to replace old states and Switch placements.
  const data = next as InfiniteData<CollaborationCommandHistory, string | undefined>;
  const first = data.pages[0];
  if (!first || data.pages.length !== data.pageParams.length || data.pageParams[0] !== undefined) throw new CollaborationUnavailableError();
  const cursors = new Set<string>();
  for (const [index, page] of data.pages.entries()) {
    const cursor = data.pageParams[index];
    if (page.issue_id !== first.issue_id || (index > 0 && cursor !== data.pages[index - 1]?.next_cursor)) throw new CollaborationUnavailableError();
    if (cursor !== undefined) {
      if (cursors.has(cursor)) throw new CollaborationUnavailableError();
      cursors.add(cursor);
    }
    if (page.next_cursor !== null && cursors.has(page.next_cursor)) throw new CollaborationUnavailableError();
  }
  if (!hasValidCollaborationCommandHistoryItems(data.pages.flatMap((page) => page.items))) throw new CollaborationUnavailableError();
  return replaceEqualDeep(previous, data);
}

export function collaborationCommandHistoryOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "commands"],
    staleTime: 0, retry: false, refetchOnMount: "always", refetchOnWindowFocus: "always", refetchOnReconnect: "always",
    refetchInterval: 15_000, refetchIntervalInBackground: false,
    structuralSharing: shareCommandHistoryPages,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      const api = getApi();
      const page = await api.getCollaborationCommandHistory(issueId, { cursor: pageParam, limit: 50 }, { signal });
      // An optional route or malformed history does not establish issue denial.
      // The current parent recheck stays inside the same response epoch fence.
      if (page === null) await api.getCollaborationIssue(issueId, { signal });
      return page;
    }),
    getNextPageParam: (page, _pages, _param, params) => page.next_cursor !== null && !params.includes(page.next_cursor)
      ? page.next_cursor : undefined,
  });
}
