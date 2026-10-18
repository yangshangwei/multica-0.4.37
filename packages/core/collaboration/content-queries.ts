import { infiniteQueryOptions, queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";

const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const, refetchInterval: 15_000 };
export function collaborationCommentsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "comments"], ...freshness,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationComments(issueId, { cursor: pageParam }, { signal })),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}
export function collaborationMentionTargetsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "mention-targets"], ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationMentionTargets(issueId, { signal })),
  });
}
export function collaborationNotificationsOptions(client: QueryClient, scope: CollaborationScope, includeArchived = false) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.notifications(scope), { includeArchived }], ...freshness,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, null, () => getApi().getCollaborationNotifications({ cursor: pageParam, includeArchived }, { signal })),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}
