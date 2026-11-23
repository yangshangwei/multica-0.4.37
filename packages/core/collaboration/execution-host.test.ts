// @vitest-environment node
import { describe, expect, it } from "vitest";
import { CollaborationExecutionRunSchema, CollaborationExecutionSchema } from "./execution-schema";
import { parseCollaborationIssue } from "./schema";
import { collaborationExecutionFixture as legacy, collaborationExecutionIssueFixture as issue } from "./test-fixtures";

const mode = "host_process_supervision_v1";
const cleanup = { state: "best_effort", scope: "process_group", attempted: true };
const ended = {
  ...legacy.runs[0], execution_mode: mode, launch_count: "2", descendant_cleanup: cleanup,
  progress_state: "settled", proof_state: "host_exit_observed", settled_at: "2026-10-25T01:00:00Z",
};
const v6 = {
  ...legacy, protocol_version: 6,
  runs: legacy.runs.map((run, index) => ({ ...run, execution_mode: index === 0 ? mode : "linux_pid_namespace_v1", launch_count: "1", descendant_cleanup: null })),
};

describe("collaboration host execution views", () => {
  it("keeps v5 Linux reads and explicitly negotiated v6 graphs", () => {
    expect(CollaborationExecutionSchema.parse(legacy)).toEqual(legacy);
    expect(CollaborationExecutionSchema.safeParse(v6).success).toBe(true);
    expect(parseCollaborationIssue({ ...issue, execution: v6 })?.execution?.protocol_version).toBe(6);
  });

  it.each(["success", "failure", null])("keeps the business result independent of host termination: %s", (outcome) => {
    const result = CollaborationExecutionRunSchema.safeParse({ ...ended, business_outcome: outcome, recording_incomplete: outcome === null });
    expect(result.success).toBe(true);
    if (result.success) expect(result.data).toMatchObject({ business_outcome: outcome, proof_state: "host_exit_observed", descendant_cleanup: cleanup });
  });

  it("retains unknown managed work even when cleanup was attempted", () => {
    const run = { ...v6.runs[0], descendant_cleanup: cleanup };
    const result = CollaborationExecutionRunSchema.safeParse(run);
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.proof_state).toBe("unknown");
    expect(CollaborationExecutionSchema.safeParse({ ...v6, runs: [{ ...run, progress_state: "settled", settled_at: ended.settled_at }, ...v6.runs.slice(1)] }).success).toBe(false);
  });

  it.each([
    { execution_mode: "linux_pid_namespace_v1" },
    { execution_mode: "future_mode" },
    { execution_mode: undefined },
    { proof_state: "exited_proven" },
    { launch_count: "0" },
    { launch_count: "01" },
    { launch_count: 2 },
    { launch_count: undefined },
    { descendant_cleanup: null },
    { descendant_cleanup: undefined },
    { descendant_cleanup: { ...cleanup, attempted: false } },
    { descendant_cleanup: { ...cleanup, scope: "all_descendants" } },
    { descendant_cleanup: { ...cleanup, state: "all_gone" } },
  ])("rejects incomplete or cross-mode terminal claims: %j", (patch) => {
    expect(CollaborationExecutionRunSchema.safeParse({ ...ended, ...patch }).success).toBe(false);
  });

  it("does not strip v6 markers to accept a malformed host run as legacy", () => {
    for (const patch of [
      { execution_mode: mode }, { descendant_cleanup: cleanup }, { launch_count: "1" },
      { execution_mode: "future_mode", descendant_cleanup: null, launch_count: "1" },
    ]) expect(CollaborationExecutionRunSchema.safeParse({ ...legacy.runs[0], ...patch }).success).toBe(false);
    expect(CollaborationExecutionSchema.safeParse({ ...legacy, runs: v6.runs }).success).toBe(false);
    expect(CollaborationExecutionSchema.safeParse({ ...v6, runs: legacy.runs }).success).toBe(false);
  });

  it("requires a complete linked graph, even for a known host mode", () => {
    expect(CollaborationExecutionSchema.safeParse({ ...v6, runs: v6.runs.slice(1) }).success).toBe(false);
    expect(CollaborationExecutionSchema.safeParse({ ...v6, protocol_version: 7 }).success).toBe(false);
  });

  it("keeps a valid old-client unavailable projection readable for humans", () => {
    const old = { ...legacy, channels: [], targets: [], commands: [], groups: [], runs: [] };
    const result = parseCollaborationIssue({ ...issue, execution: old, capabilities: { ...issue.capabilities, control_ai: false, submit_command: false } });
    expect(result?.capabilities).toMatchObject({ read: true, comment: true, control_ai: false, submit_command: false });
    expect(result?.execution?.runs).toEqual([]);
  });
});
