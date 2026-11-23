// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { noopLogger } from "../logger";
import { setSchemaLogger } from "../api/schema";
import { parseCollaborationCommandHistory } from "./command-history-schema";
import { collaborationCommandHistoryFixture as fixture, collaborationUnknownCancellationFixture as unknownCancellation } from "./command-history-test-fixtures";

const entry = fixture.items[0]!;
const other = "00000000-0000-4000-8000-000000000099";
const parseEntry = (changes: Record<string, unknown>) => parseCollaborationCommandHistory({ ...fixture, items: [{ ...entry, ...changes }] });
afterEach(() => setSchemaLogger(noopLogger));

describe("immutable command history API boundary", () => {
  it("retains the original body, typed target, exact decimal identities and complete cancellation evidence", () => {
    expect(parseCollaborationCommandHistory(fixture)).toEqual(fixture);
    expect(parseEntry({ seq: "9223372036854775807" })?.items[0]?.seq).toBe("9223372036854775807");
  });

  it("normalizes mixed-case retained attachment and cancellation UUIDs without changing evidence", () => {
    expect(parseEntry({ attachment_ids: entry.attachment_ids.map((id) => id.toUpperCase()),
      cancellation: { ...entry.cancellation, event_id: entry.cancellation.event_id.toUpperCase() } })).toEqual(fixture);
  });

  it("keeps absent legacy evidence and unavailable account summaries explicitly unknown", () => {
    expect(parseEntry({ author: null, reason: null, input_event_seq: null, comment_id: null, cancellation: unknownCancellation })?.items[0])
      .toMatchObject({ author_user_id: entry.author_user_id, author: null, reason: null, cancellation: unknownCancellation });
    expect(parseEntry({ cancellation: { ...entry.cancellation, actor: null } })?.items[0]?.cancellation?.actor_user_id)
      .toBe(entry.cancellation.actor_user_id);
  });

  it("distinguishes actual human-triggered and background automatic cancellation without inventing an actor", () => {
    const automatic = { ...entry.cancellation, kind: "automatic" };
    expect(parseEntry({ reason: "participant_removed", cancellation: automatic })?.items[0]?.cancellation?.actor_user_id).toBe(entry.cancellation.actor_user_id);
    expect(parseEntry({ reason: "credentials_revoked", cancellation: { ...automatic, actor: null, actor_user_id: null } })?.items[0]?.cancellation)
      .toMatchObject({ kind: "automatic", actor_user_id: null, actor: null, event_id: entry.cancellation.event_id });
  });

  it("keeps Switch placement separate from older cancellation identities and allows unread grouped waiting input", () => {
    const older = { ...entry, id: other, input_event_id: other, input_event_seq: null, comment_id: null,
      channel_id: other, batch_id: other, seq: "1", created_at: "2026-10-20T08:00:00Z", cancellation: unknownCancellation };
    expect(parseCollaborationCommandHistory({ ...fixture, items: [entry, older] })?.items.map((item) => item.channel_id))
      .toEqual([entry.channel_id, other]);
    expect(parseEntry({ state: "waiting", group_id: other, reason: null, cancellation: null })?.items[0]?.group_id).toBe(other);
  });

  it.each(["waiting", "in_flight", "succeeded", "failed", "stopped"])("accepts %s as a separate recorded state without cancellation evidence", (state) => {
    expect(parseEntry({ state, group_id: other, reason: null, cancellation: null })?.items[0]?.state).toBe(state);
  });

  it("allows distinct typed targets to retain the same original comment event and text", () => {
    const peer = { ...entry, id: other, target_id: other, target: { kind: "agent", resource_id: other },
      seq: "9007199254740994", state: "waiting", reason: null, cancellation: null };
    expect(parseCollaborationCommandHistory({ ...fixture, items: [peer, entry] })?.items.map((item) => item.content))
      .toEqual([entry.content, entry.content]);
  });

  it.each([
    { seq: 9007199254740992 }, { seq: "9223372036854775808" }, { seq: "01" }, { seq: "0" },
    { state: "suppressed" }, { state: "unknown" }, { target: { kind: "member", resource_id: other } },
    { attachment_ids: [other, other.toUpperCase()] }, { created_at: "not-a-time" },
    { author: { ...entry.author, user_id: other } }, { input_event_seq: null }, { comment_id: null },
    { cancellation: null }, { cancellation: undefined }, { state: "waiting" },
    { cancellation: { ...entry.cancellation, event_id: null } },
    { cancellation: { ...entry.cancellation, event_seq: entry.input_event_seq } },
    { cancellation: { ...entry.cancellation, event_id: entry.input_event_id } },
    { cancellation: { ...entry.cancellation, actor_user_id: null, actor: null } },
    { cancellation: { ...entry.cancellation, actor: { ...entry.cancellation.actor, user_id: other } } },
    { cancellation: { ...unknownCancellation, actor_user_id: other } },
    { cancellation: { ...unknownCancellation, created_at: entry.created_at } },
    { cancellation: { ...entry.cancellation, kind: "system" } },
    { reason: "participant_removed" },
  ])("rejects contradictory or malformed identity, state and cancellation evidence: %j", (changes) => {
    expect(parseEntry(changes)).toBeNull();
  });

  it("orders by exact timestamp and UUID rather than channel sequence or rounded milliseconds", () => {
    const older = { ...entry, id: other, seq: "9007199254740994", created_at: "2026-10-21T08:00:00.123455Z", cancellation: unknownCancellation };
    expect(parseCollaborationCommandHistory({ ...fixture, items: [entry, older] })).not.toBeNull();
    expect(parseCollaborationCommandHistory({ ...fixture, items: [older, entry] })).toBeNull();
    const tied = { ...older, created_at: "2026-10-21T16:00:00.123456+08:00" };
    expect(parseCollaborationCommandHistory({ ...fixture, items: [tied, entry] })).not.toBeNull();
    expect(parseCollaborationCommandHistory({ ...fixture, items: [entry, tied] })).toBeNull();
  });

  it("rejects duplicate, mixed-session and conflicting source/cancellation facts in one page", () => {
    const older = { ...entry, id: other, seq: "2", created_at: "2026-10-20T08:00:00Z", cancellation: unknownCancellation };
    for (const items of [
      [entry, entry], [entry, { ...older, session_id: other }], [entry, { ...older, seq: entry.seq }],
      [entry, { ...older, content: "Different immutable input" }], [entry, { ...older, comment_id: other }],
      [entry, { ...older, input_event_seq: "2" }], [entry, { ...older, target: { kind: "agent", resource_id: other } }],
      [entry, { ...older, cancellation: entry.cancellation }],
    ]) expect(parseCollaborationCommandHistory({ ...fixture, items })).toBeNull();
  });

  it("rejects unsupported or incomplete pages while stripping unrelated server fields", () => {
    for (const page of [null, {}, { ...fixture, protocol_version: 2 }, { ...fixture, next_cursor: "" },
      { ...fixture, next_cursor: "x".repeat(1025) }, { ...fixture, items: [], next_cursor: "more" }]) {
      expect(parseCollaborationCommandHistory(page)).toBeNull();
    }
    expect(parseCollaborationCommandHistory({ ...fixture, can_cancel: true, items: [{ ...entry, control_ai: true, private_configuration: "private" }] }))
      .toEqual(fixture);
  });

  it("redacts malformed private command values and validation details from schema logs", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseEntry({ content: "PRIVATE_ORIGINAL_COMMAND", cancellation: { ...entry.cancellation, actor_user_id: "PRIVATE_ACTOR" } })).toBeNull();
    expect(warn).toHaveBeenCalledWith("API response failed schema validation: collaboration command history", { endpoint: "collaboration command history" });
    expect(JSON.stringify(warn.mock.calls)).not.toMatch(/PRIVATE_|received|issues/);
  });
});
