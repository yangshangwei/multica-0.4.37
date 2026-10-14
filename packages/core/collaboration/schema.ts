import { z } from "zod";
import { parseWithFallback } from "../api/schema";

const decimal = /^[1-9][0-9]{0,18}$/;
const positiveVersion = z.string().regex(decimal).refine(
  (value) => decimal.test(value) && BigInt(value) <= 9223372036854775807n,
  "Version exceeds a signed 64-bit integer",
);
const statusCategory = z.enum(["backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"]);
const statusAction = z.enum(["change_status", "complete", "cancel", "reopen", "request_information", "return_information", "submit_acceptance"]);

export type CollaborationStatusAction = z.infer<typeof statusAction>;
export type CollaborationLifecycleAction = Extract<CollaborationStatusAction, "complete" | "cancel" | "reopen">;

const accountSummary = z.object({
  user_id: z.string().uuid(),
  name: z.string().catch("Unavailable account"),
  avatar_url: z.string().catch(""),
  external: z.boolean(),
});

const sourceWorkspaceSummary = z.object({
  id: z.string().uuid(),
  name: z.string().catch(""),
  identifier: z.string().catch(""),
});

// The open top of the information request chain. A malformed present value
// makes the issue unavailable: its requester decides where a return lands.
const informationRequest = z.object({
  id: z.string().uuid(),
  requester: accountSummary,
  recipient: accountSummary,
  note: z.string().catch(""),
  // Null when the source has no valid restore target; the return is unavailable.
  return_status: z.string().min(1).nullable(),
  return_status_fallback: z.boolean(),
  created_at: z.string().datetime({ offset: true }),
});

export const CollaborationIssueSchema = z.object({
  id: z.string().uuid(),
  title: z.string(),
  description: z.string(),
  // Absence on an older server is unknown, not a date-clear instruction.
  start_date: z.iso.date().nullish(),
  due_date: z.iso.date().nullish(),
  status: z.string().min(1),
  status_category: statusCategory,
  priority: z.string(),
  source_workspace: sourceWorkspaceSummary,
  creator: accountSummary,
  handler: accountSummary,
  collaboration_revision: positiveVersion,
  lifecycle_epoch: positiveVersion,
  write_context: z.object({
    participant_epoch: positiveVersion,
    account_epoch: positiveVersion,
  }).nullish(),
  session: z.object({
    id: z.string().uuid(),
    queue_state: z.enum(["paused", "active", "closed"]),
    control_version: positiveVersion,
  }),
  capabilities: z.object({
    reference: z.boolean(), read: z.boolean(), comment: z.boolean(),
    control_ai: z.boolean(), submit_command: z.boolean(),
    invite: z.boolean(), transfer: z.boolean(), remove_participants: z.boolean(),
    leave: z.boolean(), edit: z.boolean(), edit_source_structure: z.boolean(),
    reorder: z.boolean(), change_status: z.boolean(), complete: z.boolean(),
    cancel: z.boolean(), reopen: z.boolean(), resolve_thread: z.boolean(), react: z.boolean(),
    // Directed handoffs are explicit-true only; an older server omits them.
    request_information: z.boolean().catch(false),
    return_information: z.boolean().catch(false),
    submit_acceptance: z.boolean().catch(false),
  }),
  current_information_request: informationRequest.nullish(),
});

export type CollaborationIssue = z.infer<typeof CollaborationIssueSchema>;

export const CollaborationWorkbenchSchema = z.object({
  items: z.array(z.object({
    id: z.string().uuid(), title: z.string(), status: z.string(),
    status_category: z.string(), priority: z.string(),
    collaboration_revision: positiveVersion,
    updated_at: z.string().datetime({ offset: true }),
    source_workspace: sourceWorkspaceSummary,
    handler: accountSummary,
  })),
  total: z.number().int().nonnegative().safe(),
  next_cursor: z.string().max(512).nullable(),
});

export type CollaborationWorkbench = z.infer<typeof CollaborationWorkbenchSchema>;

export const CollaborationStatusOptionsSchema = z.object({
  action: statusAction,
  status_options_version: z.string().regex(/^[a-f0-9]{64}$/),
  default_status: z.string().min(1),
  options: z.array(z.object({
    key: z.string().min(1),
    name: z.string(),
    category: statusCategory,
  })).min(1),
}).refine(
  (value) => value.options.some((option) => option.key === value.default_status),
  "Default status must be an available option",
).refine(
  (value) => new Set(value.options.map((option) => option.key)).size === value.options.length,
  "Status option keys must be unique",
).refine(
  (value) => value.options.every((option) => {
    switch (value.action) {
      case "complete": return option.category === "done";
      case "cancel": return option.category === "cancelled";
      case "request_information": return option.category === "blocked";
      case "submit_acceptance": return option.category === "in_review";
      case "return_information":
      case "change_status":
      case "reopen": return option.category !== "done" && option.category !== "cancelled";
      default: return false;
    }
  }),
  "Status category must match the action",
).refine(
  (value) => !["request_information", "return_information", "submit_acceptance"].includes(value.action) || value.options.length === 1,
  "A directed handoff has exactly one prescribed status",
);

export type CollaborationStatusOptions = z.infer<typeof CollaborationStatusOptionsSchema>;

export const CollaborationOperationSchema = z.object({
  code: z.string().refine((value) => value.trim().length > 0, "Operation code must not be empty"),
  collaboration_revision: positiveVersion,
  status_options: CollaborationStatusOptionsSchema.nullish(),
});

export type CollaborationOperation = z.infer<typeof CollaborationOperationSchema>;

/** Capture these versions and the request ID once for the original intent. */
export interface CollaborationWriteInput {
  requestId: string;
  expectedCollaborationRevision: string;
  expectedLifecycleEpoch: string;
  expectedParticipantEpoch: string;
  expectedAccountEpoch: string;
}

export interface CollaborationParticipantInput extends CollaborationWriteInput {
  targetUserId: string;
}

/** An ordinary transfer and an information request have different effects. */
export type CollaborationHandoffPurpose = "transfer" | "information";

/** Preserve both values from the action options shown for this exact intent. */
export interface CollaborationDirectedStatusInput {
  expectedStatus: string;
  statusOptionsVersion: string;
}

export type CollaborationHandoffInput = CollaborationParticipantInput & { note: string } & (
  | { purpose: "transfer" }
  | ({ purpose: "information" } & CollaborationDirectedStatusInput)
);

export interface CollaborationInformationReturnInput extends CollaborationWriteInput, CollaborationDirectedStatusInput {
  informationRequestId: string;
  note: string;
}

export interface CollaborationAcceptanceInput extends CollaborationWriteInput, CollaborationDirectedStatusInput {
  note: string;
}

/** Omit untouched dates; null or an empty string explicitly clears a date. */
export interface CollaborationEditInput extends CollaborationWriteInput {
  title?: string;
  description?: string;
  startDate?: string | null;
  dueDate?: string | null;
}

export interface CollaborationStatusInput extends CollaborationWriteInput {
  status: string;
  statusOptionsVersion: string;
}

export interface CollaborationLifecycleInput extends CollaborationStatusInput {
  action: CollaborationLifecycleAction;
}

/** Null means unavailable, never an ordinary issue or an empty successful list. */
export function parseCollaborationIssue(data: unknown): CollaborationIssue | null {
  return parseWithFallback<CollaborationIssue | null>(data, CollaborationIssueSchema, null, {
    endpoint: "GET collaboration issue",
    redact: true,
  });
}

export function parseCollaborationWorkbench(data: unknown): CollaborationWorkbench | null {
  return parseWithFallback<CollaborationWorkbench | null>(data, CollaborationWorkbenchSchema, null, {
    endpoint: "GET account workbench",
    redact: true,
  });
}

/** A missing receipt leaves the original request unconfirmed, never successful. */
export function parseCollaborationOperation(data: unknown): CollaborationOperation | null {
  return parseWithFallback<CollaborationOperation | null>(data, CollaborationOperationSchema, null, {
    endpoint: "collaboration operation",
    redact: true,
  });
}

export function parseCollaborationStatusOptions(data: unknown): CollaborationStatusOptions | null {
  return parseWithFallback<CollaborationStatusOptions | null>(data, CollaborationStatusOptionsSchema, null, {
    endpoint: "collaboration status options",
    redact: true,
  });
}
