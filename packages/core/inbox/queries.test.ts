// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import type { InboxItem, InboxWorkspaceUnread } from "../types";
import {
  deduplicateArchivedInboxItems,
  deduplicateInboxItems,
  deduplicateSnoozedInboxItems,
  hasOtherWorkspaceUnread,
  inboxKeys,
  unreadWorkspaceIds,
} from "./queries";

function item(overrides: Partial<InboxItem>): InboxItem {
  return {
    id: "inbox-1",
    workspace_id: "workspace-1",
    recipient_type: "member",
    recipient_id: "member-1",
    actor_type: "agent",
    actor_id: "agent-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    issue_status: null,
    read: false,
    archived: false,
    created_at: "2026-06-15T08:00:00Z",
    details: null,
    ...overrides,
  };
}

afterEach(() => vi.useRealTimers());

describe("deduplicateInboxItems", () => {
  it("keeps the newest issue row while preserving an older comment anchor", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "comment-notification",
        type: "new_comment",
        created_at: "2026-06-15T08:00:00Z",
        details: { comment_id: "comment-1" },
      }),
      item({
        id: "status-notification",
        type: "status_changed",
        created_at: "2026-06-15T08:01:00Z",
        details: { from: "in_progress", to: "in_review" },
      }),
    ]);

    expect(merged).toHaveLength(1);
    expect(merged[0]).toMatchObject({
      id: "status-notification",
      type: "status_changed",
      details: {
        from: "in_progress",
        to: "in_review",
        comment_id: "comment-1",
      },
    });
  });

  it("preserves the newest row's own comment anchor", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "older-comment",
        created_at: "2026-06-15T08:00:00Z",
        details: { comment_id: "comment-1" },
      }),
      item({
        id: "newer-comment",
        created_at: "2026-06-15T08:02:00Z",
        details: { comment_id: "comment-2" },
      }),
    ]);

    expect(merged).toHaveLength(1);
    expect(merged[0]?.id).toBe("newer-comment");
    expect(merged[0]?.details?.comment_id).toBe("comment-2");
  });

  it("drops archived rows so an optimistic archive leaves the list at once", () => {
    const merged = deduplicateInboxItems([
      item({ id: "active", issue_id: "issue-1" }),
      item({ id: "filed-away", issue_id: "issue-2", archived: true }),
    ]);

    expect(merged.map((i) => i.id)).toEqual(["active"]);
  });

  it("hides the whole future-snoozed group including a newer ordinary sibling", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-15T08:00:00Z"));
    const snoozed = {
      ...item({ id: "delayed" }),
      snoozed_until: "2026-06-15T09:00:00Z",
    };

    expect(
      deduplicateInboxItems([
        snoozed,
        item({ id: "ordinary-update", created_at: "2026-06-15T08:01:00Z" }),
        item({ id: "other-issue", issue_id: "issue-2" }),
      ]).map((entry) => entry.id),
    ).toEqual(["other-issue"]);
  });

  it("moves a reminded group forward without replacing its newest original event", () => {
    const reminder = {
      ...item({ id: "original-newest", created_at: "2026-06-15T07:00:00Z" }),
      reminded_at: "2026-06-15T09:00:00Z",
    };
    const olderAnchor = {
      ...item({
        id: "original-comment",
        created_at: "2026-06-15T06:00:00Z",
        details: { comment_id: "comment-before-delay" },
      }),
      reminded_at: "2026-06-15T09:00:00Z",
    };

    const result = deduplicateInboxItems([
      reminder,
      olderAnchor,
      item({ id: "recent-other", issue_id: "issue-2" }),
    ]);

    expect(result.map((entry) => entry.id)).toEqual(["original-newest", "recent-other"]);
    expect(result[0]?.created_at).toBe("2026-06-15T07:00:00Z");
    expect(result[0]?.details?.comment_id).toBe("comment-before-delay");
  });
});

describe("deduplicateSnoozedInboxItems", () => {
  it("groups all delayed siblings and sorts by the nearest reminder time", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-15T08:00:00Z"));
    const result = deduplicateSnoozedInboxItems([
      item({ id: "later", issue_id: "issue-2", snoozed_until: "2026-06-15T11:00:00Z" }),
      item({ id: "soon-old", snoozed_until: "2026-06-15T09:00:00Z" }),
      item({ id: "soon-new", created_at: "2026-06-15T08:05:00Z" }),
      item({ id: "active", issue_id: "issue-3" }),
      item({ id: "archived", issue_id: "issue-4", archived: true, snoozed_until: "2026-06-15T09:30:00Z" }),
    ]);

    expect(result.map((entry) => entry.id)).toEqual(["soon-new", "later"]);
    expect(result[0]?.snoozed_until).toBe("2026-06-15T09:00:00Z");
  });

  it("keeps issue-less notification state independent and releases expired entries", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-15T08:00:00Z"));
    const rows = [
      item({ id: "one", issue_id: null, snoozed_until: "2026-06-15T09:00:00Z" }),
      item({ id: "two", issue_id: null }),
      item({ id: "expired", issue_id: "issue-expired", snoozed_until: "2026-06-15T07:00:00Z" }),
    ];
    expect(deduplicateSnoozedInboxItems(rows).map((entry) => entry.id)).toEqual(["one"]);
    expect(deduplicateInboxItems(rows).map((entry) => entry.id)).toEqual(["two", "expired"]);
  });
});

describe("deduplicateArchivedInboxItems", () => {
  it("keeps only archived rows, one per issue, newest first", () => {
    const merged = deduplicateArchivedInboxItems([
      item({
        id: "archived-older",
        issue_id: "issue-1",
        archived: true,
        created_at: "2026-06-15T08:00:00Z",
      }),
      item({
        id: "archived-newer",
        issue_id: "issue-1",
        archived: true,
        created_at: "2026-06-15T09:00:00Z",
      }),
      item({
        id: "archived-other-issue",
        issue_id: "issue-2",
        archived: true,
        created_at: "2026-06-15T07:00:00Z",
      }),
      item({ id: "still-active", issue_id: "issue-3" }),
    ]);

    expect(merged.map((i) => i.id)).toEqual([
      "archived-newer",
      "archived-other-issue",
    ]);
  });

  it("drops a row the moment an optimistic unarchive flips it back", () => {
    // What useUnarchiveInbox's onMutate does: flip `archived` on the archived
    // cache. The row must leave this list without waiting for the refetch.
    const restored = item({ id: "restored", archived: false });

    expect(deduplicateArchivedInboxItems([restored])).toEqual([]);
  });

  it("groups issue-less notifications on their own id rather than merging them", () => {
    const merged = deduplicateArchivedInboxItems([
      item({ id: "standalone-1", issue_id: null, archived: true }),
      item({ id: "standalone-2", issue_id: null, archived: true }),
    ]);

    expect(merged).toHaveLength(2);
  });
});

describe("hasOtherWorkspaceUnread", () => {
  const summary = (entries: InboxWorkspaceUnread[]) => entries;

  it("is true when a workspace other than the active one has unread", () => {
    expect(
      hasOtherWorkspaceUnread(
        summary([{ workspace_id: "ws-2", count: 3 }]),
        "ws-1",
      ),
    ).toBe(true);
  });

  it("excludes the active workspace's own unread", () => {
    expect(
      hasOtherWorkspaceUnread(
        summary([{ workspace_id: "ws-1", count: 5 }]),
        "ws-1",
      ),
    ).toBe(false);
  });

  it("ignores other workspaces whose count is zero", () => {
    expect(
      hasOtherWorkspaceUnread(
        summary([{ workspace_id: "ws-2", count: 0 }]),
        "ws-1",
      ),
    ).toBe(false);
  });

  it("is true when at least one non-active workspace has unread", () => {
    expect(
      hasOtherWorkspaceUnread(
        summary([
          { workspace_id: "ws-1", count: 4 },
          { workspace_id: "ws-2", count: 1 },
        ]),
        "ws-1",
      ),
    ).toBe(true);
  });

  it("is false for an empty summary", () => {
    expect(hasOtherWorkspaceUnread([], "ws-1")).toBe(false);
  });

  it("counts every workspace as 'other' when there is no active workspace", () => {
    expect(
      hasOtherWorkspaceUnread(
        summary([{ workspace_id: "ws-1", count: 2 }]),
        null,
      ),
    ).toBe(true);
  });
});

describe("unreadWorkspaceIds", () => {
  it("collects only workspaces with a non-zero count", () => {
    const ids = unreadWorkspaceIds([
      { workspace_id: "ws-1", count: 0 },
      { workspace_id: "ws-2", count: 3 },
      { workspace_id: "ws-3", count: 1 },
    ]);
    expect(ids.has("ws-1")).toBe(false);
    expect(ids.has("ws-2")).toBe(true);
    expect(ids.has("ws-3")).toBe(true);
    expect(ids.size).toBe(2);
  });

  it("returns an empty set for an empty summary", () => {
    expect(unreadWorkspaceIds([]).size).toBe(0);
  });
});

describe("inboxKeys.unreadSummary", () => {
  it("is a stable account-level key independent of any workspace", () => {
    expect(inboxKeys.unreadSummary()).toEqual(["inbox", "unread-summary"]);
  });
});
