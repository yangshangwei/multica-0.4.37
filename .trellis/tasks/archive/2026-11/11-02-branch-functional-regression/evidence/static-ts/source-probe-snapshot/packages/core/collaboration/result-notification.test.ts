// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { accountPath } from "../paths/account";
import { parseCollaborationNotifications } from "./content-schema";

const issueId = "00000000-0000-4000-8000-000000000001";
const runId = "00000000-0000-4000-8000-0000000000ab";
const item = { id: issueId, issue_id: issueId, event_seq: "9007199254740993", kind: "ai_completed", is_mention: false,
  source_kind: "issue", source_id: null, run_id: runId, issue_title: "Shared issue", can_open: true,
  created_at: "2026-10-19T00:00:00Z", read_at: null, archived_at: null };
const page = (notification: unknown) => ({ items: [notification], unread_count: 1, next_cursor: null });
afterEach(() => setSchemaLogger(noopLogger));

it.each(["ai_completed", "ai_failed", "ai_stopped"])("retains %s's exact representative run while preserving legacy notification sources", (kind) => {
  const parsed = parseCollaborationNotifications(page({ ...item, kind }));
  expect(parsed?.items[0]?.run_id).toBe(runId);
  expect(parsed?.items[0]?.source_kind).toBe("issue");
  expect(accountPath({ view: "participating", issueId, runId: parsed?.items[0]?.run_id })).toContain(`#run-${runId}`);
  expect(parseCollaborationNotifications(page({ ...item, kind, run_id: undefined }))?.items[0]?.run_id).toBeUndefined();
});

it.each([
  { run_id: null }, { run_id: "invalid" }, { is_mention: true }, { kind: "comment_created" },
  { source_kind: "comment", source_id: runId }, { source_kind: "body" }, { source_id: issueId },
  { can_open: false, issue_title: null },
])("refuses malformed or unauthorized result anchors: %j", (patch) => {
  expect(parseCollaborationNotifications(page({ ...item, ...patch }))).toBeNull();
});

it("drops a revoked notification's title and run anchor and redacts malformed responses", () => {
  expect(parseCollaborationNotifications(page({ ...item, run_id: undefined, can_open: false, issue_title: null }))?.items[0]).toMatchObject({ can_open: false, issue_title: null });
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationNotifications(page({ ...item, run_id: "PRIVATE_RUN", issue_title: "PRIVATE_TITLE" }))).toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});
