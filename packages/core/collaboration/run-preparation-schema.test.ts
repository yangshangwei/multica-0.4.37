// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationRunRecords } from "./run-records-schema";
import { collaborationRunRecordsFixture as fixture } from "./run-records-test-fixtures";

const summary = {
  version: 1,
  attachments: [{ id: "00000000-0000-4000-8000-000000000101", version: "2", name: "source.txt", size_bytes: 17 }],
  git: { object_format: "sha1", commit_oid: "b".repeat(40) },
  stages: [{ seq: "3", stage: "attachments", state: "prepared" },
    { seq: "4", stage: "git", state: "failed", code: "git_unavailable" }],
};

afterEach(() => setSchemaLogger(noopLogger));

describe("recorded preparation summary", () => {
  it("keeps only frozen public file, commit and factual stage fields", () => {
    expect(parseCollaborationRunRecords({ ...fixture, preparation: summary })).toMatchObject({ preparation: summary });
  });

  it("preserves legacy absent and explicit null summaries", () => {
    expect(parseCollaborationRunRecords(fixture)).toEqual(fixture);
    expect(parseCollaborationRunRecords({ ...fixture, preparation: null })).toEqual({ ...fixture, preparation: null });
  });

  it("strips private runtime data and never treats a preparation stage as outcome or proof", () => {
    const run = { ...fixture.run, progress_state: "preparing", proof_state: "unknown", business_outcome: null, settled_at: null };
    const parsed = parseCollaborationRunRecords({ ...fixture, run, preparation: {
      ...summary, manifest_digest: "PRIVATE_HASH", invocation: { instructions: "PRIVATE_INSTRUCTIONS" }, proof_state: "exited_proven",
      git: { ...summary.git, bundle_digest: "PRIVATE_BUNDLE_HASH", source_path: "PRIVATE_DIRECTORY" },
      attachments: [{ ...summary.attachments[0], destination: "PRIVATE_PATH", encryption_key: "PRIVATE_KEY" }],
      stages: summary.stages.map((stage) => ({ ...stage, attempt_nonce: "PRIVATE_NONCE", capability: "PRIVATE_CAPABILITY" })),
    } });
    expect(parsed?.preparation).toEqual(summary);
    expect(parsed?.run).toEqual(run);
    expect(JSON.stringify(parsed)).not.toContain("PRIVATE_");
  });

  it("keeps exact display names, fixed SHA-256 commits, large sequences and zero-sized files", () => {
    const preparation = { ...summary, attachments: [{ ...summary.attachments[0], name: " 文档.txt ", size_bytes: 0 }],
      git: { object_format: "sha256", commit_oid: "c".repeat(64) },
      stages: [{ seq: "9007199254740993", stage: "workspace", state: "prepared" }] };
    expect(parseCollaborationRunRecords({ ...fixture, preparation })?.preparation).toEqual(preparation);
    const planned = { ...summary, attachments: [], git: null, stages: [] };
    expect(parseCollaborationRunRecords({ ...fixture, preparation: planned })?.preparation).toEqual(planned);
  });

  it.each([
    { version: 2 }, { attachments: null }, { attachments: [summary.attachments[0], summary.attachments[0]] },
    { attachments: [{ ...summary.attachments[0], id: "invalid" }] },
    { attachments: [{ ...summary.attachments[0], version: "0" }] },
    { attachments: [{ ...summary.attachments[0], name: "  " }] },
    { attachments: [{ ...summary.attachments[0], name: "界".repeat(342) }] },
    { attachments: [{ ...summary.attachments[0], name: "NUL\0name" }] },
    { attachments: [{ ...summary.attachments[0], size_bytes: -1 }] },
    { attachments: [{ ...summary.attachments[0], size_bytes: "17" }] },
    { attachments: [{ ...summary.attachments[0], size_bytes: 100 * 1024 * 1024 + 1 }] },
    { attachments: Array.from({ length: 33 }, (_, index) => ({ ...summary.attachments[0], id: `00000000-0000-4000-8000-${String(index + 1).padStart(12, "0")}` })) },
    { attachments: Array.from({ length: 3 }, (_, index) => ({ ...summary.attachments[0], id: `00000000-0000-4000-8000-${String(index + 1).padStart(12, "0")}`, size_bytes: 100 * 1024 * 1024 })) },
    { git: null }, { git: { object_format: "sha1", commit_oid: "b".repeat(39) } },
    { git: { object_format: "sha256", commit_oid: "b".repeat(40) } },
    { git: { object_format: "sha1", commit_oid: "HEAD" } },
    { stages: null }, { stages: [{ ...summary.stages[0], stage: "provider" }] },
    { stages: [{ ...summary.stages[0], state: "running" }] },
    { stages: [{ ...summary.stages[0], seq: "private invalid sequence" }] },
    { stages: [{ ...summary.stages[0], seq: "9223372036854775808" }] },
    { stages: [{ ...summary.stages[0], code: "cancelled" }] },
    { stages: [{ ...summary.stages[0], state: "failed" }] },
    { stages: [{ ...summary.stages[1], code: "private failure" }] },
    { stages: [summary.stages[0], { ...summary.stages[0], seq: "5" }] },
    { stages: [summary.stages[1], summary.stages[0]] },
    { stages: [summary.stages[0], { ...summary.stages[1], seq: "3" }] },
  ])("rejects malformed or contradictory optional summary: %j", (patch) => {
    expect(parseCollaborationRunRecords({ ...fixture, preparation: { ...summary, ...patch } })).toBeNull();
  });

  it("requires frozen inputs and every recorded input's attachment in the manifest", () => {
    expect(parseCollaborationRunRecords({ ...fixture, preparation: summary, recording: { ...fixture.recording, inputs_recorded: false }, items: [] })).toBeNull();
    expect(parseCollaborationRunRecords({ ...fixture, preparation: summary, items: [{ ...fixture.items[0], attachment_ids: [fixture.issue_id] }] })).toBeNull();
    const included = { ...fixture, preparation: summary, items: [{ ...fixture.items[0], attachment_ids: [summary.attachments[0]!.id] }] };
    expect(parseCollaborationRunRecords(included)).toEqual(included);
  });

  it("redacts malformed preparation details in schema diagnostics", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationRunRecords({ ...fixture, preparation: { ...summary, stages: [{ ...summary.stages[0], state: "PRIVATE_STAGE" }] } })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_STAGE");
  });
});
