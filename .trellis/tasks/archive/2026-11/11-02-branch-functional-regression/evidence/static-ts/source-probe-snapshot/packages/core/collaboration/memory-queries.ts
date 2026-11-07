import { infiniteQueryOptions, queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";

/**
 * Shared-context keys live under the issue prefix, so the existing
 * `collaboration:changed` invalidation refreshes them without a new frame.
 */
export const collaborationMemoryKeys = {
  root: (scope: CollaborationScope, issueId: string) => [...collaborationKeys.issue(scope, issueId), "memory"] as const,
  versions: (scope: CollaborationScope, issueId: string) => [...collaborationMemoryKeys.root(scope, issueId), "versions"] as const,
  version: (scope: CollaborationScope, issueId: string, version: string) => [...collaborationMemoryKeys.root(scope, issueId), "version", version] as const,
};

const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const, refetchIntervalInBackground: false };
// An older server has no shared-context route. Stop polling it; focus,
// reconnect and realtime invalidation still retry.
const polling = (query: { state: { error: unknown } }) => query.state.error instanceof CollaborationUnavailableError ? false : 15_000;

export function collaborationMemoryOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return queryOptions({
    queryKey: collaborationMemoryKeys.root(scope, issueId), ...freshness, refetchInterval: polling,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationMemory(issueId, { signal })),
  });
}

export function collaborationMemoryVersionsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: collaborationMemoryKeys.versions(scope, issueId), ...freshness, refetchInterval: polling,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId,
      () => getApi().getCollaborationMemoryVersions(issueId, { cursor: pageParam, limit: 50 }, { signal })),
    getNextPageParam: (page, _pages, _param, params) => page.next_cursor !== null && !params.includes(page.next_cursor) ? page.next_cursor : undefined,
  });
}

/** Version text is immutable except for a purge, which realtime invalidation reports. */
export function collaborationMemoryVersionOptions(client: QueryClient, scope: CollaborationScope, issueId: string, version: string) {
  return queryOptions({
    queryKey: collaborationMemoryKeys.version(scope, issueId, version), ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationMemoryVersion(issueId, version, { signal })),
  });
}
