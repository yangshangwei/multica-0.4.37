import { infiniteQueryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";
import { collaborationDeliveryKey, protectCollaborationDelivery } from "./delivery-access";

/** The picker runs in one existing issue or while creating from a source workspace. */
export type CollaborationRecipientSearchContext = { kind: "issue"; issueId: string } | { kind: "creation"; sourceWorkspaceId: string };

export function collaborationRecipientSearchOptions(client: QueryClient, scope: CollaborationScope, context: CollaborationRecipientSearchContext, q: string, membersOnly: boolean) {
  const query = q.trim();
  const shared = { staleTime: 30_000, retry: false as const, initialPageParam: undefined as string | undefined };
  if (context.kind === "issue") {
    const { issueId } = context;
    return infiniteQueryOptions({
      queryKey: [...collaborationKeys.issue(scope, issueId), "recipient-search", query, membersOnly], ...shared,
      queryFn: ({ pageParam, signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().searchCollaborationRecipients({ issueId, q: query, cursor: pageParam, membersOnly }, { signal })),
      getNextPageParam: (page) => page.next_cursor ?? undefined,
    });
  }
  const delivery = { kind: "creation" as const, sourceWorkspaceId: context.sourceWorkspaceId };
  return infiniteQueryOptions({
    queryKey: [...collaborationDeliveryKey(scope, delivery), "recipient-search", query, membersOnly], ...shared,
    queryFn: ({ pageParam, signal }) => protectCollaborationDelivery(client, scope, delivery, () => getApi().searchCollaborationRecipients({ sourceWorkspaceId: delivery.sourceWorkspaceId, q: query, cursor: pageParam, membersOnly }, { signal })),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}
