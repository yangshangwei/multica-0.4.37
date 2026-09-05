import type { AgentAutonomyLevel, AgentInvocationTargetInput } from "./agent";
import type { Squad } from "./squad";

/**
 * A built-in role template: one of the engineering roles a workspace can staff
 * in one step.
 *
 * Creating from a template produces an ORDINARY agent. `instructions` here is
 * the text that gets copied onto that agent's row, which is why the picker can
 * show it: what you read is what you get, and the workspace owns it afterwards.
 */
export interface AgentRoleTemplate {
  /** Stable identity, recorded on created agents as `template_key`. */
  key: string;
  version: number;
  /** Default agent name — stored, and renameable afterwards. */
  name: string;
  /** Localized role label for display. Follows the requested language. */
  title: string;
  description: string;
  /** Widened to string at the boundary: a level this client does not know must
   *  still render. Narrow with `isKnownAutonomyLevel` before branching. */
  autonomy_level: string;
  avatar_emoji: string;
  max_concurrent_tasks: number;
  /** Role skills materialized as workspace skills and attached on creation. */
  skill_names: string[];
  instructions: string;
}

/** One seat in a squad template's roster. */
export interface SquadTemplateRole {
  template_key: string;
  title: string;
  name: string;
  autonomy_level: string;
  avatar_emoji: string;
  /** What the leader is told this seat is for. Context, not permission. */
  role: string;
}

/**
 * A built-in squad template. Staffing one creates the missing role agents, the
 * squad, and its membership together — the leader still routes work by
 * @mention, exactly as in a hand-built squad.
 */
export interface SquadTemplate {
  key: string;
  version: number;
  name: string;
  title: string;
  description: string;
  avatar_emoji: string;
  /** The routing policy copied into `squad.instructions`, leader-only. */
  instructions: string;
  leader: SquadTemplateRole;
  members: SquadTemplateRole[];
}

/**
 * Result of staffing a squad template. The two id lists matter to the person who
 * asked: an agent that already existed for a role is REUSED as it stands, edits
 * and all, rather than recreated.
 */
export interface StaffedSquad {
  squad: Squad;
  created_agent_ids: string[];
  reused_agent_ids: string[];
}

export interface CreateAgentFromTemplateRequest {
  template_key: string;
  runtime_id: string;
  /** Overrides the template's default name. */
  name?: string;
  model?: string;
  thinking_level?: string;
  service_tier?: string;
  permission_mode?: "private" | "public_to";
  invocation_targets?: AgentInvocationTargetInput[];
  language?: string;
}

export interface CreateSquadFromTemplateRequest {
  template_key: string;
  /** Binds every agent this call creates. One runtime per squad in this version. */
  runtime_id: string;
  name?: string;
  /** Applied to every agent this call creates. Absent means private, so
   *  staffing never silently widens who can run something. */
  permission_mode?: "private" | "public_to";
  invocation_targets?: AgentInvocationTargetInput[];
  language?: string;
}

/**
 * High-risk action classes that require a person's approval before an agent may
 * carry them out.
 */
export type ApprovalRiskClass =
  | "production_release"
  | "database_migration"
  | "secret_access"
  | "external_notification"
  | "destructive_operation";

export type ApprovalStatus =
  | "pending"
  | "approved"
  | "rejected"
  | "executed"
  | "cancelled";

/**
 * One approval request: an agent's statement of what it intends to do, and the
 * record of who authorized it.
 *
 * `status` and `risk_class` are strings at the boundary rather than the unions
 * above, because a newer backend may add a value. Treat any unrecognised status
 * as NOT approved.
 */
export interface AgentApproval {
  id: string;
  workspace_id: string;
  agent_id: string;
  /** The task turn that filed it, when one was in flight. Provenance only. */
  task_id: string | null;
  issue_id: string | null;
  risk_class: string;
  /** One line naming the single action. */
  summary: string;
  /** The full plan a person reviews: commands, expected effect, rollback. */
  plan: string;
  status: string;
  decided_by: string | null;
  decided_at: string | null;
  decision_note: string;
  executed_at: string | null;
  execution_note: string;
  created_at: string;
  updated_at: string;
}

export interface CreateAgentApprovalRequest {
  /** Omitted by an agent: identity comes from its task token, not the body. */
  agent_id?: string;
  issue_id?: string;
  risk_class: ApprovalRiskClass;
  summary: string;
  plan?: string;
}

/** Autonomy levels a workspace may assign, in increasing order. */
export const AGENT_AUTONOMY_LEVELS: AgentAutonomyLevel[] = [
  "observer",
  "contributor",
  "coordinator",
  "operator",
];

/**
 * Narrows a wire value to a level this client understands.
 *
 * An unknown or empty value is NOT a level: it means the backend declared no
 * policy for that agent, and the UI must not claim a limit that is not being
 * enforced.
 */
export function isKnownAutonomyLevel(
  value: string | undefined | null,
): value is AgentAutonomyLevel {
  return (
    !!value && (AGENT_AUTONOMY_LEVELS as string[]).includes(value)
  );
}

export const APPROVAL_RISK_CLASSES: ApprovalRiskClass[] = [
  "production_release",
  "database_migration",
  "secret_access",
  "external_notification",
  "destructive_operation",
];

/** Every status this client has copy for, in the order the state machine walks. */
export const APPROVAL_STATUSES: ApprovalStatus[] = [
  "pending",
  "approved",
  "rejected",
  "executed",
  "cancelled",
];

/**
 * Narrows a wire status to one this client understands.
 *
 * Only for choosing localized copy. Never branch authorization on it: a status
 * this build does not recognise is not approved, and `isApprovalActionable` is
 * the function that answers that question.
 */
export function isKnownApprovalStatus(
  value: string | undefined | null,
): value is ApprovalStatus {
  return !!value && (APPROVAL_STATUSES as string[]).includes(value);
}

/**
 * Narrows a wire risk class to one this client has copy for. A newer backend may
 * name a class this build has never heard of; the raw value is shown rather than
 * hidden, because "some high-risk action" is still the thing a person must read.
 */
export function isKnownApprovalRiskClass(
  value: string | undefined | null,
): value is ApprovalRiskClass {
  return !!value && (APPROVAL_RISK_CLASSES as string[]).includes(value);
}

/** True only for the one status that authorizes an action. */
export function isApprovalActionable(status: string): boolean {
  return status === "approved";
}

/** True when a person still has to look at this request. */
export function isApprovalPending(status: string): boolean {
  return status === "pending";
}
