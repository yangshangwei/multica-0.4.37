import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import { issueKeys } from "../issues/queries";
import { projectKeys } from "../projects/queries";
import { collaborationKeys, isCollaborationScopeCurrent, type CollaborationScope } from "./access";

const source = z.object({ source_workspace: z.object({ id: z.uuid() }) });
const emptyTick = z.strictObject({});

export function isCollaborationInvalidation(payload: unknown): boolean {
  return emptyTick.safeParse(payload).success;
}

/** Shared workflow changes affect source grouping, child progress and reports.
 * Empty account ticks do not identify a workspace; inactive caches become stale
 * while only mounted source views refetch. Never copy an account DTO into them.
 */
export function invalidateSourceIssueViews(client: QueryClient, workspaceId?: string, issueId?: string): void {
  if (workspaceId) {
    void client.invalidateQueries({ queryKey: issueKeys.all(workspaceId) });
    void client.invalidateQueries({ queryKey: projectKeys.all(workspaceId) });
    void client.invalidateQueries({ queryKey: ["iterations", workspaceId] });
    if (issueId) {
      void client.invalidateQueries({ queryKey: issueKeys.timeline(issueId) });
      void client.invalidateQueries({ queryKey: issueKeys.attachments(issueId) });
      void client.invalidateQueries({ queryKey: issueKeys.reactions(issueId) });
    }
    return;
  }
  void client.invalidateQueries({ predicate: (query) => ["issues", "projects", "iterations"].includes(String(query.queryKey[0])) });
}

/** Wait for the issue query's current re-read to settle; a mutation awaits this
 * because the next action captures its versions from that cache. Realtime ticks
 * restart an in-flight read, and TanStack forwards a cancelled read's caller
 * only one restart, so follow the query's current read until it is idle instead
 * of awaiting invalidation.
 */
export async function waitForCollaborationIssueRefresh(client: QueryClient, scope: CollaborationScope, issueId: string): Promise<void> {
  const queryKey = collaborationKeys.issue(scope, issueId);
  const cache = client.getQueryCache();
  const query = cache.find({ queryKey, exact: true });
  let seen: Promise<unknown> | undefined;
  while (query && cache.get(query.queryHash) === query && query.state.fetchStatus === "fetching" && query.promise && query.promise !== seen) {
    seen = query.promise;
    await seen.catch(() => undefined);
  }
}

/** Invalidate the issue subtree and wait for its current re-read to settle. */
export async function refreshCollaborationIssue(client: QueryClient, scope: CollaborationScope, issueId: string): Promise<void> {
  void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issueId) });
  await waitForCollaborationIssueRefresh(client, scope, issueId);
}

export function invalidateCollaborationSourceViews(client: QueryClient, scope: CollaborationScope, issueId?: string, workspaceId?: string): void {
  if (!isCollaborationScopeCurrent(scope)) return;
  const snapshot = issueId ? source.safeParse(client.getQueryData(collaborationKeys.issue(scope, issueId))) : null;
  invalidateSourceIssueViews(client, workspaceId ?? (snapshot?.success ? snapshot.data.source_workspace.id : undefined), issueId);
}
