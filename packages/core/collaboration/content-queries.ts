import { infiniteQueryOptions, queryOptions, useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";
import type { CollaborationPreviewInput } from "./content-schema";
import { withQueryAccessBoundary } from "../query-access";

const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const, refetchInterval: 15_000 };
export function collaborationActivityOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "activity"], ...freshness,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationActivity(issueId, { cursor: pageParam }, { signal })),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}
export function collaborationCommentsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "comments"], ...freshness,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationComments(issueId, { cursor: pageParam }, { signal })),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}
export function collaborationCommentOptions(client: QueryClient, scope: CollaborationScope, issueId: string, commentId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "comment", commentId], ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationComment(issueId, commentId, { signal })),
  });
}
export function collaborationCommentHistoryOptions(client: QueryClient, scope: CollaborationScope, issueId: string, commentId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "comment", commentId, "history"], ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationCommentHistory(issueId, commentId, { signal })),
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
    queryFn: (context) => withQueryAccessBoundary(() => protectCollaborationRead(client, scope, null, () => getApi().getCollaborationNotifications({ cursor: context.pageParam, includeArchived }, { signal: context.signal })))(context),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function useCollaborationCommentPreview(scope: CollaborationScope, issueId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: [...collaborationKeys.issue(scope, issueId), "comment-preview"], retry: false, networkMode: "always",
    mutationFn: (input: CollaborationPreviewInput) => protectCollaborationRead(client, scope, issueId, () => getApi().previewCollaborationComment(issueId, input)),
  });
}

export function useUpdateCollaborationNotification(scope: CollaborationScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: [...collaborationKeys.notifications(scope), "update"], retry: false, networkMode: "always",
    mutationFn: ({ id, action }: { id: string; action: "read" | "archive" }) => protectCollaborationRead(client, scope, null, () => getApi().updateCollaborationNotification(id, action)),
    onSuccess: () => { void client.invalidateQueries({ queryKey: collaborationKeys.notifications(scope) }); },
  });
}
