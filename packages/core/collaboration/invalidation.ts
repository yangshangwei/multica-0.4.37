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

export function invalidateCollaborationSourceViews(client: QueryClient, scope: CollaborationScope, issueId?: string, workspaceId?: string): void {
  if (!isCollaborationScopeCurrent(scope)) return;
  const snapshot = issueId ? source.safeParse(client.getQueryData(collaborationKeys.issue(scope, issueId))) : null;
  invalidateSourceIssueViews(client, workspaceId ?? (snapshot?.success ? snapshot.data.source_workspace.id : undefined), issueId);
}
