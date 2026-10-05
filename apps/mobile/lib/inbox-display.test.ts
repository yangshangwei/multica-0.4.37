import { describe, expect, it } from "vitest";
import type { InboxItem } from "@multica/core/types";
import { deduplicateInboxItems, getInboxProjectTarget } from "./inbox-display";

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

describe("deduplicateInboxItems", () => {
  it("keeps the newest issue row while preserving an older comment anchor", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "comment-notification",
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
});

describe("project update notification target", () => {
  const projectId = "77777777-7777-4777-8777-777777777777";
  const notification = { type: "project_update", workspace_id: "workspace-1", details: { project_id: projectId, revision: "2" } };
  it("opens an authorized project notification without an issue id", () => {
    expect(getInboxProjectTarget(notification, "workspace-1")).toBe(projectId);
  });
  it("ignores foreign workspace, other notification types and malformed project ids", () => {
    expect(getInboxProjectTarget(notification, "workspace-2")).toBeNull();
    expect(getInboxProjectTarget(notification, null)).toBeNull();
    expect(getInboxProjectTarget({ ...notification, type: "new_comment" }, "workspace-1")).toBeNull();
    expect(getInboxProjectTarget({ ...notification, details: { project_id: "../../settings" } }, "workspace-1")).toBeNull();
    expect(getInboxProjectTarget({ ...notification, details: null }, "workspace-1")).toBeNull();
  });
});
