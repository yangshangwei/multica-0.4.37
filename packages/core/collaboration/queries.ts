import { useEffect, useMemo } from "react";
import { infiniteQueryOptions, queryOptions, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import { z } from "zod";
import { withQueryAccessBoundary } from "../query-access";
import { useAuthStore } from "../auth";
import type { AccountView } from "../paths/account";
import { collaborationKeys, collaborationScope, protectCollaborationRead, retainCollaborationScope, type CollaborationScope } from "./access";
import type { CollaborationStatusAction } from "./schema";

const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const, refetchInterval: 15_000, refetchIntervalInBackground: false };

export function useCollaborationScope(): CollaborationScope {
  const client = useQueryClient();
  const userId = useAuthStore((state) => state.user?.id ?? "");
  const authenticated = useAuthStore((state) => state.status === "authenticated");
  const { apiScope, server } = collaborationScope(userId);
  const scope = useMemo(() => ({ apiScope, server, userId }), [apiScope, userId, server]);
  useEffect(() => {
    if (authenticated && userId) retainCollaborationScope(client, scope);
  }, [client, scope, authenticated, userId]);
  return scope;
}

export function collaborationIssueOptions(client: ReturnType<typeof useQueryClient>, scope: CollaborationScope, issueId: string) {
  return queryOptions({
    queryKey: collaborationKeys.issue(scope, issueId), ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationIssue(issueId, { signal }), true),
  });
}

export function collaborationWorkbenchOptions(client: ReturnType<typeof useQueryClient>, scope: CollaborationScope, view: AccountView) {
  return infiniteQueryOptions({
    queryKey: collaborationKeys.workbench(scope, view), ...freshness,
    initialPageParam: undefined as string | undefined,
    queryFn: (context) => withQueryAccessBoundary(() => protectCollaborationRead(client, scope, null, () => getApi().getCollaborationWorkbench({ view, cursor: context.pageParam, limit: 50 }, { signal: context.signal })))(context),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function useCollaborationIssue(scope: CollaborationScope, issueId: string) {
  const client = useQueryClient();
  return useQuery({ ...collaborationIssueOptions(client, scope, issueId), enabled: scope.userId !== "" });
}

export function useCollaborationStatusOptions(scope: CollaborationScope, issueId: string, action: CollaborationStatusAction, enabled: boolean) {
  const client = useQueryClient();
  return useQuery({
    queryKey: [...collaborationKeys.issue(scope, issueId), "status-options", action],
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationStatusOptions(issueId, action, { signal })),
    enabled, staleTime: 0, retry: false,
  });
}

/** The confirmed lookup text owns its result; editing it selects a different Query. */
export class CollaborationRecipientNotFoundError extends Error {
  constructor() { super("No matching collaboration recipient"); this.name = "CollaborationRecipientNotFoundError"; }
}

export function collaborationRecipientOptions(client: ReturnType<typeof useQueryClient>, scope: CollaborationScope, issueId: string, value: string) {
  const exact = value.trim();
  const kind = z.uuid().safeParse(exact).success ? "user_id" : exact.includes("@") ? "email" : "username";
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "recipient", kind, exact],
    enabled: exact !== "", staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      try { return await getApi().resolveCollaborationRecipient({ issueId, kind, value: exact }, { signal }); }
      catch (error) {
        if (error instanceof ApiError && error.status === 404 && z.object({ code: z.literal("recipient_not_found") }).safeParse(error.body).success) throw new CollaborationRecipientNotFoundError();
        throw error;
      }
    }),
  });
}
