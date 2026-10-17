import { z } from "zod";
import { nonnegativeVersion, positiveVersion } from "./version";

const uuid = z.string().uuid().transform((value) => value.toLowerCase());
const timestamp = z.string().datetime({ offset: true }).nullable();
const disposition = z.enum(["stopped", "cancelled"]).nullable();

const target = z.object({
  id: uuid,
  kind: z.enum(["agent", "squad"]),
  resource_id: uuid,
  target_epoch: positiveVersion,
  config_version: positiveVersion,
  control: z.object({ scope: z.literal("target"), queue_state: z.enum(["paused", "active"]), control_version: positiveVersion }),
  group_generation: nonnegativeVersion,
  next_command_seq: positiveVersion,
  active_group_id: uuid.nullable(),
  pause_reason: z.string(),
  revoked_at: timestamp,
});

const command = z.object({
  id: uuid,
  target_id: uuid,
  input_event_id: uuid,
  sequence: positiveVersion,
  author_user_id: uuid,
  state: z.enum(["waiting", "in_flight", "succeeded", "failed", "stopped", "cancelled"]),
});

const group = z.object({
  id: uuid,
  target_id: uuid,
  command_id: uuid,
  generation: positiveVersion,
  target_epoch: positiveVersion,
  config_version: positiveVersion,
  state: z.enum(["active", "waiting_children", "ending", "succeeded", "failed", "stopped", "cancelled"]),
  expansion_sealed_at: timestamp,
  forced_disposition: disposition,
  settled_at: timestamp,
});

const run = z.object({
  id: uuid,
  command_id: uuid,
  target_id: uuid,
  group_id: uuid,
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
});

const execution = z.object({
  protocol_version: z.literal(2),
  issue_id: uuid,
  session_id: uuid,
  // This foundation does not implement public execution/control endpoints.
  availability: z.literal("unavailable"),
  unavailable_reason: z.literal("shared_runtime_unavailable"),
  target_set_version: positiveVersion,
  control: z.object({ scope: z.literal("issue"), queue_state: z.enum(["paused", "active", "closed"]), control_version: positiveVersion }),
  targets: z.array(target),
  commands: z.array(command),
  groups: z.array(group),
  runs: z.array(run),
  legacy_unresolved: z.object({ command_ids: z.array(uuid), run_ids: z.array(uuid), active_run_id: uuid.nullable() }),
});

function unique(values: string[]): boolean {
  return new Set(values).size === values.length;
}

function atMostVersion(left: string, right: string): boolean {
  // Zod can run aggregate refinements after a child string refinement failed.
  // Invalid strings must become unavailable, never throw from BigInt parsing.
  return nonnegativeVersion.safeParse(left).success && nonnegativeVersion.safeParse(right).success && BigInt(left) <= BigInt(right);
}

function consistentExecution(value: z.infer<typeof execution>): boolean {
  const { targets, commands, groups, runs, legacy_unresolved: legacy } = value;
  const targetsById = new Map(targets.map((item) => [item.id, item]));
  const commandsById = new Map(commands.map((item) => [item.id, item]));
  const groupsById = new Map(groups.map((item) => [item.id, item]));
  const groupsByCommand = new Map(groups.map((item) => [item.command_id, item]));
  const runsById = new Map(runs.map((item) => [item.id, item]));
  if (targetsById.size !== targets.length || commandsById.size !== commands.length || groupsById.size !== groups.length || runsById.size !== runs.length ||
    groupsByCommand.size !== groups.length || !unique(targets.map((item) => `${item.kind}:${item.resource_id}`)) ||
    !unique(commands.map((item) => `${item.target_id}:${item.sequence}`)) || !unique(commands.map((item) => `${item.input_event_id}:${item.target_id}`)) ||
    !unique(groups.map((item) => `${item.target_id}:${item.generation}`)) || !unique(runs.map((item) => `${item.group_id}:${item.step_key}`)) ||
    !unique(legacy.command_ids) || !unique(legacy.run_ids) || legacy.command_ids.some((id) => commandsById.has(id)) || legacy.run_ids.some((id) => runsById.has(id)) ||
    (legacy.active_run_id !== null && !legacy.run_ids.includes(legacy.active_run_id))) return false;

  for (const item of commands) {
    const owner = targetsById.get(item.target_id);
    if (!owner || !atMostVersion(item.sequence, owner.next_command_seq) || item.sequence === owner.next_command_seq) return false;
    if (item.state !== "waiting" && item.state !== "cancelled" && !groupsByCommand.has(item.id)) return false;
  }
  for (const item of groups) {
    const owner = targetsById.get(item.target_id);
    const input = commandsById.get(item.command_id);
    if (!owner || !input || input.target_id !== owner.id || !atMostVersion(item.generation, owner.group_generation) ||
      !atMostVersion(item.target_epoch, owner.target_epoch) || !atMostVersion(item.config_version, owner.config_version)) return false;
    const terminal = item.state !== "active" && item.state !== "waiting_children" && item.state !== "ending";
    if (terminal !== (item.settled_at !== null)) return false;
    if (terminal) {
      if (item.expansion_sealed_at === null || input.state !== item.state || (item.forced_disposition !== null && item.forced_disposition !== item.state)) return false;
    } else if (owner.active_group_id !== item.id || owner.group_generation !== item.generation || input.state !== "in_flight") return false;
  }
  for (const item of targets) {
    if (item.active_group_id === null) continue;
    const active = groupsById.get(item.active_group_id);
    if (!active || active.target_id !== item.id || active.settled_at !== null || active.generation !== item.group_generation) return false;
  }

  const roots = new Set<string>();
  for (const item of runs) {
    const owner = targetsById.get(item.target_id);
    const root = groupsById.get(item.group_id);
    if (!owner || !root || root.target_id !== item.target_id || root.command_id !== item.command_id ||
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
    }
  }
  if (groups.some((item) => item.settled_at !== null && !roots.has(item.id))) return false;
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
export type CollaborationExecutionTarget = z.infer<typeof target>;
export type CollaborationExecutionCommand = z.infer<typeof command>;
export type CollaborationExecutionGroup = z.infer<typeof group>;
export type CollaborationExecutionRun = z.infer<typeof run>;
