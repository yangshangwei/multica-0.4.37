// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationIssue } from "./schema";
import { collaborationExecutionFixture as execution, collaborationExecutionIssueFixture as issue } from "./test-fixtures";

const unknownId = "00000000-0000-4000-8000-000000000099";
const settledAt = "2026-10-18T00:00:00Z";
const [agentTarget, squadTarget] = [execution.targets[0]!, execution.targets[1]!];
const [agentChannel, squadChannel] = [execution.channels[0]!, execution.channels[1]!];
const [agentCommand, waitingCommand, squadCommand] = [execution.commands[0]!, execution.commands[1]!, execution.commands[2]!];
const [agentGroup, squadGroup] = [execution.groups[0]!, execution.groups[1]!];
const [agentRun, leaderRun, memberRun] = [execution.runs[0]!, execution.runs[1]!, execution.runs[2]!];
const parse = (value: unknown) => parseCollaborationIssue({ ...issue, execution: value });

function waitingLane() {
  return {
    ...execution,
    channels: [{ ...agentChannel, group_generation: "0", next_command_seq: "4", active_group_id: null }],
    targets: [agentTarget, {
      ...squadTarget, channel_id: agentChannel.id, control: agentTarget.control, pause_reason: agentTarget.pause_reason,
    }].map((target) => ({ ...target, group_generation: "0", active_group_id: null })),
    commands: [
      { ...agentCommand, state: "waiting", group_id: null },
      { ...squadCommand, channel_id: agentChannel.id, sequence: "2", state: "waiting", group_id: null },
      { ...waitingCommand, sequence: "3" },
    ],
    groups: [], runs: [],
  };
}

function switchedSquad() {
  return {
    ...execution,
    channels: [{ ...agentChannel, next_command_seq: "4" }, { ...squadChannel, group_generation: "10", active_group_id: null }],
    targets: [agentTarget, {
      ...squadTarget, channel_id: agentChannel.id, control: agentTarget.control, group_generation: agentChannel.group_generation,
      active_group_id: agentChannel.active_group_id, pause_reason: agentChannel.pause_reason,
      config_version: "9007199254740994", hold_reason: "switch_hold", hold_version: "3",
    }],
    commands: [agentCommand, waitingCommand, { ...squadCommand, state: "stopped" }, {
      ...waitingCommand, id: unknownId, target_id: squadTarget.id, sequence: "3", input_event_id: unknownId,
    }],
    groups: [agentGroup, { ...squadGroup, generation: "9", state: "stopped", settled_at: settledAt }],
    runs: [agentRun, leaderRun, { ...memberRun, progress_state: "settled", proof_state: "exited_proven", settled_at: settledAt }],
  };
}

function requeuedAgent() {
  return {
    ...execution,
    channels: [{ ...agentChannel, active_group_id: null }, squadChannel],
    targets: [{ ...agentTarget, active_group_id: null }, squadTarget],
    commands: [{ ...agentCommand, state: "waiting", group_id: null, batch_id: waitingCommand.batch_id }, waitingCommand, squadCommand],
    groups: [{ ...agentGroup, state: "stopped", settled_at: settledAt }, squadGroup],
    runs: [{ ...agentRun, progress_state: "settled", proof_state: "never_launched_proven", settled_at: settledAt }, leaderRun, memberRun],
  };
}

function mixedSquadBatch() {
  const directInput = { ...agentCommand, channel_id: squadChannel.id, batch_id: squadGroup.batch_id, group_id: squadGroup.id };
  const squadInput = { ...squadCommand, sequence: "2" };
  return {
    ...execution,
    channels: [{ ...squadChannel, next_command_seq: "3" }],
    targets: [{ ...agentTarget, channel_id: squadChannel.id, control: squadTarget.control, active_group_id: squadGroup.id }, squadTarget],
    commands: [directInput, squadInput],
    groups: [{ ...squadGroup, inputs: [directInput, squadInput].map((input) => ({ command_id: input.id,
      channel_id: input.channel_id, batch_id: input.batch_id, sequence: input.sequence, consuming_run_id: leaderRun.id })) }],
    runs: [{ ...leaderRun, agent_id: agentTarget.resource_id }, memberRun],
  };
}

afterEach(() => setSchemaLogger(noopLogger));

describe("collaboration channel FIFO and Switch history", () => {
  it("requires v4 relationships and never accepts v3 as an idle projection", () => {
    expect(parse(execution)?.execution).toEqual(execution);
    expect(parse({ ...execution, protocol_version: 3 })).toBeNull();
    const result = parse(execution);
    expect(result?.execution?.targets[0]).not.toHaveProperty("next_command_seq");
    expect(result?.capabilities).toMatchObject({ control_ai: false, submit_command: false });
  });

  it("preserves one sequence across direct and held squad inputs in a channel", () => {
    const waiting = waitingLane();
    expect(parse(waiting)?.execution?.commands).toEqual(waiting.commands);
    expect(parse(waiting)?.execution?.targets[1]?.hold_reason).toBe("grant_required");
    expect(parse({ ...waiting, commands: [waiting.commands[0], { ...waiting.commands[1], sequence: "1" }, waiting.commands[2]] })).toBeNull();
  });

  it("keeps the squad representative distinct from the earlier direct-agent input", () => {
    const mixed = mixedSquadBatch();
    expect(parse(mixed)?.execution).toEqual(mixed);
    expect(parse(mixed)?.execution?.groups[0]?.command_id).toBe(squadCommand.id);
    expect(parse(mixed)?.execution?.groups[0]?.inputs[0]?.command_id).toBe(agentCommand.id);
  });

  it.each(["agent", "squad"])("rejects an unrelated %s as a consumed input in a squad batch", (kind) => {
    const mixed = mixedSquadBatch();
    const extra = { ...squadTarget, id: unknownId, kind, resource_id: unknownId };
    const command = { ...squadCommand, id: "00000000-0000-4000-8000-000000000085", target_id: extra.id, sequence: "3" };
    const forged = { ...mixed, channels: [{ ...mixed.channels[0], next_command_seq: "4" }], targets: [...mixed.targets, extra], commands: [...mixed.commands, command],
      groups: [{ ...mixed.groups[0], inputs: [...mixed.groups[0]!.inputs, { ...mixed.groups[0]!.inputs[0], command_id: command.id, sequence: "3" }] }] };
    expect(parse(forged)).toBeNull();
  });

  it("refuses squad input under an agent group and member proof as initial consumption", () => {
    const mixed = mixedSquadBatch();
    expect(parse({ ...mixed, groups: [{ ...mixed.groups[0], target_id: agentTarget.id, command_id: agentCommand.id }],
      runs: [{ ...mixed.runs[0], step_kind: "agent", target_id: agentTarget.id, command_id: agentCommand.id }],
    })).toBeNull();
    expect(parse({ ...mixed, groups: [{ ...mixed.groups[0], inputs: mixed.groups[0]!.inputs.map((input) => ({ ...input, consuming_run_id: memberRun.id })) }] })).toBeNull();
  });

  it("bounds every current input by its channel counter without narrowing integers", () => {
    const waiting = waitingLane();
    const large = { ...waiting, channels: [{ ...waiting.channels[0], next_command_seq: "9223372036854775807" }],
      commands: waiting.commands.map((input, index) => ({ ...input, sequence: (9007199254740993n + BigInt(index)).toString() })) };
    expect(parse(large)?.execution?.commands[0]?.sequence).toBe("9007199254740993");
    for (const sequence of ["9223372036854775807", "9223372036854775808"]) {
      expect(parse({ ...large, commands: [{ ...large.commands[0], sequence }, ...large.commands.slice(1)] })).toBeNull();
    }
    for (const next_command_seq of [undefined, null, "0", "01", "1e3", "1.5", 4, "9223372036854775808"]) {
      expect(parse({ ...waiting, channels: [{ ...waiting.channels[0], next_command_seq }] })).toBeNull();
    }
    const ordered = { ...waiting, channels: [{ ...waiting.channels[0], next_command_seq: "10" }],
      commands: waiting.commands.map((input, index) => ({ ...input, sequence: String(index + 7) })) };
    expect(parse(ordered)?.execution?.commands[2]?.sequence).toBe("9");
    expect(parse({ ...ordered, channels: [{ ...ordered.channels[0], next_command_seq: "9" }] })).toBeNull();
  });

  it("keeps settled squad history on its original channel after moving into a busy channel", () => {
    const switched = switchedSquad();
    const result = parse(switched);
    expect(result?.execution).toEqual(switched);
    expect(result?.execution?.targets[1]?.active_group_id).toBe(agentGroup.id);
    expect(result?.execution?.groups[1]?.channel_id).toBe(squadChannel.id);
    expect(result?.execution?.groups[1]?.generation).toBe("9");
    expect(result?.execution?.commands[3]?.channel_id).toBe(agentChannel.id);
    expect(result?.execution?.targets[1]?.hold_reason).toBe("switch_hold");
  });

  it("still validates original channel generations and sealed proof after Switch", () => {
    const switched = switchedSquad();
    expect(parse({ ...switched, channels: [switched.channels[0]] })).toBeNull();
    expect(parse({ ...switched, groups: [agentGroup, { ...switched.groups[1], generation: "11" }] })).toBeNull();
    expect(parse({ ...switched, groups: [agentGroup, { ...switched.groups[1], settled_at: null }] })).toBeNull();
    expect(parse({ ...switched, runs: [agentRun, leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...switched, commands: [agentCommand, waitingCommand, { ...switched.commands[2], channel_id: agentChannel.id }, switched.commands[3]] })).toBeNull();
    expect(parse({ ...switched, commands: [agentCommand, waitingCommand, switched.commands[2], { ...switched.commands[3], channel_id: squadChannel.id }] })).toBeNull();
  });

  it("allows unstarted inputs to change batches while preserving identity and sequence", () => {
    const waiting = waitingLane();
    const regrouped = { ...waiting, commands: waiting.commands.map((input) => ({ ...input, batch_id: unknownId })) };
    expect(parse(regrouped)?.execution?.commands).toEqual(regrouped.commands);
    expect(parse(regrouped)?.execution?.commands.map(({ id, target_id, sequence }) => ({ id, target_id, sequence })))
      .toEqual(waiting.commands.map(({ id, target_id, sequence }) => ({ id, target_id, sequence })));
  });

  it("requires immutable input membership and a consuming run for every initial group", () => {
    for (const inputs of [undefined, null, [], "PRIVATE_INPUTS", [{ ...agentGroup.inputs[0], consuming_run_id: null }]]) {
      expect(parse({ ...execution, groups: [{ ...agentGroup, inputs }, squadGroup] })).toBeNull();
    }
    for (const patch of [{ command_id: unknownId }, { channel_id: squadChannel.id }, { batch_id: unknownId },
      { sequence: "2" }, { sequence: "1e3" }, { consuming_run_id: unknownId }, { consuming_run_id: leaderRun.id }]) {
      expect(parse({ ...execution, groups: [{ ...agentGroup, inputs: [{ ...agentGroup.inputs[0], ...patch }] }, squadGroup] })).toBeNull();
    }
    expect(parse({ ...execution, groups: [{ ...agentGroup, inputs: [...agentGroup.inputs, agentGroup.inputs[0]] }, squadGroup] })).toBeNull();
    expect(parse({ ...execution, runs: [leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...execution, commands: [{ ...agentCommand, group_id: null }, waitingCommand, squadCommand] })).toBeNull();
  });

  it("returns a proven never-launched command to waiting without rewriting the old group", () => {
    const requeued = requeuedAgent();
    const result = parse(requeued);
    expect(result?.execution).toEqual(requeued);
    expect(result?.execution?.commands[0]).toMatchObject({ state: "waiting", group_id: null, sequence: "1", batch_id: waitingCommand.batch_id });
    expect(result?.execution?.groups[0]?.inputs[0]).toEqual(agentGroup.inputs[0]);
    expect(result?.execution?.runs[0]?.batch_id).toBe(agentCommand.batch_id);
  });

  it("retains the original consumption when a returned command joins a later group", () => {
    const requeued = requeuedAgent();
    const batchId = "00000000-0000-4000-8000-000000000081";
    const runId = "00000000-0000-4000-8000-000000000082";
    const nextGroup = { ...agentGroup, id: unknownId, generation: "2", batch_id: batchId, state: "active", expansion_sealed_at: null, forced_disposition: null,
      inputs: [{ ...agentGroup.inputs[0], batch_id: batchId, consuming_run_id: runId }] };
    const retried = { ...requeued,
      channels: [{ ...agentChannel, active_group_id: nextGroup.id, group_generation: "2" }, squadChannel],
      targets: [{ ...agentTarget, active_group_id: nextGroup.id, group_generation: "2" }, squadTarget],
      commands: [{ ...agentCommand, group_id: nextGroup.id, batch_id: batchId }, waitingCommand, squadCommand],
      groups: [...requeued.groups, nextGroup],
      runs: [...requeued.runs, { ...agentRun, id: runId, group_id: nextGroup.id, batch_id: batchId, progress_state: "prepared", forced_disposition: null }],
    };
    expect(parse(retried)?.execution).toEqual(retried);
    expect(parse({ ...retried, runs: [{ ...retried.runs[0], proof_state: "exited_proven" }, ...retried.runs.slice(1)] })).toBeNull();
    expect(parse({ ...retried, groups: [{ ...retried.groups[0], generation: "3" }, ...retried.groups.slice(1)],
      channels: [{ ...retried.channels[0], group_generation: "3" }, squadChannel],
      targets: [{ ...retried.targets[0], group_generation: "3" }, squadTarget],
    })).toBeNull();
  });

  it("keeps a never-launched squad input's old membership after waiting migrates", () => {
    const switched = switchedSquad();
    const migrated = { ...switched,
      commands: [agentCommand, waitingCommand, { ...squadCommand, state: "waiting", group_id: null,
        channel_id: agentChannel.id, batch_id: waitingCommand.batch_id, sequence: "3" }],
      runs: [agentRun, { ...leaderRun, proof_state: "never_launched_proven", business_outcome: null, settled_at: settledAt }],
    };
    expect(parse(migrated)?.execution).toEqual(migrated);
    expect(parse({ ...migrated, runs: [agentRun, { ...migrated.runs[1], proof_state: "exited_proven" }] })).toBeNull();
    expect(parse({ ...migrated, commands: [agentCommand, waitingCommand, { ...migrated.commands[2], channel_id: squadChannel.id }] })).toBeNull();
  });

  it("does not reinterpret stopped, unknown or successful execution as safe requeue", () => {
    const requeued = requeuedAgent();
    for (const proof_state of ["unknown", "exited_proven", "never_started_proven"]) {
      expect(parse({ ...requeued, runs: [{ ...requeued.runs[0], proof_state }, leaderRun, memberRun] })).toBeNull();
    }
    expect(parse({ ...requeued, runs: [{ ...requeued.runs[0], business_outcome: "success" }, leaderRun, memberRun] })).toBeNull();
    expect(parse({ ...requeued, groups: [agentGroup, squadGroup] })).toBeNull();
    expect(parse({ ...requeued, groups: [{ ...requeued.groups[0], expansion_sealed_at: null }, squadGroup] })).toBeNull();
    expect(parse({ ...requeued, commands: [{ ...requeued.commands[0], group_id: agentGroup.id }, waitingCommand, squadCommand] })).toBeNull();
  });

  it("retains consumed cancellation after a completely unlaunched group stops", () => {
    const requeued = requeuedAgent();
    const cancelled = { ...requeued, commands: [{ ...agentCommand, state: "cancelled" }, waitingCommand, squadCommand] };
    expect(parse(cancelled)?.execution).toEqual(cancelled);
    expect(parse({ ...cancelled, runs: [{ ...cancelled.runs[0], proof_state: "exited_proven" }, leaderRun, memberRun] })).toBeNull();
  });

  it("requires complete nonlaunch proof before returning any initial group input", () => {
    const switched = switchedSquad();
    const partial = { ...switched,
      commands: [agentCommand, waitingCommand, { ...squadCommand, state: "waiting", group_id: null,
        channel_id: agentChannel.id, batch_id: waitingCommand.batch_id, sequence: "3" }],
      runs: [agentRun, { ...leaderRun, proof_state: "never_launched_proven", business_outcome: null, settled_at: settledAt }, switched.runs[2]],
    };
    expect(parse(partial)).toBeNull();
  });

  it("keeps unconsumed cancellation tombstones out of a group's consumed membership", () => {
    const cancelled = { ...waitingCommand, batch_id: agentCommand.batch_id, state: "cancelled" };
    expect(parse({ ...execution, commands: [agentCommand, cancelled, squadCommand] })?.execution?.commands[1]).toEqual(cancelled);
    const forged = { ...agentGroup.inputs[0], command_id: cancelled.id, sequence: cancelled.sequence };
    expect(parse({ ...execution, commands: [agentCommand, cancelled, squadCommand],
      groups: [{ ...agentGroup, inputs: [...agentGroup.inputs, forged] }, squadGroup] })).toBeNull();
  });

  it("retains cancellation history on the old channel without pretending it was consumed", () => {
    const switched = switchedSquad();
    const cancelled = { ...waitingCommand, id: "00000000-0000-4000-8000-000000000083", target_id: squadTarget.id,
      channel_id: squadChannel.id, batch_id: squadGroup.batch_id, sequence: "2", input_event_id: "00000000-0000-4000-8000-000000000084", state: "cancelled" };
    const history = { ...switched, channels: [switched.channels[0], { ...switched.channels[1], next_command_seq: "3" }], commands: [...switched.commands, cancelled] };
    expect(parse(history)?.execution?.commands[4]).toEqual(cancelled);
    expect(parse({ ...history, commands: [...switched.commands, { ...cancelled, state: "waiting" }] })).toBeNull();
    expect(parse({ ...history, commands: [...switched.commands, { ...cancelled, group_id: squadGroup.id }] })).toBeNull();
  });

  it("rejects sequence reuse by a different command even after the original input moved", () => {
    const switched = switchedSquad();
    const reused = { ...waitingCommand, id: "00000000-0000-4000-8000-000000000083", target_id: squadTarget.id,
      channel_id: squadChannel.id, batch_id: squadGroup.batch_id, sequence: "1", input_event_id: "00000000-0000-4000-8000-000000000084", state: "cancelled" };
    const migrated = { ...switched,
      commands: [agentCommand, waitingCommand, { ...squadCommand, state: "waiting", group_id: null,
        channel_id: agentChannel.id, batch_id: waitingCommand.batch_id, sequence: "3" }, reused],
      runs: [agentRun, { ...leaderRun, proof_state: "never_launched_proven", business_outcome: null, settled_at: settledAt }],
    };
    expect(parse(migrated)).toBeNull();
  });

  it("keeps waiting inputs detached until explicit M11 read membership is supported", () => {
    const waiting = { ...waitingCommand, batch_id: agentCommand.batch_id, group_id: agentGroup.id };
    expect(parse({ ...execution, commands: [agentCommand, waiting, squadCommand] })).toBeNull();
    const merged = { ...agentGroup.inputs[0], command_id: waiting.id, sequence: waiting.sequence, consuming_run_id: null };
    expect(parse({ ...execution, commands: [agentCommand, waiting, squadCommand],
      groups: [{ ...agentGroup, inputs: [...agentGroup.inputs, merged] }, squadGroup] })).toBeNull();
  });

  it("redacts malformed historical membership and strips private execution data", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    const privateInput = { ...agentGroup.inputs[0], body: "PRIVATE_BODY", grant: "PRIVATE_GRANT", launch_nonce: "PRIVATE_NONCE" };
    const projected = parse({ ...execution, groups: [{ ...agentGroup, inputs: [privateInput] }, squadGroup] });
    expect(projected?.execution?.groups[0]?.inputs[0]).toEqual(agentGroup.inputs[0]);
    expect(JSON.stringify(projected)).not.toContain("PRIVATE_");
    expect(parse({ ...execution, groups: [{ ...agentGroup, inputs: [{ ...privateInput, consuming_run_id: "PRIVATE_INVALID" }] }, squadGroup] })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
  });
});
