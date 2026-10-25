// @vitest-environment node
import { expect, it } from "vitest";
import { CollaborationExecutionSchema } from "./execution-schema";
import { collaborationExecutionFixture } from "./test-fixtures";

const followupId = "00000000-0000-4000-8000-000000000090";
const continuationId = "00000000-0000-4000-8000-000000000091";
const eventId = "00000000-0000-4000-8000-000000000092";
const returnedBatch = "00000000-0000-4000-8000-000000000093";
const ended = "2026-10-18T00:00:00Z";

function joinedSquad() {
  const value = CollaborationExecutionSchema.parse(collaborationExecutionFixture);
  if (value.protocol_version !== 5) throw new Error("Expected the legacy squad fixture");
  value.control.queue_state = "active";
  const group = value.groups[1]!, target = value.targets[1]!, channel = value.channels[1]!;
  group.state = "active"; group.expansion_sealed_at = null; group.forced_disposition = null;
  target.control.queue_state = "active"; target.hold_reason = null; target.pause_reason = "";
  channel.queue_state = "active"; channel.pause_reason = ""; channel.next_command_seq = "3";
  value.runs[2]!.progress_state = "running"; value.runs[2]!.forced_disposition = null;
  const followup = { ...value.commands[2]!, id: followupId, sequence: "2", input_event_id: eventId, state: "waiting" as const };
  value.commands.push(followup);
  value.runs.push({ ...value.runs[1]!, id: continuationId, parent_run_id: value.runs[2]!.id, step_key: "continuation-1", step_kind: "continuation",
    progress_state: "prepared", settled_at: null, business_outcome: null, proof_state: "unknown" });
  group.queued_continuation_run_id = continuationId;
  group.unread_inputs.push({ command_id: followupId, channel_id: group.channel_id, batch_id: group.batch_id, sequence: "2" });
  return value;
}

function consumedSquad() {
  const value = joinedSquad();
  const group = value.groups[1]!;
  value.commands[3]!.state = "in_flight";
  value.runs[3]!.progress_state = "running";
  group.queued_continuation_run_id = null;
  group.inputs.push({ ...group.unread_inputs[0]!, consuming_run_id: continuationId, reader_run_ids: [continuationId] });
  group.unread_inputs = [];
  return value;
}

it("represents an unread join separately from the continuation's consumed prompt", () => {
  const joined = joinedSquad();
  expect(CollaborationExecutionSchema.safeParse(joined).success).toBe(true);
  const consumed = consumedSquad();
  expect(CollaborationExecutionSchema.safeParse(consumed).success).toBe(true);
  expect(consumed.groups[1]!.inputs[1]!.consuming_run_id).toBe(continuationId);
});

it("requires explicit complete unread membership and the actual prepared continuation", () => {
  for (const change of [
    { unread_inputs: undefined }, { unread_inputs: null }, { unread_inputs: [] },
    { queued_continuation_run_id: null }, { queued_continuation_run_id: followupId },
    { expansion_sealed_at: ended }, { forced_disposition: "stopped" },
  ]) {
    const value = joinedSquad();
    expect(CollaborationExecutionSchema.safeParse({ ...value, groups: [value.groups[0], { ...value.groups[1], ...change }] }).success).toBe(false);
  }
  const duplicate = joinedSquad();
  duplicate.groups[1]!.unread_inputs.push(duplicate.groups[1]!.unread_inputs[0]!);
  expect(CollaborationExecutionSchema.safeParse(duplicate).success).toBe(false);
  const extra = joinedSquad();
  extra.runs.push({ ...extra.runs[3]!, id: eventId, step_key: "continuation-2" });
  expect(CollaborationExecutionSchema.safeParse(extra).success).toBe(false);
});

it("never accepts a member, wrong leader or another group as the input consumer", () => {
  for (const consumer of [collaborationExecutionFixture.runs[0]!.id, collaborationExecutionFixture.runs[2]!.id]) {
    const value = consumedSquad();
    value.groups[1]!.inputs[1]!.consuming_run_id = consumer;
    expect(CollaborationExecutionSchema.safeParse(value).success).toBe(false);
  }
  const wrongLeader = consumedSquad();
  wrongLeader.runs[3]!.agent_id = wrongLeader.runs[2]!.agent_id;
  expect(CollaborationExecutionSchema.safeParse(wrongLeader).success).toBe(false);
});

it("keeps a returned continuation input's own nonlaunch proof without requiring prior members to be unlaunched", () => {
  const value = consumedSquad();
  const group = value.groups[1]!;
  group.state = "failed"; group.expansion_sealed_at = ended; group.settled_at = ended;
  value.commands[2]!.state = "failed";
  value.commands[3]!.state = "waiting"; value.commands[3]!.group_id = null; value.commands[3]!.batch_id = returnedBatch;
  value.channels[1]!.active_group_id = null; value.targets[1]!.active_group_id = null;
  value.runs[2]!.progress_state = "settled"; value.runs[2]!.proof_state = "exited_proven"; value.runs[2]!.settled_at = ended;
  value.runs[3]!.progress_state = "settled"; value.runs[3]!.proof_state = "never_launched_proven"; value.runs[3]!.settled_at = ended;
  expect(CollaborationExecutionSchema.safeParse(value).success).toBe(true);
  value.runs[3]!.proof_state = "exited_proven";
  expect(CollaborationExecutionSchema.safeParse(value).success).toBe(false);
});

function returnedWhileSquadActive() {
  const value = consumedSquad();
  value.commands[3]!.state = "waiting";
  value.commands[3]!.group_id = null;
  value.commands[3]!.batch_id = returnedBatch;
  value.runs[3]!.progress_state = "settled";
  value.runs[3]!.proof_state = "never_launched_proven";
  value.runs[3]!.settled_at = ended;
  return value;
}

it("returns an unread continuation input while the original squad is still active", () => {
  const value = returnedWhileSquadActive();
  expect(CollaborationExecutionSchema.safeParse(value).success).toBe(true);
  expect(value.runs[2]!.progress_state).toBe("running");
  expect(value.groups[1]!.settled_at).toBeNull();
});

it("requires every immutable reader to be a unique same-group run including the consumer", () => {
  for (const readers of [undefined, null, [], [continuationId, continuationId], [eventId],
    [continuationId, collaborationExecutionFixture.runs[0]!.id]]) {
    const value = returnedWhileSquadActive();
    const group = value.groups[1]!;
    expect(CollaborationExecutionSchema.safeParse({ ...value, groups: [value.groups[0], {
      ...group, inputs: [group.inputs[0], { ...group.inputs[1], reader_run_ids: readers }],
    }] }).success).toBe(false);
  }
});

it("rejects returned input if a later prompt reader may have launched", () => {
  for (const proof of ["unknown", "exited_proven", "never_started_proven"] as const) {
    const value = returnedWhileSquadActive();
    value.groups[1]!.inputs[1]!.reader_run_ids.push(value.runs[2]!.id);
    value.runs[2]!.proof_state = proof;
    if (proof !== "unknown") {
      value.runs[2]!.progress_state = "settled";
      value.runs[2]!.settled_at = ended;
    }
    expect(CollaborationExecutionSchema.safeParse(value).success).toBe(false);
  }
  const allNeverLaunched = returnedWhileSquadActive();
  allNeverLaunched.groups[1]!.inputs[1]!.reader_run_ids.push(allNeverLaunched.runs[2]!.id);
  allNeverLaunched.runs[2]!.proof_state = "never_launched_proven";
  allNeverLaunched.runs[2]!.progress_state = "settled";
  allNeverLaunched.runs[2]!.settled_at = ended;
  expect(CollaborationExecutionSchema.safeParse(allNeverLaunched).success).toBe(true);
});
