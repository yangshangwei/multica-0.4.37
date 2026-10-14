// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { CommentSchema, IssueSchema } from "../api/schemas";
import { noopLogger } from "../logger";
import { parseCollaborationIssue, parseCollaborationOperation, parseCollaborationWorkbench } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";

const id = "00000000-0000-4000-8000-000000000001";
const valid = collaborationIssueFixture;
const invalidVersions = ["0", "01", "1.1", "1e3", "-1", "+1", "1\n", "1\r", " 1", "1 ", "", "9223372036854775808", "1".repeat(1000), 1, null, undefined];

afterEach(() => setSchemaLogger(noopLogger));

describe("collaboration response boundary", () => {
  it("keeps versions beyond the JS safe integer range exact", () => {
    const result = parseCollaborationIssue(valid);
    expect(result?.collaboration_revision).toBe("9007199254740993");
    expect(result?.session.control_version).toBe("9007199254740995");
  });

  it.each(invalidVersions)("rejects invalid version %s", (version) => {
    expect(parseCollaborationIssue({ ...valid, collaboration_revision: version })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, lifecycle_epoch: version })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, session: { ...valid.session, control_version: version } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, write_context: { participant_epoch: version, account_epoch: "1" } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, write_context: { participant_epoch: "1", account_epoch: version } })).toBeNull();
    expect(parseCollaborationOperation({ code: "transferred", collaboration_revision: version })).toBeNull();
  });

  it("accepts the exact positive signed 64-bit boundary", () => {
    expect(parseCollaborationIssue({ ...valid, collaboration_revision: "9223372036854775807" })?.collaboration_revision).toBe("9223372036854775807");
    expect(parseCollaborationIssue({ ...valid, write_context: { participant_epoch: "9223372036854775807", account_epoch: "9007199254740993" } })?.write_context)
      .toEqual({ participant_epoch: "9223372036854775807", account_epoch: "9007199254740993" });
    expect(parseCollaborationOperation({ code: "transferred", collaboration_revision: "9223372036854775807" })?.collaboration_revision).toBe("9223372036854775807");
  });

  it("keeps a missing or null write context unavailable without inventing epochs", () => {
    expect(parseCollaborationIssue(valid)?.write_context).toBeUndefined();
    expect(parseCollaborationIssue({ ...valid, write_context: null })?.write_context).toBeNull();
  });

  it.each([{}, [], "PRIVATE_CONTEXT", { participant_epoch: "1" }, { account_epoch: "1" }])("rejects a malformed present write context", (writeContext) => {
    expect(parseCollaborationIssue({ ...valid, write_context: writeContext })).toBeNull();
  });

  it.each(["participant_added", "already_participant", "transferred", "already_handler", "participant_removed", "participant_left", "stale_revision", "forbidden", "future_result"])("accepts an operation receipt with result code %s", (code) => {
    expect(parseCollaborationOperation({ code, collaboration_revision: "9007199254740993", description: "PRIVATE_BODY" }))
      .toEqual({ code, collaboration_revision: "9007199254740993" });
  });

  it.each(["", " ", "\n", 1, null, undefined])("rejects an unusable operation result code %s", (code) => {
    expect(parseCollaborationOperation({ code, collaboration_revision: "1" })).toBeNull();
  });

  it("does not turn malformed identity or authority into ordinary/default access", () => {
    expect(parseCollaborationIssue({ ...valid, id: "not-an-id" })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, capabilities: { ...valid.capabilities, complete: "true" } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, session: { ...valid.session, queue_state: "future-state" } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, session: { ...valid.session, id: null } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, capabilities: undefined })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, capabilities: { ...valid.capabilities, read: undefined } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, creator: { ...valid.creator, user_id: "bad" } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, handler: { ...valid.handler, external: "false" } })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, source_workspace: { ...valid.source_workspace, id: "bad" } })).toBeNull();
  });

  it("degrades optional display fields without retaining private account fields", () => {
    const result = parseCollaborationIssue({
      ...valid,
      creator: { ...valid.creator, name: null, avatar_url: null, email: "PRIVATE_EMAIL", username: "PRIVATE_USERNAME" },
      handler: { ...valid.handler, email: "PRIVATE_EMAIL", username: "PRIVATE_USERNAME" },
    });
    expect(result?.creator).toEqual({ user_id: id, name: "Unavailable account", avatar_url: "", external: true });
    expect(JSON.stringify(result)).not.toContain("PRIVATE_");
  });

  it("does not log private response values on schema failure", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationIssue({ ...valid, id: "bad", description: "PRIVATE_SENTINEL" })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_SENTINEL");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
    expect(parseCollaborationWorkbench({ items: [{ title: "PRIVATE_WORKBENCH_SENTINEL" }], total: 1, next_cursor: null })).toBeNull();
    expect(parseCollaborationOperation({ code: "PRIVATE_CODE_SENTINEL", collaboration_revision: "PRIVATE_VERSION_SENTINEL" })).toBeNull();
    expect(parseCollaborationIssue({ ...valid, write_context: { participant_epoch: "PRIVATE_EPOCH_SENTINEL" } })).toBeNull();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
  });

  it("preserves the legacy numeric issue revision in mixed lists", () => {
    const ordinary = {
      id, workspace_id: id, number: 1, identifier: "SRC-1", title: "Ordinary",
      description: null, status: "todo", priority: "none", assignee_type: null,
      assignee_id: null, creator_type: "member", creator_id: id, parent_issue_id: null,
      project_id: null, position: 0, start_date: null, due_date: null,
      metadata: {}, properties: {}, created_at: "2026-10-13T00:00:00Z",
      updated_at: "2026-10-13T00:00:00Z", revision: 1,
    };
    const result = IssueSchema.array().safeParse([ordinary, { ...ordinary, collaboration_revision: valid.collaboration_revision }]);
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.map((item) => item.revision)).toEqual([1, 1]);
  });

  it("keeps legacy comment revision numeric when collaboration metadata is additive", () => {
    const comment = {
      id, issue_id: id, author_type: "member", author_id: id, content: "Comment",
      type: "comment", parent_id: null, created_at: "2026-10-13T00:00:00Z", updated_at: "2026-10-13T00:00:00Z", revision: 3,
    };
    const result = CommentSchema.array().safeParse([comment, { ...comment, comment_revision: "9007199254740993" }]);
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.map((item) => item.revision)).toEqual([3, 3]);
    expect(CommentSchema.safeParse({ ...comment, revision: "3" }).success).toBe(false);
  });

  it("distinguishes an empty workbench from malformed data", () => {
    expect(parseCollaborationWorkbench({ items: [], total: 0, next_cursor: null })).toEqual({ items: [], total: 0, next_cursor: null });
    expect(parseCollaborationWorkbench({ items: "not-an-array", total: 0, next_cursor: null })).toBeNull();
    expect(parseCollaborationWorkbench({ items: [], total: -1, next_cursor: null })).toBeNull();
    const item = {
      id, title: valid.title, status: valid.status, status_category: valid.status_category,
      priority: valid.priority, collaboration_revision: valid.collaboration_revision,
      updated_at: "2026-10-13T00:00:00Z", source_workspace: valid.source_workspace, handler: valid.handler,
    };
    expect(parseCollaborationWorkbench({ items: [item], total: 1, next_cursor: null })?.items[0]?.collaboration_revision).toBe(valid.collaboration_revision);
    expect(parseCollaborationWorkbench({ items: [{ ...item, id: "bad" }], total: 1, next_cursor: null })).toBeNull();
  });
});
