import type { QueryClient } from "@tanstack/react-query";
import { inboxKeys } from "./queries";
import type { InboxItem, IssuePriority, IssueStatus } from "../types";

export function onInboxNew(
  qc: QueryClient,
  wsId: string,
  _item: InboxItem,
) {
  // Use invalidateQueries instead of setQueryData — triggers a refetch that
  // reliably notifies all observers. The inbox list is small so this is cheap.
  //
  // Delivery can revive an archived group or end a snooze. The server owns
  // that split, so all three views refresh together to remain exclusive.
  qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
}

export function patchInboxIssueProjection(
  qc: QueryClient,
  wsId: string,
  issueId: string,
  patch: { status?: IssueStatus; priority?: IssuePriority },
) {
  const project = (old: InboxItem[] | undefined) => {
    if (!old) return old;
    let changed = false;
    const next = old.map((item) => {
      if (item.issue_id !== issueId) return item;
      changed = true;
      return {
        ...item,
        ...(patch.status !== undefined
          ? { issue_status: patch.status }
          : {}),
        // Do not manufacture the projection on data returned by an older
        // backend. Capability detection relies on `undefined` continuing to
        // mean "this endpoint version does not provide issue_priority".
        ...(patch.priority !== undefined && item.issue_priority !== undefined
          ? { issue_priority: patch.priority }
          : {}),
      };
    });
    return changed ? next : old;
  };
  qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), project);
  // Archived rows expose the same issue fields and filter controls.
  qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), project);
  qc.setQueryData<InboxItem[]>(inboxKeys.snoozed(wsId), project);
}

export function patchInboxIssueStatus(
  qc: QueryClient,
  wsId: string,
  issueId: string,
  status: IssueStatus,
) {
  patchInboxIssueProjection(qc, wsId, issueId, { status });
}

export function onInboxIssueStatusChanged(
  qc: QueryClient,
  wsId: string,
  issueId: string,
  status: IssueStatus,
) {
  patchInboxIssueStatus(qc, wsId, issueId, status);
}

// Issue deletion removes its notifications server-side. Drop the same rows
// from every cached view, including postponed groups that can no longer open.
export function onInboxIssueDeleted(
  qc: QueryClient,
  wsId: string,
  issueId: string,
) {
  const drop = (old: InboxItem[] | undefined) =>
    old?.filter((i) => i.issue_id !== issueId);
  qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), drop);
  qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), drop);
  qc.setQueryData<InboxItem[]>(inboxKeys.snoozed(wsId), drop);
}

// Each state event can move a group across the three server-owned views.
export function onInboxInvalidate(qc: QueryClient, wsId: string) {
  qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
}

// Refresh the cross-workspace unread summary (workspace-switcher dot). The
// summary spans every workspace, so it is invalidated on ANY inbox event
// regardless of which workspace the event came from — including read/archive
// events from a workspace other than the active one, which the workspace-
// scoped list invalidation cannot reach.
export function onInboxSummaryInvalidate(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
}

export function inboxEventWorkspaceId(payload: unknown): string | undefined {
  if (
    payload !== null && typeof payload === "object" &&
    "workspace_id" in payload && typeof payload.workspace_id === "string" &&
    payload.workspace_id.length > 0
  ) return payload.workspace_id;
  return undefined;
}

export function onInboxUpdated(qc: QueryClient, payload: unknown) {
  const sourceWsId = inboxEventWorkspaceId(payload);
  if (sourceWsId) onInboxInvalidate(qc, sourceWsId);
  // Older personal events omit their workspace; recover every cached Inbox
  // instead of guessing from whichever route this device currently displays.
  else qc.invalidateQueries({ queryKey: ["inbox"] });
  onInboxSummaryInvalidate(qc);
}
