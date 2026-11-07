import { z } from "zod";
import { nonnegativeVersion, positiveVersion } from "./version";

const uuid = z.string().uuid().transform((value) => value.toLowerCase());
const timestamp = z.string().datetime({ offset: true }).nullable();
const disposition = z.enum(["stopped", "cancelled"]).nullable();
export const COLLABORATION_EXECUTION_VIEW_VERSION = 6;
export const CollaborationExecutionViewVersionSchema = z.literal(COLLABORATION_EXECUTION_VIEW_VERSION);
/** Capture comprehension with a new intent; never add it while replaying one. */
export function collaborationExecutionViewInput(execution: { protocol_version: number } | null | undefined): { execution_view_version?: 6 } {
  return execution?.protocol_version === COLLABORATION_EXECUTION_VIEW_VERSION ? { execution_view_version: COLLABORATION_EXECUTION_VIEW_VERSION } : {};
}
export const CollaborationTargetHoldReasonSchema = z.enum(["initial", "grant_required", "config_invalid", "switch_hold"]);
export const CollaborationExecutionModeSchema = z.enum(["linux_pid_namespace_v1", "host_process_supervision_v1"]);
export const CollaborationDescendantCleanupSchema = z.object({
  state: z.enum(["tracked_scope_complete", "best_effort", "unverified"]),
  scope: z.enum(["process_group", "windows_job", "direct_process", "remote_session", "mixed"]),
  attempted: z.boolean(),
}).refine((value) => value.state !== "best_effort" || value.attempted);

const channel = z.object({
  id: uuid,
  queue_state: z.enum(["paused", "active"]),
  control_version: positiveVersion,
  group_generation: nonnegativeVersion,
  next_command_seq: positiveVersion,
  active_group_id: uuid.nullable(),
  pause_reason: z.string(),
});

const target = z.object({
  id: uuid,
  channel_id: uuid.nullable(),
  kind: z.enum(["agent", "squad"]),
  resource_id: uuid,
  target_epoch: positiveVersion,
  config_version: positiveVersion,
  hold_reason: CollaborationTargetHoldReasonSchema.nullable(),
  hold_version: positiveVersion,
  control: z.object({ scope: z.literal("target"), queue_state: z.enum(["paused", "active"]), control_version: positiveVersion }),
  group_generation: nonnegativeVersion,
  active_group_id: uuid.nullable(),
  pause_reason: z.string(),
  revoked_at: timestamp,
});

const command = z.object({
  id: uuid,
  target_id: uuid,
  channel_id: uuid,
  batch_id: uuid,
  group_id: uuid.nullable(),
  input_event_id: uuid,
  sequence: positiveVersion,
  author_user_id: uuid,
  state: z.enum(["waiting", "in_flight", "succeeded", "failed", "stopped", "cancelled"]),
});

const groupInput = z.object({
  command_id: uuid,
  channel_id: uuid,
  batch_id: uuid,
  sequence: positiveVersion,
  // A root or leader continuation consumed this input; unread joins are separate.
  consuming_run_id: uuid,
  reader_run_ids: z.array(uuid).min(1),
});

const unreadInput = groupInput.omit({ consuming_run_id: true, reader_run_ids: true });

const group = z.object({
  id: uuid,
  target_id: uuid,
  command_id: uuid,
  channel_id: uuid,
  batch_id: uuid,
  generation: positiveVersion,
  target_epoch: positiveVersion,
  config_version: positiveVersion,
  state: z.enum(["active", "waiting_children", "ending", "succeeded", "failed", "stopped", "cancelled"]),
  expansion_sealed_at: timestamp,
  forced_disposition: disposition,
  settled_at: timestamp,
  inputs: z.array(groupInput).min(1),
  unread_inputs: z.array(unreadInput),
  queued_continuation_run_id: uuid.nullable(),
});

const runBase = z.object({
  id: uuid,
  command_id: uuid,
  target_id: uuid,
  group_id: uuid,
  channel_id: uuid,
  batch_id: uuid,
  parent_run_id: uuid.nullable(),
  step_key: z.string().min(1).max(128),
  step_kind: z.enum(["agent", "leader", "member", "continuation"]),
  agent_id: uuid,
  target_epoch: positiveVersion,
  config_version: positiveVersion,
  progress_state: z.enum(["prepared", "dispatched", "preparing", "launch_authorized", "running", "ending", "settled"]),
  proof_state: z.enum(["unknown", "never_launched_proven", "never_started_proven", "exited_proven"]),
  business_outcome: z.enum(["success", "failure"]).nullable(),
  forced_disposition: disposition,
  recording_incomplete: z.boolean(),
  settled_at: timestamp,
  // Additive shared-context facts, absent for runs before the feature. They are
  // display-only: an unknown support value or malformed fact is dropped.
  memory: z.object({
    support: z.enum(["supported", "unsupported"]),
    base_version: nonnegativeVersion.nullable(),
    written_version: positiveVersion.nullable(),
  }).optional().catch(undefined),
});

// Explicitly exclude v6 markers from the legacy branch. Otherwise a malformed
// host observation could pass a union by having its mode fields stripped.
const legacyRun = runBase.extend({
  execution_mode: z.never().optional(),
  descendant_cleanup: z.never().optional(),
  launch_count: z.never().optional(),
});
const modeRun = runBase.extend({
  execution_mode: CollaborationExecutionModeSchema,
  proof_state: z.enum(["unknown", "never_launched_proven", "never_started_proven", "exited_proven", "host_exit_observed"]),
  descendant_cleanup: CollaborationDescendantCleanupSchema.nullable(),
  launch_count: nonnegativeVersion,
}).refine((value) => {
  if (value.execution_mode === "linux_pid_namespace_v1") return value.proof_state !== "host_exit_observed" && value.descendant_cleanup === null;
  if (value.proof_state === "exited_proven") return false;
  return value.proof_state !== "host_exit_observed" || value.launch_count !== "0" && value.descendant_cleanup !== null;
}, "Termination evidence must match the fixed execution mode");
const run = z.union([modeRun, legacyRun]);

const executionV5 = z.object({
  protocol_version: z.literal(5),
  issue_id: uuid,
  session_id: uuid,
  // Client execution capabilities remain unavailable until full acceptance.
  availability: z.literal("unavailable"),
  unavailable_reason: z.literal("shared_runtime_unavailable"),
  target_set_version: positiveVersion,
  execution_gate: z.object({ enabled: z.boolean(), epoch: positiveVersion }),
  control: z.object({ scope: z.literal("issue"), queue_state: z.enum(["paused", "active", "closed"]), control_version: positiveVersion }),
  channels: z.array(channel),
  targets: z.array(target),
  commands: z.array(command),
  groups: z.array(group),
  runs: z.array(legacyRun),
});
const executionV6 = executionV5.extend({ protocol_version: CollaborationExecutionViewVersionSchema, runs: z.array(modeRun) });
const execution = z.discriminatedUnion("protocol_version", [executionV5, executionV6]);

function unique(values: string[]): boolean {
  return new Set(values).size === values.length;
}

function atMostVersion(left: string, right: string): boolean {
  // Zod can run aggregate refinements after a child string refinement failed.
  // Invalid strings must become unavailable, never throw from BigInt parsing.
  return nonnegativeVersion.safeParse(left).success && nonnegativeVersion.safeParse(right).success && BigInt(left) <= BigInt(right);
}

function consistentExecution(value: z.infer<typeof execution>): boolean {
  const { channels, targets, commands, groups, runs } = value;
  const channelsById = new Map(channels.map((item) => [item.id, item]));
  const targetsById = new Map(targets.map((item) => [item.id, item]));
  const commandsById = new Map(commands.map((item) => [item.id, item]));
  const groupsById = new Map(groups.map((item) => [item.id, item]));
  const groupsByBatch = new Map(groups.map((item) => [item.batch_id, item]));
  const inputsByGroup = new Map(groups.map((item) => [item.id, new Map(item.inputs.map((input) => [input.command_id, input]))]));
  const unreadByGroup = new Map(groups.map((item) => [item.id, new Map(item.unread_inputs.map((input) => [input.command_id, input]))]));
  const runsById = new Map(runs.map((item) => [item.id, item]));
  const rootRuns = new Map(runs.filter((item) => item.step_kind === "agent" || item.step_kind === "leader").map((item) => [item.group_id, item]));
  const queuedRuns = runs.filter((item) => item.step_kind === "continuation" && item.progress_state === "prepared" && item.forced_disposition === null && item.settled_at === null);
  if (!unique(queuedRuns.map((item) => item.group_id))) return false;
  const queuedByGroup = new Map(queuedRuns.map((item) => [item.group_id, item.id]));
  const continuationNeverLaunched = (consumer: z.infer<typeof run> | undefined) => consumer?.step_kind === "continuation" &&
    consumer.settled_at !== null && consumer.progress_state === "settled" && consumer.proof_state === "never_launched_proven" && consumer.business_outcome !== "success";
  const continuationInputNeverLaunched = (input: z.infer<typeof groupInput>) => continuationNeverLaunched(runsById.get(input.consuming_run_id)) &&
    input.reader_run_ids.every((id) => {
      const reader = runsById.get(id);
      return reader !== undefined && reader.settled_at !== null && reader.progress_state === "settled" &&
        reader.proof_state === "never_launched_proven" && reader.business_outcome !== "success";
    });
  const allRunsNeverLaunched = new Map<string, boolean>();
  for (const item of runs) {
    const neverLaunched = item.settled_at !== null && item.proof_state === "never_launched_proven" && item.business_outcome !== "success";
    allRunsNeverLaunched.set(item.group_id, allRunsNeverLaunched.get(item.group_id) !== false && neverLaunched);
  }
  if (channelsById.size !== channels.length || targetsById.size !== targets.length || commandsById.size !== commands.length || groupsById.size !== groups.length || runsById.size !== runs.length ||
    groupsByBatch.size !== groups.length || !unique(targets.map((item) => `${item.kind}:${item.resource_id}`)) ||
    !unique(commands.map((item) => `${item.channel_id}:${item.sequence}`)) || !unique(commands.map((item) => `${item.input_event_id}:${item.target_id}`)) ||
    !unique(groups.map((item) => `${item.channel_id}:${item.generation}`)) || !unique(runs.map((item) => `${item.group_id}:${item.step_key}`)) ||
    !unique(groups.filter((item) => item.settled_at === null).map((item) => item.channel_id))) return false;

  const batchChannels = new Map(groups.map((item) => [item.batch_id, item.channel_id]));
  const sequenceOwners = new Map(commands.map((item) => [`${item.channel_id}:${item.sequence}`, item.id]));
  for (const item of commands) {
    const owner = targetsById.get(item.target_id);
    const channel = channelsById.get(item.channel_id);
    if (!owner || owner.channel_id === null || !channel || !atMostVersion(item.sequence, channel.next_command_seq) || item.sequence === channel.next_command_seq) return false;
    if ((item.state === "waiting" || item.state === "in_flight") && owner.channel_id !== item.channel_id) return false;
    const batchChannel = batchChannels.get(item.batch_id);
    if (batchChannel !== undefined && batchChannel !== item.channel_id) return false;
    batchChannels.set(item.batch_id, item.channel_id);
    if (item.group_id === null) {
      if (item.state !== "waiting" && item.state !== "cancelled") return false;
      // Only cancelled, never-consumed tombstones may share an executed batch.
      if (item.state === "waiting" && groupsByBatch.has(item.batch_id)) return false;
      continue;
    }
    const currentGroup = groupsById.get(item.group_id);
    const input = inputsByGroup.get(item.group_id)?.get(item.id);
    if (item.state === "waiting") {
      const unread = unreadByGroup.get(item.group_id)?.get(item.id);
      if (!currentGroup || !unread || input || currentGroup.channel_id !== item.channel_id || currentGroup.batch_id !== item.batch_id || unread.sequence !== item.sequence) return false;
      continue;
    }
    if (!currentGroup || !input || currentGroup.channel_id !== item.channel_id || currentGroup.batch_id !== item.batch_id ||
      input.sequence !== item.sequence) return false;
    const cancelledBeforeLaunch = item.state === "cancelled" &&
      ((currentGroup.settled_at !== null && currentGroup.state !== "succeeded" && allRunsNeverLaunched.get(currentGroup.id) === true) || continuationInputNeverLaunched(input));
    if (item.state !== (currentGroup.settled_at === null ? "in_flight" : currentGroup.state) && !cancelledBeforeLaunch) return false;
  }
  for (const item of groups) {
    const owner = targetsById.get(item.target_id);
    const input = commandsById.get(item.command_id);
    const channel = channelsById.get(item.channel_id);
    if (!owner || !input || !channel || input.target_id !== owner.id || !inputsByGroup.get(item.id)?.has(item.command_id) ||
      !atMostVersion(item.generation, channel.group_generation) ||
      !atMostVersion(item.target_epoch, owner.target_epoch) || !atMostVersion(item.config_version, owner.config_version)) return false;
    const terminal = item.state !== "active" && item.state !== "waiting_children" && item.state !== "ending";
    if (item.queued_continuation_run_id !== (queuedByGroup.get(item.id) ?? null)) return false;
    if (terminal !== (item.settled_at !== null)) return false;
    if (terminal) {
      if (item.expansion_sealed_at === null || (item.forced_disposition !== null && item.forced_disposition !== item.state)) return false;
    } else if (owner.channel_id !== item.channel_id || channel.active_group_id !== item.id || channel.group_generation !== item.generation) return false;

    if (!unique([...item.inputs, ...item.unread_inputs].map((member) => member.command_id)) ||
      !unique([...item.inputs, ...item.unread_inputs].map((member) => member.sequence))) return false;
    if (item.unread_inputs.length > 0 && (owner.kind !== "squad" || terminal || item.expansion_sealed_at !== null ||
      item.forced_disposition !== null || item.queued_continuation_run_id === null)) return false;
    for (const unread of item.unread_inputs) {
      const command = commandsById.get(unread.command_id);
      const commandTarget = command && targetsById.get(command.target_id);
      const leader = rootRuns.get(item.id);
      if (!command || !commandTarget || !leader || command.state !== "waiting" || command.group_id !== item.id ||
        command.channel_id !== unread.channel_id || command.batch_id !== unread.batch_id || command.sequence !== unread.sequence ||
        unread.channel_id !== item.channel_id || unread.batch_id !== item.batch_id ||
        (command.target_id !== owner.id && (commandTarget.kind !== "agent" || commandTarget.resource_id !== leader.agent_id))) return false;
    }
    for (const member of item.inputs) {
      const command = commandsById.get(member.command_id);
      const commandTarget = command && targetsById.get(command.target_id);
      const consumingRun = runsById.get(member.consuming_run_id);
      if (!command || !commandTarget || member.channel_id !== item.channel_id || member.batch_id !== item.batch_id ||
        !atMostVersion(member.sequence, channel.next_command_seq) || member.sequence === channel.next_command_seq ||
        !consumingRun || consumingRun.group_id !== item.id || !["agent", "leader", "continuation"].includes(consumingRun.step_kind) ||
        consumingRun.agent_id !== rootRuns.get(item.id)?.agent_id) return false;
      if (command.target_id !== owner.id && (owner.kind !== "squad" || commandTarget.kind !== "agent" || commandTarget.resource_id !== consumingRun.agent_id)) return false;
      if (!unique(member.reader_run_ids) || !member.reader_run_ids.includes(member.consuming_run_id) ||
        member.reader_run_ids.some((id) => runsById.get(id)?.group_id !== item.id)) return false;
      const position = `${member.channel_id}:${member.sequence}`;
      const previousOwner = sequenceOwners.get(position);
      if (previousOwner !== undefined && previousOwner !== member.command_id) return false;
      sequenceOwners.set(position, member.command_id);
      if (command.group_id === item.id) continue;
      // Requeue changes only the command's current association. Its original
      // An unlaunched continuation can return its input while unrelated member
      // work remains active, only after every immutable prompt reader is safe.
      if (!continuationInputNeverLaunched(member) && (!terminal || item.state === "succeeded" || allRunsNeverLaunched.get(item.id) !== true)) return false;
      const currentGroup = command.group_id === null ? undefined : groupsById.get(command.group_id);
      if (currentGroup?.channel_id === item.channel_id && atMostVersion(currentGroup.generation, item.generation)) return false;
    }
  }
  for (const item of targets) {
    if (item.channel_id === null && (item.group_generation !== "0" || item.active_group_id !== null)) return false;
    if (item.channel_id !== null) {
      const owner = channelsById.get(item.channel_id);
      if (!owner || owner.queue_state !== item.control.queue_state || owner.control_version !== item.control.control_version ||
        owner.group_generation !== item.group_generation || owner.active_group_id !== item.active_group_id || owner.pause_reason !== item.pause_reason) return false;
    }
  }
  for (const item of channels) {
    if (item.active_group_id === null) continue;
    const active = groupsById.get(item.active_group_id);
    if (!active || active.channel_id !== item.id || active.settled_at !== null || active.generation !== item.group_generation) return false;
  }

  const roots = new Set<string>();
  for (const item of runs) {
    const owner = targetsById.get(item.target_id);
    const root = groupsById.get(item.group_id);
    if (!owner || !root || root.target_id !== item.target_id || root.command_id !== item.command_id ||
      root.channel_id !== item.channel_id || root.batch_id !== item.batch_id ||
      root.target_epoch !== item.target_epoch || root.config_version !== item.config_version) return false;
    const terminal = item.settled_at !== null;
    if (terminal !== (item.progress_state === "settled") || (terminal && item.proof_state === "unknown") || (root.settled_at !== null && !terminal)) return false;
    if (owner.kind === "agent" && (item.step_kind !== "agent" || item.agent_id !== owner.resource_id)) return false;
    if (owner.kind === "squad" && item.step_kind === "agent") return false;
    if (item.step_kind === "agent" || item.step_kind === "leader") {
      if (item.parent_run_id !== null || roots.has(item.group_id)) return false;
      roots.add(item.group_id);
    } else {
      const parent = item.parent_run_id === null ? undefined : runsById.get(item.parent_run_id);
      if (!parent || parent.group_id !== item.group_id || parent.id === item.id) return false;
      if (item.step_kind === "continuation" && item.agent_id !== rootRuns.get(item.group_id)?.agent_id) return false;
    }
  }
  if (groups.some((item) => !roots.has(item.id))) return false;
  // Iterative traversal handles deep, untrusted parent chains without recursion.
  const checked = new Set<string>();
  for (const item of runs) {
    const path = new Set<string>();
    let current: z.infer<typeof run> | undefined = item;
    while (current && !checked.has(current.id)) {
      if (path.has(current.id)) return false;
      path.add(current.id);
      current = current.parent_run_id === null ? undefined : runsById.get(current.parent_run_id);
    }
    for (const id of path) checked.add(id);
  }
  return true;
}

/** Complete projections only: missing rows cannot be interpreted as idle work. */
export const CollaborationExecutionSchema = execution.refine(consistentExecution, "Execution identities, control snapshots and lineage must be consistent");
export type CollaborationExecution = z.infer<typeof CollaborationExecutionSchema>;
export type CollaborationExecutionChannel = z.infer<typeof channel>;
export type CollaborationTargetHoldReason = z.infer<typeof CollaborationTargetHoldReasonSchema>;
export type CollaborationExecutionTarget = z.infer<typeof target>;
export type CollaborationExecutionCommand = z.infer<typeof command>;
export type CollaborationExecutionGroup = z.infer<typeof group>;
export type CollaborationExecutionRun = z.infer<typeof run>;
export const CollaborationExecutionRunSchema = run;
