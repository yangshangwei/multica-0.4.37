import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { InboxItem, InboxWorkspaceUnread } from "../types";

export const inboxKeys = {
  all: (wsId: string) => ["inbox", wsId] as const,
  list: (wsId: string) => [...inboxKeys.all(wsId), "list"] as const,
  archived: (wsId: string) => [...inboxKeys.all(wsId), "archived"] as const,
  snoozed: (wsId: string) => [...inboxKeys.all(wsId), "snoozed"] as const,
  // Account-level (not workspace-scoped): a single shared cache entry that
  // holds unread counts for every workspace the user belongs to.
  unreadSummary: () => ["inbox", "unread-summary"] as const,
};

export function inboxListOptions(wsId: string) {
  return queryOptions({
    queryKey: inboxKeys.list(wsId),
    queryFn: () => api.listInbox({ workspaceId: wsId }),
    refetchInterval: 30_000,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
  });
}

/**
 * Archived notifications, backing the inbox's "Archived" sub-view. A separate
 * cache entry from the main list rather than one flat cache split locally
 * (which is what chat does): the archive grows without end, so it is fetched
 * from its own capped endpoint, and the server — not the client — decides
 * which issues belong in which list.
 */
export function archivedInboxListOptions(wsId: string) {
  return queryOptions({
    queryKey: inboxKeys.archived(wsId),
    queryFn: () => api.listArchivedInbox({ workspaceId: wsId }),
    refetchInterval: 30_000,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
  });
}

export function snoozedInboxListOptions(wsId: string) {
  return queryOptions({
    queryKey: inboxKeys.snoozed(wsId),
    queryFn: () => api.listSnoozedInbox({ workspaceId: wsId }),
    refetchInterval: 30_000,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
  });
}

/**
 * Cross-workspace unread inbox summary. One cache entry shared across all
 * workspaces — the data is account-level, so switching workspaces does not
 * refetch it; only the derived "is this for another workspace" view changes.
 */
export function inboxUnreadSummaryOptions() {
  return queryOptions({
    queryKey: inboxKeys.unreadSummary(),
    queryFn: () => api.getInboxUnreadSummary(),
    refetchInterval: 30_000,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
  });
}

/**
 * Whether any workspace OTHER than `currentWsId` has unread inbox items.
 * Drives the workspace-switcher dot: the active workspace's own unread is
 * already surfaced by the Inbox nav count, so it is excluded here to avoid a
 * duplicate signal.
 */
export function hasOtherWorkspaceUnread(
  summary: InboxWorkspaceUnread[],
  currentWsId: string | null | undefined,
): boolean {
  return summary.some((s) => s.workspace_id !== currentWsId && s.count > 0);
}

/**
 * Set of workspace ids that have unread inbox items. Lets the workspace
 * switcher dropdown mark WHICH workspace a pending message lives in (the
 * aggregate switcher dot only says "somewhere else"). Workspaces with a zero
 * count are excluded.
 */
export function unreadWorkspaceIds(summary: InboxWorkspaceUnread[]): Set<string> {
  return new Set(summary.filter((s) => s.count > 0).map((s) => s.workspace_id));
}

/**
 * Unread inbox count for the given workspace, aligned with what the inbox
 * list UI renders: archived and future-snoozed groups excluded, then deduped so a
 * single issue with three unread notifications counts once.
 */
export function useInboxUnreadCount(wsId: string | null | undefined): number {
  const { data } = useQuery({
    ...inboxListOptions(wsId ?? ""),
    enabled: !!wsId,
    select: (items: InboxItem[]) =>
      deduplicateInboxItems(items).filter((i) => !i.read).length,
  });
  return data ?? 0;
}

/**
 * Deduplicate inbox items by issue_id (one entry per issue, Linear-style).
 * Exported for consumers to use in useMemo — not in queryOptions select
 * (to avoid new array references on every cache update).
 */
export function deduplicateInboxItems(items: InboxItem[]): InboxItem[] {
  const snoozed = snoozedGroupKeys(items);
  return groupInboxItemsByIssue(
    items.filter((i) => !i.archived && !snoozed.has(inboxGroupKey(i))),
  );
}

/**
 * Same grouping for the archived sub-view. The `archived` filter is what makes
 * an optimistic unarchive drop the row out of the archived list immediately —
 * exactly mirroring how `deduplicateInboxItems`' filter drops an optimistically
 * archived row out of the main list.
 */
export function deduplicateArchivedInboxItems(items: InboxItem[]): InboxItem[] {
  return groupInboxItemsByIssue(items.filter((i) => i.archived));
}

export function deduplicateSnoozedInboxItems(items: InboxItem[]): InboxItem[] {
  const states = new Map<string, InboxItem>();
  for (const item of items) {
    if (!item.archived && isInboxItemSnoozed(item)) states.set(inboxGroupKey(item), item);
  }
  return groupInboxItemsByIssue(
    items.filter((item) => !item.archived && states.has(inboxGroupKey(item))),
  ).map((item) => {
    const state = states.get(inboxGroupKey(item));
    return {
      ...item,
      snoozed_until: state?.snoozed_until,
      ...(state?.snooze_revision !== undefined ? { snooze_revision: state.snooze_revision } : {}),
    };
  }).sort((a, b) => inboxTimestamp(a.snoozed_until) - inboxTimestamp(b.snoozed_until));
}

function groupInboxItemsByIssue(items: InboxItem[]): InboxItem[] {
  const groups = new Map<string, InboxItem[]>();
  for (const item of items) {
    const key = inboxGroupKey(item);
    const group = groups.get(key) ?? [];
    group.push(item);
    groups.set(key, group);
  }
  const merged: InboxItem[] = [];
  for (const group of groups.values()) {
    group.sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
    const newest = group[0];
    if (!newest) continue;

    const commentId =
      newest.details?.comment_id ??
      group.find((item) => item.details?.comment_id)?.details?.comment_id;

    if (commentId && newest.details?.comment_id !== commentId) {
      merged.push({
        ...newest,
        details: { ...(newest.details ?? {}), comment_id: commentId },
      });
      continue;
    }

    merged.push(newest);
  }
  return merged.sort(
    (a, b) =>
      Math.max(inboxTimestamp(b.reminded_at), inboxTimestamp(b.created_at)) -
      Math.max(inboxTimestamp(a.reminded_at), inboxTimestamp(a.created_at)),
  );
}

function inboxGroupKey(item: InboxItem): string {
  return `${item.workspace_id}:${item.issue_id ? "issue" : "item"}:${item.issue_id ?? item.id}`;
}

function inboxTimestamp(value: string | null | undefined): number {
  const timestamp = value ? new Date(value).getTime() : NaN;
  return Number.isFinite(timestamp) ? timestamp : 0;
}

export function isInboxItemSnoozed(item: InboxItem, now = Date.now()): boolean {
  return inboxTimestamp(item.snoozed_until) > now;
}

function snoozedGroupKeys(items: InboxItem[]): Set<string> {
  return new Set(
    items.filter((item) => !item.archived && isInboxItemSnoozed(item)).map(inboxGroupKey),
  );
}
