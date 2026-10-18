// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationIssue } from "./schema";
import { collaborationExecutionFixture as execution, collaborationExecutionIssueFixture as valid, collaborationIssueFixture as legacy } from "./test-fixtures";

const unknownId = "00000000-0000-4000-8000-000000000099";
const [agentTarget, squadTarget] = [execution.targets[0]!, execution.targets[1]!];
const [agentChannel, squadChannel] = [execution.channels[0]!, execution.channels[1]!];
const [agentCommand, waitingCommand, squadCommand] = [execution.commands[0]!, execution.commands[1]!, execution.commands[2]!];
const [agentGroup, squadGroup] = [execution.groups[0]!, execution.groups[1]!];
const [agentRun, leaderRun, memberRun] = [execution.runs[0]!, execution.runs[1]!, execution.runs[2]!];
const parse = (value: unknown) => parseCollaborationIssue({ ...valid, execution: value });

afterEach(() => setSchemaLogger(noopLogger));

describe("collaboration execution protocol", () => {
  it("requires current human authority and a supported snapshot for controls while AI submission stays gated", () => {
    const context = { participant_epoch: "1", account_epoch: "2" };
    const supported = { ...valid, write_context: context, execution: { ...execution, execution_gate: { ...execution.execution_gate, enabled: false } } };
    expect(parseCollaborationIssue(supported)?.capabilities).toMatchObject({ control_ai: true, submit_command: false });
    expect(parseCollaborationIssue({ ...supported, write_context: null })?.capabilities.control_ai).toBe(false);
    expect(parseCollaborationIssue({ ...supported, capabilities: { ...supported.capabilities, control_ai: false } })?.capabilities.control_ai).toBe(false);
    expect(parseCollaborationIssue({ ...supported, execution: undefined })?.capabilities.control_ai).toBe(false);
    expect(parseCollaborationIssue({ ...supported, write_context: { ...context, participant_epoch: "0" } })).toBeNull();
    expect(parseCollaborationIssue({ ...supported, execution: { ...execution, protocol_version: 4 } })).toBeNull();
  });
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
    { ...execution, protocol_version: 2 }, { ...execution, protocol_version: 3 }, { ...execution, protocol_version: 4 }, { ...execution, protocol_version: 6 }, { ...execution, protocol_version: "5" }, { ...execution, availability: "available" },
    { ...execution, unavailable_reason: "future_reason" }, { ...execution, control: { ...execution.control, scope: "target" } },
  ])("refuses unknown or malformed present protocols", (value) => {
    expect(parse(value)).toBeNull();
  });

  it.each(["channels", "targets", "commands", "groups", "runs"])("never treats missing or malformed %s as empty", (field) => {
    for (const value of [undefined, null, "PRIVATE_COLLECTION", 1]) {
      expect(parse({ ...execution, [field]: value })).toBeNull();
    }
  });

  it("accepts complete empty collections without granting execution", () => {
    const empty = { ...execution, channels: [], targets: [], commands: [], groups: [], runs: [] };
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
      channels: [{ ...agentChannel, group_generation: "9007199254740993" }, squadChannel],
      groups: [{ ...agentGroup, generation: "9007199254740993" }, squadGroup] })?.execution?.groups[0]?.generation).toBe("9007199254740993");
    expect(parse({ ...execution, targets: [{ ...agentTarget, group_generation: "10" }, squadTarget],
      groups: [{ ...agentGroup, generation: "9", state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup] })).toBeNull();
  });

  it.each(["0", "01", "1e3", "1.5", "-1", "+1", " 1", "1\n", "9223372036854775808", "9".repeat(1000), 1, null, undefined])("rejects malformed version %s at every execution boundary", (value) => {
    expect(parse({ ...execution, target_set_version: value })).toBeNull();
    expect(parse({ ...execution, control: { ...execution.control, control_version: value } })).toBeNull();
    expect(parse({ ...execution, execution_gate: { enabled: true, epoch: value } })).toBeNull();
    expect(parse({ ...execution, channels: [{ ...agentChannel, control_version: value }, squadChannel] })).toBeNull();
    expect(parse({ ...execution, channels: [{ ...agentChannel, next_command_seq: value }, squadChannel] })).toBeNull();
    for (const field of ["target_epoch", "config_version", "hold_version"]) {
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
    const empty = { ...execution, channels: [{ ...agentChannel, group_generation: "0", next_command_seq: "1", active_group_id: null }], targets: [{ ...agentTarget, group_generation: "0", active_group_id: null }], commands: [], groups: [], runs: [] };
    expect(parse(empty)?.execution?.targets[0]?.group_generation).toBe("0");
    expect(parse({ ...empty, targets: [{ ...empty.targets[0], group_generation: "-1" }] })).toBeNull();
    expect(parse({ ...execution, targets: [{ ...agentTarget, group_generation: "0" }, squadTarget] })).toBeNull();
  });

  it("rejects duplicate identities, channel FIFO positions and per-comment targets", () => {
    for (const field of ["channels", "targets", "commands", "groups", "runs"] as const) {
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
    const continuation = { ...memberRun, id: unknownId, agent_id: leaderRun.agent_id, step_kind: "continuation", step_key: "continue", parent_run_id: memberRun.id };
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

  it("requires a consuming run and preserves a valid prepared root", () => {
    const terminal = { ...execution,
      channels: [{ ...agentChannel, active_group_id: null }, squadChannel],
      targets: [{ ...agentTarget, active_group_id: null }, squadTarget],
      commands: [{ ...agentCommand, state: "stopped" }, waitingCommand, squadCommand],
      groups: [{ ...agentGroup, state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup],
      runs: [leaderRun, memberRun],
    };
    expect(parse(terminal)).toBeNull();
    const stoppedRun = { ...agentRun, progress_state: "settled", proof_state: "exited_proven", settled_at: "2026-10-17T00:00:00Z" };
    expect(parse({ ...terminal, runs: [stoppedRun, leaderRun, memberRun] })?.execution?.groups[0]?.state).toBe("stopped");
    const preparing = { ...execution, groups: [{ ...agentGroup, state: "active", expansion_sealed_at: null, forced_disposition: null }, squadGroup], runs: [leaderRun, memberRun] };
    expect(parse(preparing)).toBeNull();
    expect(parse({ ...preparing, runs: [{ ...agentRun, progress_state: "prepared", forced_disposition: null }, leaderRun, memberRun] })?.execution?.groups[0]?.state).toBe("active");
  });

  it("compares historical generations numerically without narrowing large values", () => {
    const history = { ...execution,
      channels: [{ ...agentChannel, group_generation: "10", active_group_id: null }, squadChannel],
      targets: [{ ...agentTarget, group_generation: "10", active_group_id: null }, squadTarget],
      commands: [{ ...agentCommand, state: "stopped" }, waitingCommand, squadCommand],
      groups: [{ ...agentGroup, generation: "9", state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup],
      runs: [{ ...agentRun, progress_state: "settled", proof_state: "exited_proven", settled_at: "2026-10-17T00:00:00Z" }, leaderRun, memberRun],
    };
    expect(parse(history)?.execution?.groups[0]?.generation).toBe("9");
    expect(parse({ ...history, groups: [{ ...history.groups[0], generation: "11" }, squadGroup] })).toBeNull();
    const large = { ...history, targets: [{ ...history.targets[0], group_generation: "9007199254740993" }, squadTarget],
      channels: [{ ...agentChannel, group_generation: "9007199254740993", active_group_id: null }, squadChannel],
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

  it("reads current execution without retaining deleted legacy mappings", () => {
    const result = parse(execution);
    expect(result?.execution?.protocol_version).toBe(5);
    expect(result?.execution).not.toHaveProperty("legacy_unresolved");
    expect(parse({ ...execution, protocol_version: 2, legacy_unresolved: { command_ids: [], run_ids: [], active_run_id: null } })).toBeNull();
  });

  it.each(["commands", "groups", "runs"] as const)("requires channel and batch identities on %s", (collection) => {
    for (const field of ["channel_id", "batch_id"]) {
      for (const value of [undefined, null, 1, "PRIVATE_ID", unknownId]) {
        expect(parse({ ...execution, [collection]: [{ ...execution[collection][0], [field]: value }, ...execution[collection].slice(1)] })).toBeNull();
      }
    }
  });

  it("retains two immutable inputs in one execution batch", () => {
    const batched = { ...execution,
      commands: [agentCommand, { ...waitingCommand, state: "in_flight", group_id: agentGroup.id, batch_id: agentCommand.batch_id, author_user_id: unknownId }, squadCommand],
      groups: [{ ...agentGroup, inputs: [...agentGroup.inputs, { ...agentGroup.inputs[0], command_id: waitingCommand.id, sequence: waitingCommand.sequence }] }, squadGroup],
    };
    const result = parse(batched);
    expect(result?.execution?.commands).toHaveLength(3);
    expect(result?.execution?.groups).toHaveLength(2);
    expect(result?.execution?.commands[1]).toEqual(batched.commands[1]);
    expect(parse({ ...batched, commands: [agentCommand, { ...batched.commands[1], channel_id: unknownId }, squadCommand] })).toBeNull();
    expect(parse({ ...batched, commands: [agentCommand, { ...batched.commands[1], batch_id: unknownId }, squadCommand] })).toBeNull();
    for (const state of ["waiting", "succeeded", "failed", "stopped"]) {
      expect(parse({ ...batched, commands: [agentCommand, { ...batched.commands[1], state }, squadCommand] })).toBeNull();
    }
  });

  it("retains cancelled input tombstones while the surviving batch settles", () => {
    const cancelled = { ...waitingCommand, batch_id: agentCommand.batch_id, state: "cancelled" };
    expect(parse({ ...execution, commands: [agentCommand, cancelled, squadCommand] })?.execution?.commands[1]).toEqual(cancelled);
    const terminal = { ...execution,
      channels: [{ ...agentChannel, active_group_id: null }, squadChannel],
      targets: [{ ...agentTarget, active_group_id: null }, squadTarget],
      commands: [{ ...agentCommand, state: "stopped" }, cancelled, squadCommand],
      groups: [{ ...agentGroup, state: "stopped", settled_at: "2026-10-17T00:00:00Z" }, squadGroup],
      runs: [{ ...agentRun, progress_state: "settled", proof_state: "exited_proven", settled_at: "2026-10-17T00:00:00Z" }, leaderRun, memberRun],
    };
    expect(parse(terminal)?.execution?.commands[1]).toEqual(cancelled);
    expect(parse({ ...terminal, commands: [terminal.commands[0], { ...cancelled, state: "in_flight" }, squadCommand] })).toBeNull();
  });

  it("rejects a waiting batch shared across channels before any group starts", () => {
    const waiting = { ...execution,
      channels: execution.channels.map((item) => ({ ...item, group_generation: "0", active_group_id: null })),
      targets: execution.targets.map((item) => ({ ...item, group_generation: "0", active_group_id: null })),
      commands: execution.commands.map((item) => ({ ...item, state: "waiting", group_id: null })), groups: [], runs: [],
    };
    expect(parse(waiting)?.execution?.commands).toHaveLength(3);
    expect(parse({ ...waiting, commands: [waiting.commands[0], { ...waiting.commands[1], batch_id: squadCommand.batch_id }, waiting.commands[2]] })).toBeNull();
  });

  it("accepts a shared active group only for targets in the same channel", () => {
    const alias = { ...agentTarget, id: unknownId, kind: "squad", resource_id: unknownId };
    expect(parse({ ...execution, targets: [...execution.targets, alias] })?.execution?.targets).toHaveLength(3);
    expect(parse({ ...execution, targets: [...execution.targets, { ...alias, channel_id: squadTarget.channel_id }] })).toBeNull();
  });

  it("requires a channel for every target with input or execution", () => {
    expect(parse({ ...execution, targets: [{ ...agentTarget, channel_id: null }, squadTarget] })).toBeNull();
    const unused = { ...execution, channels: [], targets: [{ ...agentTarget, channel_id: null, group_generation: "0", active_group_id: null }], commands: [], groups: [], runs: [] };
    expect(parse(unused)?.execution?.targets[0]?.channel_id).toBeNull();
    expect(parse({ ...unused, targets: [{ ...unused.targets[0], channel_id: undefined }] })).toBeNull();
    expect(parse({ ...unused, targets: [{ ...unused.targets[0], group_generation: "1" }] })).toBeNull();
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

  it("keeps each target hold independent of its shared channel pause", () => {
    const alias = { ...agentTarget, id: unknownId, kind: "squad", resource_id: unknownId, hold_reason: "initial", hold_version: "9223372036854775807" };
    const result = parse({ ...execution, targets: [...execution.targets, alias] });
    expect(result?.execution?.targets[0]?.hold_reason).toBeNull();
    expect(result?.execution?.targets[2]?.hold_reason).toBe("initial");
    expect(result?.execution?.targets[2]?.hold_version).toBe("9223372036854775807");
    expect(result?.execution?.channels).toHaveLength(2);
    expect(result?.capabilities.control_ai).toBe(false);
  });

  it("requires complete hold, channel and gate facts before trusting a v4 snapshot", () => {
    for (const hold_reason of [undefined, "", "paused", "PRIVATE_HOLD", 0, false]) {
      expect(parse({ ...execution, targets: [{ ...agentTarget, hold_reason }, squadTarget] })).toBeNull();
    }
    for (const hold_reason of [null, "initial", "grant_required", "config_invalid", "switch_hold"]) {
      expect(parse({ ...execution, targets: [{ ...agentTarget, hold_reason }, squadTarget] })).not.toBeNull();
    }
    for (const execution_gate of [undefined, null, {}, { enabled: true }, { enabled: "true", epoch: "1" }]) {
      expect(parse({ ...execution, execution_gate })).toBeNull();
    }
    expect(parse({ ...execution, execution_gate: { enabled: false, epoch: "9223372036854775807" } })?.execution?.execution_gate.enabled).toBe(false);
    expect(parse({ ...execution, channels: [squadChannel] })).toBeNull();
    for (const patch of [{ queue_state: "active" }, { control_version: "3" }, { group_generation: "2" }, { active_group_id: null }, { pause_reason: "different" }]) {
      expect(parse({ ...execution, channels: [{ ...agentChannel, ...patch }, squadChannel] })).toBeNull();
    }
  });

  it("retains channels without current targets but never invents missing group proof", () => {
    const retained = { id: unknownId, queue_state: "paused", control_version: "7", group_generation: "4", next_command_seq: "1", active_group_id: null, pause_reason: "stop_requested" };
    expect(parse({ ...execution, channels: [...execution.channels, retained] })?.execution?.channels[2]).toEqual(retained);
    expect(parse({ ...execution, channels: [...execution.channels, { ...retained, active_group_id: unknownId }] })).toBeNull();
    expect(parse({ ...execution, channels: [...execution.channels, { ...retained, active_group_id: agentGroup.id }] })).toBeNull();
  });
});
