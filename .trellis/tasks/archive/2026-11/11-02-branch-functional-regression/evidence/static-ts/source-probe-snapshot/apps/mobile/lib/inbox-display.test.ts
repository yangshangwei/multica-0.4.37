// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { InboxItem } from "@multica/core/types";
import { deduplicateInboxItems, getInboxIterationTarget, getInboxProjectTarget } from "./inbox-display";

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

describe("iteration notification target", () => {
  const iterationId = "88888888-8888-4888-8888-888888888888";
  const notification = item({
    type: "iteration",
    issue_id: null,
    details: { iteration_id: iterationId, kind: "end" },
  });

  it.each(["start", "end", "cancel", "dates_changed", "overdue"])(
    "recognizes a current-workspace %s notice without an issue id",
    (kind) => {
      expect(getInboxIterationTarget({
        ...notification,
        details: { iteration_id: iterationId, kind },
      }, "workspace-1")).toEqual({ kind: "detail", iterationId });
    },
  );

  it("recognizes workspace disable notices without an iteration id", () => {
    expect(getInboxIterationTarget({
      ...notification,
      details: { kind: "disable" },
    }, "workspace-1")).toEqual({ kind: "list" });
  });

  it("ignores unavailable workspaces, invalid targets and unknown notice kinds", () => {
    expect(getInboxIterationTarget(notification, null)).toBeNull();
    expect(getInboxIterationTarget(notification, "workspace-2")).toBeNull();
    expect(getInboxIterationTarget({ ...notification, type: "new_comment" }, "workspace-1")).toBeNull();
    const invalidDetails: InboxItem["details"][] = [
      null,
      { kind: "end" },
      { kind: "end", iteration_id: "../../settings" },
      { iteration_id: iterationId },
      { kind: "future_event", iteration_id: iterationId },
    ];
    for (const details of invalidDetails) {
      expect(getInboxIterationTarget({ ...notification, details }, "workspace-1")).toBeNull();
    }
  });

  it("leaves the read state and retained notification payload untouched", () => {
    const details = Object.freeze({ ...notification.details, future_field: "retained" });
    const retained = Object.freeze({ ...notification, details });
    expect(getInboxIterationTarget(retained, "workspace-1")).toEqual({ kind: "detail", iterationId });
    expect(retained.read).toBe(false);
    expect(retained.details).toBe(details);
    expect(retained.details.future_field).toBe("retained");
  });
});
