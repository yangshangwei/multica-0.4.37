// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationIssue } from "./schema";
import { collaborationExecutionFixture as execution, collaborationExecutionIssueFixture as valid, collaborationIssueFixture as legacy } from "./test-fixtures";

const unknownId = "00000000-0000-4000-8000-000000000099";
const [agentTarget, squadTarget] = [execution.targets[0]!, execution.targets[1]!];
const [agentCommand, waitingCommand, squadCommand] = [execution.commands[0]!, execution.commands[1]!, execution.commands[2]!];
const [agentGroup, squadGroup] = [execution.groups[0]!, execution.groups[1]!];
const [agentRun, leaderRun, memberRun] = [execution.runs[0]!, execution.runs[1]!, execution.runs[2]!];
const parse = (value: unknown) => parseCollaborationIssue({ ...valid, execution: value });

afterEach(() => setSchemaLogger(noopLogger));

describe("collaboration execution protocol", () => {
  it("retains typed targets, independent roots and member proof barriers", () => {
    const result = parseCollaborationIssue(valid);
    expect(result?.execution).toEqual(execution);
    expect(result?.execution?.runs[1]?.proof_state).toBe("exited_proven");
    expect(result?.execution?.runs[2]?.proof_state).toBe("unknown");
    expect(result?.capabilities.control_ai).toBe(false);
    expect(result?.capabilities.submit_command).toBe(false);
    expect(result?.capabilities.edit).toBe(true);
    expect(result?.capabilities.complete).toBe(true);
  });

  it("keeps old-server human reads and workflow without execution affordances", () => {
    const result = parseCollaborationIssue(legacy);
    expect(result?.id).toBe(legacy.id);
    expect(result?.execution).toBeNull();
    expect(result?.capabilities).toMatchObject({ read: true, comment: true, edit: true, invite: true, transfer: true, control_ai: false, submit_command: false });
  });

  it.each([null, {}, [], "PRIVATE_EXECUTION", { ...execution, protocol_version: undefined }, { ...execution, protocol_version: 1 },
    { ...execution, protocol_version: 3 }, { ...execution, protocol_version: "2" }, { ...execution, availability: "available" },
    { ...execution, unavailable_reason: "future_reason" }, { ...execution, control: { ...execution.control, scope: "target" } },
  ])("refuses unknown or malformed present protocols", (value) => {
    expect(parse(value)).toBeNull();
  });

  it.each(["targets", "commands", "groups", "runs", "legacy_unresolved"])("never treats missing or malformed %s as empty", (field) => {
    for (const value of [undefined, null, "PRIVATE_COLLECTION", 1]) {
      expect(parse({ ...execution, [field]: value })).toBeNull();
    }
  });

  it("accepts complete empty collections without granting execution", () => {
    const empty = { ...execution, targets: [], commands: [], groups: [], runs: [] };
    expect(parse(empty)?.execution).toEqual(empty);
    expect(parse(empty)?.capabilities.control_ai).toBe(false);
  });

  it("requires the same issue, session and issue-control snapshot", () => {
    expect(parse({ ...execution, issue_id: unknownId })).toBeNull();
    expect(parse({ ...execution, session_id: unknownId })).toBeNull();
    expect(parse({ ...execution, control: { ...execution.control, queue_state: "active" } })).toBeNull();
    expect(parse({ ...execution, control: { ...execution.control, control_version: "9" } })).toBeNull();
  });

  it("keeps versions beyond safe integers and the exact signed-64-bit maximum", () => {
    expect(parse(execution)?.execution?.target_set_version).toBe("9007199254740999");
    expect(parse(execution)?.execution?.targets[0]?.config_version).toBe("9007199254740993");
    expect(parse({ ...execution, target_set_version: "9223372036854775807" })?.execution?.target_set_version).toBe("9223372036854775807");
    expect(parse({ ...execution, targets: [{ ...agentTarget, group_generation: "9007199254740993" }, squadTarget],
      groups: [{ ...agentGroup, generation: "9007199254740993" }, squadGroup] })?.execution?.groups[0]?.generation).toBe("9007199254740993");
    expect(parse({ ...execution, targets: [{ ...agentTarget, group_generation: "10" }, squadTarget],
      groups: [{ ...agentGroup, generation: "9", state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup] })).toBeNull();
  });

  it.each(["0", "01", "1e3", "1.5", "-1", "+1", " 1", "1\n", "9223372036854775808", "9".repeat(1000), 1, null, undefined])("rejects malformed version %s at every execution boundary", (value) => {
    expect(parse({ ...execution, target_set_version: value })).toBeNull();
    expect(parse({ ...execution, control: { ...execution.control, control_version: value } })).toBeNull();
    for (const field of ["target_epoch", "config_version", "next_command_seq"]) {
      expect(parse({ ...execution, targets: [{ ...agentTarget, [field]: value }, squadTarget] })).toBeNull();
    }
    expect(parse({ ...execution, targets: [{ ...agentTarget, control: { ...agentTarget.control, control_version: value } }, squadTarget] })).toBeNull();
    expect(parse({ ...execution, commands: [{ ...agentCommand, sequence: value }, waitingCommand, squadCommand] })).toBeNull();
    for (const field of ["generation", "target_epoch", "config_version"]) {
      expect(parse({ ...execution, groups: [{ ...agentGroup, [field]: value }, squadGroup] })).toBeNull();
    }
    for (const field of ["target_epoch", "config_version"]) {
      expect(parse({ ...execution, runs: [{ ...agentRun, [field]: value }, leaderRun, memberRun] })).toBeNull();
    }
  });

  it("accepts generation zero only before the first group", () => {
    const empty = { ...execution, targets: [{ ...agentTarget, group_generation: "0", active_group_id: null, next_command_seq: "1" }], commands: [], groups: [], runs: [] };
    expect(parse(empty)?.execution?.targets[0]?.group_generation).toBe("0");
    expect(parse({ ...empty, targets: [{ ...empty.targets[0], group_generation: "-1" }] })).toBeNull();
    expect(parse({ ...execution, targets: [{ ...agentTarget, group_generation: "0" }, squadTarget] })).toBeNull();
  });

  it("rejects duplicate identities, target FIFO positions and per-comment targets", () => {
    for (const field of ["targets", "commands", "groups", "runs"] as const) {
      expect(parse({ ...execution, [field]: [...execution[field], execution[field][0]] })).toBeNull();
    }
    expect(parse({ ...execution, targets: [...execution.targets, { ...agentTarget, id: unknownId, active_group_id: null }] })).toBeNull();
    expect(parse({ ...execution, targets: [...execution.targets, { ...agentTarget, id: agentTarget.id.toUpperCase() }] })).toBeNull();
    expect(parse({ ...execution, commands: [...execution.commands, { ...agentCommand, id: unknownId, input_event_id: unknownId }] })).toBeNull();
    expect(parse({ ...execution, commands: [...execution.commands, { ...agentCommand, id: unknownId, sequence: "2" }] })).toBeNull();
    expect(parse({ ...execution, runs: [...execution.runs, { ...memberRun, id: unknownId }] })).toBeNull();
  });

  it("rejects detached commands, cross-target groups and contradictory active pointers", () => {
    expect(parse({ ...execution, commands: [{ ...agentCommand, target_id: unknownId }, waitingCommand, squadCommand] })).toBeNull();
    expect(parse({ ...execution, groups: [{ ...agentGroup, target_id: squadTarget.id }, squadGroup] })).toBeNull();
    expect(parse({ ...execution, groups: [{ ...agentGroup, command_id: unknownId }, squadGroup] })).toBeNull();
    expect(parse({ ...execution, targets: [{ ...agentTarget, active_group_id: squadGroup.id }, squadTarget] })).toBeNull();
    expect(parse({ ...execution, targets: [{ ...agentTarget, active_group_id: null }, squadTarget] })).toBeNull();
    expect(parse({ ...execution, groups: [] })).toBeNull();
    expect(parse({ ...execution, commands: [{ ...agentCommand, state: "waiting" }, waitingCommand, squadCommand] })).toBeNull();
    expect(parse({ ...execution, groups: [{ ...agentGroup, generation: "2" }, squadGroup] })).toBeNull();
  });

  it("rejects mismatched run lineage, invalid role parents and cycles", () => {
    for (const patch of [
      { target_id: squadTarget.id }, { command_id: squadCommand.id }, { group_id: squadGroup.id },
      { parent_run_id: leaderRun.id }, { step_kind: "leader" }, { config_version: "1" }, { target_epoch: "1" },
    ]) expect(parse({ ...execution, runs: [{ ...agentRun, ...patch }, leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...execution, runs: [agentRun, leaderRun, { ...memberRun, parent_run_id: unknownId }] })).toBeNull();
    expect(parse({ ...execution, runs: [agentRun, leaderRun, { ...memberRun, parent_run_id: agentRun.id }] })).toBeNull();
    expect(parse({ ...execution, runs: [agentRun, leaderRun, { ...memberRun, parent_run_id: null }] })).toBeNull();
    expect(parse({ ...execution, runs: [agentRun, leaderRun, { ...memberRun, parent_run_id: memberRun.id }] })).toBeNull();
    const continuation = { ...memberRun, id: unknownId, step_kind: "continuation", step_key: "continue", parent_run_id: memberRun.id };
    expect(parse({ ...execution, runs: [agentRun, leaderRun, memberRun, continuation] })?.execution?.runs).toHaveLength(4);
    expect(parse({ ...execution, runs: [agentRun, leaderRun, { ...memberRun, parent_run_id: continuation.id }, continuation] })).toBeNull();
  });

  it("does not turn progress, a leader proof or a terminal group label into proven settlement", () => {
    expect(parse({ ...execution, runs: [{ ...agentRun, proof_state: "future_proof" }, leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...execution, runs: [{ ...agentRun, progress_state: "future_progress" }, leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...execution, runs: [{ ...agentRun, progress_state: "settled", settled_at: "2026-10-17T00:00:00Z" }, leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...execution, groups: [agentGroup, { ...squadGroup, state: "stopped", settled_at: "2026-10-17T00:00:00Z" }] })).toBeNull();
    expect(parse(execution)?.execution?.runs.find((run) => run.id === memberRun.id)?.settled_at).toBeNull();
  });

  it("rejects a terminal group with omitted run evidence while preserving active preparation", () => {
    const terminal = { ...execution,
      targets: [{ ...agentTarget, active_group_id: null }, squadTarget],
      commands: [{ ...agentCommand, state: "stopped" }, waitingCommand, squadCommand],
      groups: [{ ...agentGroup, state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup],
      runs: [leaderRun, memberRun],
    };
    expect(parse(terminal)).toBeNull();
    const stoppedRun = { ...agentRun, progress_state: "settled", proof_state: "exited_proven", settled_at: "2026-10-17T00:00:00Z" };
    expect(parse({ ...terminal, runs: [stoppedRun, leaderRun, memberRun] })?.execution?.groups[0]?.state).toBe("stopped");
    const preparing = { ...execution, groups: [{ ...agentGroup, state: "active", expansion_sealed_at: null, forced_disposition: null }, squadGroup], runs: [leaderRun, memberRun] };
    expect(parse(preparing)?.execution?.groups[0]?.state).toBe("active");
  });

  it("compares historical generations numerically without narrowing large values", () => {
    const history = { ...execution,
      targets: [{ ...agentTarget, group_generation: "10", active_group_id: null }, squadTarget],
      commands: [{ ...agentCommand, state: "stopped" }, waitingCommand, squadCommand],
      groups: [{ ...agentGroup, generation: "9", state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup],
      runs: [{ ...agentRun, progress_state: "settled", proof_state: "exited_proven", settled_at: "2026-10-17T00:00:00Z" }, leaderRun, memberRun],
    };
    expect(parse(history)?.execution?.groups[0]?.generation).toBe("9");
    expect(parse({ ...history, groups: [{ ...history.groups[0], generation: "11" }, squadGroup] })).toBeNull();
    const large = { ...history, targets: [{ ...history.targets[0], group_generation: "9007199254740993" }, squadTarget],
      groups: [{ ...history.groups[0], generation: "9007199254740992" }, squadGroup] };
    expect(parse(large)?.execution?.groups[0]?.generation).toBe("9007199254740992");
    expect(parse({ ...large, groups: [{ ...history.groups[0], generation: "9007199254740994" }, squadGroup] })).toBeNull();
  });

  it("preserves revoked-target history without treating a target as a resource grant", () => {
    const revoked = { ...agentTarget, revoked_at: "2026-10-17T00:00:00Z", target_epoch: "9007199254740994" };
    const result = parse({ ...execution, targets: [revoked, squadTarget] });
    expect(result?.execution?.targets[0]).toEqual(revoked);
    expect(result?.execution?.runs[0]?.proof_state).toBe("unknown");
    expect(result?.capabilities).toMatchObject({ read: true, comment: true, control_ai: false, submit_command: false });
  });

  it("retains unresolved legacy barriers without guessing a target", () => {
    const unresolved = { command_ids: [unknownId], run_ids: [unknownId], active_run_id: unknownId };
    expect(parse({ ...execution, legacy_unresolved: unresolved })?.execution?.legacy_unresolved).toEqual(unresolved);
    expect(parse({ ...execution, legacy_unresolved: { ...unresolved, run_ids: [] } })).toBeNull();
    expect(parse({ ...execution, legacy_unresolved: { ...unresolved, command_ids: [agentCommand.id] } })).toBeNull();
    expect(parse({ ...execution, legacy_unresolved: { ...unresolved, run_ids: [memberRun.id], active_run_id: memberRun.id } })).toBeNull();
    expect(parse({ ...execution, legacy_unresolved: { ...unresolved, run_ids: [unknownId, unknownId] } })).toBeNull();
  });

  it("discards private machine fields and logs no values on malformed execution", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    const extra = { ...execution, targets: [{ ...agentTarget, roster: "PRIVATE_ROSTER", runtime_path: "PRIVATE_PATH" }, squadTarget],
      runs: [{ ...agentRun, launch_nonce: "PRIVATE_NONCE", token: "PRIVATE_TOKEN", grant: "PRIVATE_GRANT" }, leaderRun, memberRun] };
    expect(JSON.stringify(parse(extra))).not.toContain("PRIVATE_");
    expect(parse({ ...extra, protocol_version: "PRIVATE_PROTOCOL" })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
  });
});
