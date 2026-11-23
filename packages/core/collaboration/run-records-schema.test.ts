// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import * as collaboration from "./index";
import { collaborationRunRecordsFixture as fixture } from "./run-records-test-fixtures";

afterEach(() => setSchemaLogger(noopLogger));

describe.each(["comment", "result"] as const)("%s output attachment references", (kind) => {
  const record = kind === "comment"
    ? { id: "output:comment", kind, comment_id: "00000000-0000-4000-8000-0000000000c1", body: "Original Agent comment" }
    : { id: "output:result", kind, outcome: "success", body: "Original frozen result" };
  const ids = Array.from({ length: 32 }, (_, index) => `00000000-0000-4000-8000-${(index + 1).toString(16).padStart(12, "0")}`);

  it("preserves every original output ID, order and canonical identity through the public parser", () => {
    const parsed = collaboration.parseCollaborationRunRecords({ ...fixture, items: [{ ...record, attachment_ids: ids.map((id) => id.toUpperCase()) }] });
    expect(parsed?.items).toEqual([{ ...record, attachment_ids: ids }]);
  });

  it("preserves legacy absence separately from an explicit empty output set", () => {
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, items: [record] })?.items).toEqual([record]);
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, items: [{ ...record, attachment_ids: [] }] })?.items).toEqual([{ ...record, attachment_ids: [] }]);
  });

  it.each([
    { name: "null", value: null }, { name: "a non-array", value: ids[0] },
    { name: "a malformed ID", value: ["not-a-file"] },
    { name: "a duplicate ID", value: [ids[0], ids[0]] },
    { name: "a case-equivalent duplicate ID", value: [ids[9], ids[9]!.toUpperCase()] },
    { name: "33 original outputs", value: [...ids, "00000000-0000-4000-8000-0000000000ff"] },
  ])("refuses $name without dropping output provenance", ({ value }) => {
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, items: [{ ...record, attachment_ids: value }] })).toBeNull();
  });
});

it("preserves the original required input attachment variant independently of the output bound", () => {
  const input = fixture.items[0]!;
  const ids = Array.from({ length: 33 }, (_, index) => `00000000-0000-4000-8000-${(index + 1).toString(16).padStart(12, "0")}`);
  expect(collaboration.parseCollaborationRunRecords({ ...fixture, items: [{ ...input, attachment_ids: ids }] })?.items).toEqual([{ ...input, attachment_ids: ids }]);
  expect(collaboration.parseCollaborationRunRecords({ ...fixture, items: [{ ...input, attachment_ids: undefined }] })).toBeNull();
});

describe("source-recorded run pages", () => {
  it("validates checkpoint stream ordinals and explicit retention without inventing an unknown tail", () => {
    const stream = "00000000-0000-4000-8000-000000000009";
    const retention = { version: 1, stream_id: stream, retained_through: 1, observed_through: null, sealed: false, reason: "incomplete" };
    const page = { ...fixture, recording: { ...fixture.recording, retention }, items: [{ id: `record:${stream}:0`, kind: "message", role: "assistant", body: "checkpoint" }] };
    expect(collaboration.parseCollaborationRunRecords(page)?.recording.retention).toEqual(retention);
    for (const patch of [{ version: 2 }, { retained_through: -1 }, { observed_through: 0 }, { observed_through: 1 }, { reason: "unknown" }]) {
      expect(collaboration.parseCollaborationRunRecords({ ...page, recording: { ...page.recording, retention: { ...retention, ...patch } } })).toBeNull();
    }
    for (const id of [`record:${stream}:1`, `record:${stream}:01`, `record:${stream}:256`, "record:invalid:0"]) {
      expect(collaboration.parseCollaborationRunRecords({ ...page, items: [{ ...page.items[0], id }] })).toBeNull();
    }
  });
  it("retains the typed immutable input, process, result and exact recorded usage", () => {
    const parsed = collaboration.parseCollaborationRunRecords(fixture);
    expect(parsed).toEqual(fixture);
    expect(parsed?.usage?.input_tokens).toBe("9007199254740993");
    expect(parsed?.usage?.output_tokens).toBe("0");
    expect(parsed?.usage?.cache_read_tokens).toBeNull();
    expect(parsed?.usage?.cost_usd).toBe("1234567890123456789.1234567890");
  });

  it("distinguishes legacy results, absent inputs, partial recording and completed process coverage", () => {
    const legacy = { ...fixture, usage: null, recording: { status: "not_recorded", inputs_recorded: false }, items: [fixture.items.at(-1)] };
    expect(collaboration.parseCollaborationRunRecords(legacy)).toEqual(legacy);
    const partial = { ...legacy, run: { ...fixture.run, recording_incomplete: true, business_outcome: "failure" }, recording: { status: "partial", inputs_recorded: false } };
    expect(collaboration.parseCollaborationRunRecords(partial)).toEqual(partial);
    const failed = { ...fixture, run: { ...fixture.run, business_outcome: "failure" } };
    expect(collaboration.parseCollaborationRunRecords(failed)?.recording.status).toBe("complete");
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, usage: null, items: [] })).toMatchObject({ items: [], usage: null });
  });

  it("keeps only allowlisted provenance and record fields", () => {
    const parsed = collaboration.parseCollaborationRunRecords({ ...fixture,
      resource: { ...fixture.resource, token: "PRIVATE_TOKEN", installation_key: "PRIVATE_KEY", working_directory: "PRIVATE_DIRECTORY", owner_email: "PRIVATE_EMAIL" },
      items: [{ ...fixture.items[1], private_reasoning: "PRIVATE_REASONING", raw_payload: "PRIVATE_PAYLOAD" }],
      engine_messages: "PRIVATE_ENGINE_DATA",
    });
    expect(parsed?.resource).toEqual(fixture.resource);
    expect(parsed?.items).toEqual([fixture.items[1]]);
    expect(JSON.stringify(parsed)).not.toContain("PRIVATE_");
    const missing = { ...fixture, resource: { ...fixture.resource, provider: null, model: null, account_source: null } };
    expect(collaboration.parseCollaborationRunRecords(missing)?.resource).toEqual(missing.resource);
  });

  it.each([
    { protocol_version: 2 }, { issue_id: "invalid" }, { run: null }, { run: { ...fixture.run, id: "invalid" } },
    { run: { ...fixture.run, progress_state: "running" } }, { run: { ...fixture.run, proof_state: "unknown" } },
    { run: { ...fixture.run, parent_run_id: fixture.run.id } },
    { resource: { ...fixture.resource, agent_id: fixture.issue_id } },
    { resource: { ...fixture.resource, provider: "" } },
    { resource: { ...fixture.resource, model: "界".repeat(43) } },
    { resource: { ...fixture.resource, account_source: "owner_account" } },
    { recording: { status: "unknown", inputs_recorded: true } },
    { recording: { status: "complete" } }, { recording: { status: "complete", inputs_recorded: false } },
    { run: { ...fixture.run, recording_incomplete: true } },
    { recording: { status: "not_recorded", inputs_recorded: true } },
    { next_cursor: "" }, { next_cursor: "x".repeat(1025) }, { items: null },
    { items: [fixture.items[0], fixture.items[0]] },
    { items: [fixture.items[1], fixture.items[0]] },
    { items: [{ ...fixture.items[0], attachment_ids: [fixture.issue_id, fixture.issue_id] }] },
    { items: [{ ...fixture.items[1], kind: "provider_thinking" }] },
    { items: [{ ...fixture.items[1], role: "unknown" }] },
    { items: [{ ...fixture.items[1], id: "" }] },
    { items: [{ ...fixture.items[1], id: "x".repeat(201) }] },
    { items: [{ ...fixture.items[1], body: null }] },
    { items: [{ ...fixture.items[2], call_id: "" }] },
    { items: [{ ...fixture.items[3], is_error: undefined }] },
    { items: [{ ...fixture.items.at(-1), outcome: "stopped" }] },
  ])("rejects unsupported or internally inconsistent records: %j", (patch) => {
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, ...patch })).toBeNull();
  });

  it.each([0, -1, "-1", "01", "1.0", "1e3", "9223372036854775808", "PRIVATE_USAGE"])("rejects unrecorded or lossy counter values: %j", (input_tokens) => {
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, usage: { ...fixture.usage, input_tokens } })).toBeNull();
  });

  it.each([0, "-1", "+1", "01.2", "1e3", "NaN", "1.12345678901", "12345678901234567890"])("rejects invalid recorded decimal costs: %j", (cost_usd) => {
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, usage: { ...fixture.usage, cost_usd } })).toBeNull();
  });

  it("redacts invalid private record data without inventing an empty page", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(collaboration.parseCollaborationRunRecords({ ...fixture, items: [{ id: "event", kind: "message", role: "assistant", body: { secret: "PRIVATE_RECORD_SENTINEL" } }] })).toBeNull();
    expect(collaboration.parseCollaborationRunRecords(undefined)).toBeNull();
    expect(warn).toHaveBeenCalledTimes(2);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_RECORD_SENTINEL");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
  });
});
