import { describe, expect, it } from "vitest";
import { InboxListSchema } from "./schemas";
import { deduplicateInboxItems } from "../lib/inbox-display";
import projectInboxFixture from "./project-update-inbox.fixture.json";

/**
 * Tests for mobile's CLIENT-SIDE parsing of GET /api/inbox.
 *
 * Scope, stated precisely because the name of this file used to overclaim:
 * these are hand-written fixtures run against `InboxListSchema`. They pin how
 * this client REACTS to a given payload. They cannot fail when the Go server
 * starts sending something new — nothing here executes server code.
 *
 * The legacy notification listeners use `map[string]string`. P1's separate
 * outbox publisher must keep the same wire contract; the P1 regression below
 * mirrors CreateProjectUpdateInbox plus deliverProjectUpdateNotification.
 *
 * Why both halves exist: during MUL-5483 a new inbox type was added and the
 * mobile label map was updated so `tsc` passed — but a NUMBER went into
 * `details.child_count`, and `details` is `z.record(z.string(), z.string())`.
 * Because the endpoint parses an ARRAY, one bad row fails the whole parse and
 * `listInbox` falls back to `EMPTY_INBOX_LIST`: the entire mobile inbox
 * renders empty, not just that row. The blast radius is what these tests
 * document; the compile-time type is what prevents it.
 */
describe("inbox list schema", () => {
  // Captured from real worker delivery → authorized ListInbox HTTP response.
  // Source: .trellis/tasks/10-05-projects-p1-progress/evidence/inbox-wire.json
  const projectNotification = projectInboxFixture[0]!;
  const existingIssueNotification = projectInboxFixture[1]!;

  it("reproduces a numeric P1 revision rejecting the entire mixed inbox", () => {
    const broken = { ...projectNotification, details: { ...projectNotification.details, revision: 2 } };
    const parsed = InboxListSchema.safeParse([broken, existingIssueNotification]);
    expect(parsed.success).toBe(false);
    if (!parsed.success) expect(parsed.error.issues[0]?.path).toEqual([0, "details", "revision"]);
  });

  it("preserves P1 and existing notifications when revision follows the string wire contract", () => {
    const parsed = InboxListSchema.parse(projectInboxFixture);
    expect(parsed).toHaveLength(2);
    expect(parsed[0]).toMatchObject({ type: "project_update", issue_id: null, details: { revision: "1", project_id: projectNotification.details.project_id } });
    const visible = deduplicateInboxItems(parsed);
    expect(visible.map((item) => item.id)).toEqual([projectNotification.id, existingIssueNotification.id]);
    expect(visible.filter((item) => !item.read)).toHaveLength(2);
  });
  it("parses a row shaped like the documented server payload", () => {
    const serverRow = {
      id: "inbox-1",
      workspace_id: "ws-1",
      recipient_type: "member",
      recipient_id: "user-1",
      type: "status_changed",
      severity: "info",
      issue_id: "issue-1",
      title: "P0: delegated subscription rule",
      body: "",
      actor_type: "agent",
      actor_id: "agent-1",
      read: false,
      archived: false,
      created_at: "2026-07-30T00:00:00Z",
      // Every value is a string. A number here drops the whole list.
      details: { from: "in_progress", to: "in_review" },
    };

    const parsed = InboxListSchema.safeParse([serverRow]);
    expect(parsed.success).toBe(true);
    expect(parsed.success && parsed.data[0]?.type).toBe("status_changed");
    expect(parsed.success && parsed.data[0]?.details?.to).toBe("in_review");
  });

  it("rejects a numeric details value", () => {
    const badRow = {
      id: "inbox-2",
      recipient_type: "member",
      type: "status_changed",
      details: { child_count: 3 },
    };

    expect(InboxListSchema.safeParse([badRow]).success).toBe(false);
  });

  it("keeps one malformed row from emptying the entire list observable", () => {
    // Documents the blast radius that made this a P1 rather than a cosmetic bug:
    // the schema is an array, so a single bad row invalidates every good one.
    const good = {
      id: "inbox-3",
      recipient_type: "member",
      type: "status_changed",
      details: { from: "todo", to: "in_review" },
    };
    const bad = {
      id: "inbox-4",
      recipient_type: "member",
      type: "status_changed",
      details: { child_count: 3 },
    };

    expect(InboxListSchema.safeParse([good]).success).toBe(true);
    expect(InboxListSchema.safeParse([good, bad]).success).toBe(false);
  });

  it("renders an unknown server type instead of dropping the row", () => {
    // Mirrors the root CLAUDE.md API-compatibility rule and mobile's own
    // "render every inbox type, never silently drop a category" parity rule: a
    // type this build has never heard of must still parse.
    const future = {
      id: "inbox-5",
      recipient_type: "member",
      type: "some_future_type",
      details: { anything: "still a string" },
    };

    const parsed = InboxListSchema.safeParse([future]);
    expect(parsed.success).toBe(true);
  });
});
