import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { ApiError } from "../api/client";
import { inboxKeys, isInboxItemSnoozed } from "./queries";
import { useWorkspaceId } from "../hooks";
import type { InboxItem, InboxSnoozeReceipt } from "../types";

export function useMarkInboxRead() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.markInboxRead(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.all(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      const prevArchived = qc.getQueryData<InboxItem[]>(inboxKeys.archived(wsId));
      const prevSnoozed = qc.getQueryData<InboxItem[]>(inboxKeys.snoozed(wsId));
      const markRead = (old: InboxItem[] | undefined) =>
        old?.map((item) => (item.id === id ? { ...item, read: true } : item));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), markRead);
      // Opening a notification from the archived sub-view marks it read too —
      // patch that cache as well, or its unread dot would sit there until the
      // next refetch.
      qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), markRead);
      qc.setQueryData<InboxItem[]>(inboxKeys.snoozed(wsId), markRead);
      return { prev, prevArchived, prevSnoozed };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
      if (ctx?.prevArchived) qc.setQueryData(inboxKeys.archived(wsId), ctx.prevArchived);
      if (ctx?.prevSnoozed) qc.setQueryData(inboxKeys.snoozed(wsId), ctx.prevSnoozed);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useRetrySourceContextQuickCreate() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (taskId: string) => api.retrySourceContextQuickCreate(taskId),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

/**
 * Flip a notification back to unread — the inverse of {@link useMarkInboxRead}.
 *
 * Same optimistic shape as marking read (predictable outcome, no navigation,
 * trivial rollback), and patches every list cache: an item can be actioned
 * from any view, and stale caches would disagree after a view switch.
 *
 * The unread badge is derived from the main list's cache (`useInboxUnreadCount`
 * dedupes it client-side), so the optimistic patch raises the badge without
 * waiting for the round-trip; `onSettled` re-pulls the cross-workspace summary,
 * which is server-computed and cannot be patched here.
 */
export function useMarkInboxUnread() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.markInboxUnread(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.all(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      const prevArchived = qc.getQueryData<InboxItem[]>(inboxKeys.archived(wsId));
      const prevSnoozed = qc.getQueryData<InboxItem[]>(inboxKeys.snoozed(wsId));
      const markUnread = (old: InboxItem[] | undefined) =>
        old?.map((item) => (item.id === id ? { ...item, read: false } : item));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), markUnread);
      qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), markUnread);
      qc.setQueryData<InboxItem[]>(inboxKeys.snoozed(wsId), markUnread);
      return { prev, prevArchived, prevSnoozed };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
      if (ctx?.prevArchived) qc.setQueryData(inboxKeys.archived(wsId), ctx.prevArchived);
      if (ctx?.prevSnoozed) qc.setQueryData(inboxKeys.snoozed(wsId), ctx.prevSnoozed);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      // The switcher dot must light again when the workspace goes back to
      // having unread items — that count lives on the server.
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useArchiveInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.archiveInbox(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.list(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      // Archive all items for the same issue (same behavior as store)
      const target = prev?.find((i) => i.id === id);
      const issueId = target?.issue_id;
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), (old) =>
        old?.map((item) =>
          item.id === id || (issueId && item.issue_id === issueId)
            ? { ...item, archived: true }
            : item,
        ),
      );
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
    },
    onSettled: () => {
      // Both lists: the item just moved from the main inbox into the archive.
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

/**
 * Restore an archived notification to the main inbox.
 *
 * Optimistic on the ARCHIVED cache only: flipping `archived` there makes the
 * row leave the archived list at once (the dedup helper filters on it), the
 * user stays put, and rollback is a single snapshot restore. The main list is
 * left to `onSettled` — its contents after a restore are the server's call
 * (which sibling rows come back, their read state, their order), so it is
 * invalidated rather than reconstructed client-side.
 */
export function useUnarchiveInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.unarchiveInbox(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.archived(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.archived(wsId));
      // Restore every sibling for the same issue — the server unarchives the
      // whole issue group, so the optimistic patch must too or the rest of the
      // group would linger in the archived list until the refetch lands.
      const target = prev?.find((i) => i.id === id);
      const issueId = target?.issue_id;
      qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), (old) =>
        old?.map((item) =>
          item.id === id || (issueId && item.issue_id === issueId)
            ? { ...item, archived: false }
            : item,
        ),
      );
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.archived(wsId), ctx.prev);
    },
    onSettled: () => {
      // Both lists: the item moves from one to the other, and the unread badge
      // rises again when it was archived unread.
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useMarkAllInboxRead() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.markAllInboxRead(),
    onMutate: async () => {
      await qc.cancelQueries({ queryKey: inboxKeys.list(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), (old) =>
        old?.map((item) =>
          !item.archived && !isInboxItemSnoozed(item) ? { ...item, read: true } : item,
        ),
      );
      return { prev };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

// The three batch-archive mutations below all move items into the archive, so
// each invalidates all list views and the unread summary on settle.
export function useArchiveAllInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.archiveAllInbox(),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useArchiveAllReadInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.archiveAllReadInbox(),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useArchiveCompletedInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.archiveCompletedInbox(),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

function useInboxSnoozeMutation<Variables>(
  wsId: string,
  mutationFn: (variables: Variables) => Promise<InboxSnoozeReceipt>,
) {
  const qc = useQueryClient();
  const refresh = async () => {
    await Promise.all([
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) }),
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() }),
    ]);
  };
  return useMutation({
    mutationFn,
    retry: false,
    onSuccess: refresh,
    onError: async (error) => {
      // Never restore a stale snapshot over another device's committed state.
      if (error instanceof ApiError && error.status === 409) await refresh();
    },
  });
}

export function useSnoozeInboxItem(wsId: string) {
  return useInboxSnoozeMutation(wsId, (input: { id: string; snoozedUntil: string; expectedRevision: number }) =>
    api.snoozeInboxItem(input.id, {
      snoozed_until: input.snoozedUntil,
      expected_revision: input.expectedRevision,
    }, { workspaceId: wsId }),
  );
}

export function useUnsnoozeInboxItem(wsId: string) {
  return useInboxSnoozeMutation(wsId, (input: { id: string; expectedRevision: number }) =>
    api.unsnoozeInboxItem(input.id, { expected_revision: input.expectedRevision }, { workspaceId: wsId }),
  );
}

export function useUndoInboxSnooze(wsId: string) {
  return useInboxSnoozeMutation(wsId, (input: { id: string; undoToken: string }) =>
    api.undoInboxSnooze(input.id, { undo_token: input.undoToken }, { workspaceId: wsId }),
  );
}
