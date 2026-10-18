// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationComments, parseCollaborationMentionTargets, parseCollaborationTriggerPreview, parseCollaborationCommentReceipt, parseCollaborationNotifications } from "./content-schema";

const id = (tail: number) => `00000000-0000-4000-8000-${String(tail).padStart(12, "0")}`;
const account = { user_id: id(1), name: "Participant", avatar_url: "", external: true };
const timestamp = "2026-10-17T00:00:00Z";
const comment = { id: id(2), issue_id: id(3), author_type: "member", author: account, content: "Private comment", type: "regular", parent_id: null, comment_revision: "9007199254740993", created_at: timestamp, updated_at: timestamp, deleted_at: null, resolved_at: null, resolved_by: null, can_edit: true, can_resolve: true, reactions: [] };
const target = { target_id: id(4), kind: "agent", resource_id: id(5), name: "Agent", target_epoch: "1", config_version: "1", queue_state: "paused", waiting_reason: "target_paused", execution_available: false, hold_reason: "initial", hold_version: "9007199254740993" };
afterEach(() => setSchemaLogger(noopLogger));

describe("private collaboration content boundaries", () => {
  it("retains exact versions, checks each comment belongs to the envelope, and rejects missing capabilities", () => {
    const page = { issue_id: id(3), items: [comment], next_cursor: null };
    expect(parseCollaborationComments(page)?.items[0]?.comment_revision).toBe("9007199254740993");
    expect(parseCollaborationComments({ ...page, issue_id: id(8) })).toBeNull();
    expect(parseCollaborationComments({ ...page, items: [{ ...comment, can_edit: undefined }] })).toBeNull();
  });

  it("accepts paused authorized candidates without claiming execution availability or accepting duplicate identities", () => {
    const candidates = { protocol_version: 2, issue_id: id(3), participants: [account], targets: [target], can_submit_commands: true };
    expect(parseCollaborationMentionTargets(candidates)?.targets[0]?.execution_available).toBe(false);
    expect(parseCollaborationMentionTargets({ ...candidates, targets: [target, target] })).toBeNull();
    expect(parseCollaborationMentionTargets({ ...candidates, protocol_version: 3 })).toBeNull();
  });

  it("separates a side-effect-free preview from accepted immutable inputs in a pending batch", () => {
    const result = { target_id: id(4), kind: "agent", resource_id: id(5), target_epoch: "1", config_version: "1", state: "waiting", code: "command_queued", command_id: null, hold_reason: "initial", hold_version: "9007199254740993" };
    const preview = { protocol_version: 2, issue_id: id(3), lifecycle_epoch: "1", targets: [result], recipients: [{ recipient: account, is_mention: true }] };
    expect(parseCollaborationTriggerPreview(preview)).not.toBeNull();
    expect(parseCollaborationTriggerPreview({ ...preview, targets: [{ ...result, command_id: id(6) }] })).toBeNull();
    const receipt = { status: 201, code: "comment_created", protocol_version: 2, issue_id: id(3), comment_id: id(2), comment_revision: "1", event_id: id(7), event_seq: "9", targets: [{ ...result, command_id: id(6), command_sequence: "1", channel_id: id(8), batch_id: id(9) }] };
    expect(parseCollaborationCommentReceipt(receipt)?.targets[0]?.batch_id).toBe(id(9));
    expect(parseCollaborationCommentReceipt({ ...receipt, targets: [result] })).toBeNull();
  });

  it("keeps hold feedback explicit and withholds it when the target is unavailable", () => {
    const candidates = { protocol_version: 2, issue_id: id(3), participants: [account], targets: [target], can_submit_commands: true };
    expect(parseCollaborationMentionTargets(candidates)?.targets[0]).toMatchObject({ hold_reason: "initial", hold_version: "9007199254740993", queue_state: "paused" });
    for (const patch of [{ hold_reason: undefined }, { hold_reason: "paused" }, { hold_version: null }, { hold_version: "0" }]) {
      expect(parseCollaborationMentionTargets({ ...candidates, targets: [{ ...target, ...patch }] })).toBeNull();
    }
    const hidden = { target_id: null, kind: "agent", resource_id: id(5), state: "blocked", code: "target_unavailable", command_id: null, hold_reason: null, hold_version: null };
    const preview = { protocol_version: 2, issue_id: id(3), lifecycle_epoch: "1", targets: [hidden], recipients: [] };
    expect(parseCollaborationTriggerPreview(preview)?.targets[0]?.hold_version).toBeNull();
    expect(parseCollaborationTriggerPreview({ ...preview, targets: [{ ...hidden, hold_version: "1" }] })).toBeNull();
    expect(parseCollaborationTriggerPreview({ ...preview, targets: [{ ...hidden, hold_reason: "grant_required" }] })).toBeNull();
  });

  it("keeps an unchanged edit receipt without inventing an event or weakening changed receipts", () => {
    const receipt = {
      status: 200, code: "unchanged", protocol_version: 2, issue_id: id(3),
      comment_id: id(2), comment_revision: "9007199254740993", event_id: null, targets: [],
    };
    expect(parseCollaborationCommentReceipt(receipt)?.comment_revision).toBe("9007199254740993");
    expect(parseCollaborationCommentReceipt({ ...receipt, event_id: undefined, event_seq: null })).not.toBeNull();
    for (const invalid of [
      { ...receipt, comment_id: undefined },
      { ...receipt, comment_revision: "0" },
      { ...receipt, event_id: id(7) },
      { ...receipt, event_seq: "1" },
      { ...receipt, targets: [{ target_id: null, kind: "agent", resource_id: id(5), state: "blocked", code: "target_unavailable", command_id: null }] },
      ...["comment_created", "comment_edited", "comment_deleted"].map((code) => ({ ...receipt, code })),
    ]) {
      expect(parseCollaborationCommentReceipt(invalid)).toBeNull();
    }
  });

  it("keeps a denied notification as a minimal record and never logs malformed private content", () => {
    const item = { id: id(1), issue_id: id(3), event_seq: "1", kind: "removed", is_mention: false, source_kind: "issue", source_id: null, issue_title: null, can_open: false, created_at: timestamp, read_at: null, archived_at: null };
    expect(parseCollaborationNotifications({ items: [item], unread_count: 1, next_cursor: null })).not.toBeNull();
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationNotifications({ items: [{ ...item, issue_title: "PRIVATE_SENTINEL", source_kind: "comment", source_id: id(2) }], unread_count: 1, next_cursor: null })).toBeNull();
    expect(parseCollaborationComments({ issue_id: id(3), items: [{ ...comment, content: { token: "PRIVATE_SENTINEL" } }], next_cursor: null })).toBeNull();
    expect(warn).toHaveBeenCalledTimes(2);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_SENTINEL");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
  });
});
