import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import type { IssuePriority } from "../types/issue";
import { ISSUE_PROPERTY_TYPES, parseActorRef, type IssuePropertyValue } from "../types/property";
import { CollaborationExecutionSchema } from "./execution-schema";
import { positiveVersion } from "./version";

const statusCategory = z.enum(["backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"]);
const statusAction = z.enum(["change_status", "complete", "cancel", "reopen", "request_information", "return_information", "submit_acceptance"]);

export type CollaborationStatusAction = z.infer<typeof statusAction>;
export type CollaborationLifecycleAction = Extract<CollaborationStatusAction, "complete" | "cancel" | "reopen">;

export const CollaborationAccountSummarySchema = z.object({
  user_id: z.string().uuid(),
  name: z.string().catch("Unavailable account"),
  avatar_url: z.string().catch(""),
  external: z.boolean(),
});
const accountSummary = CollaborationAccountSummarySchema;

const sourceWorkspaceSummary = z.object({
  id: z.string().uuid(),
  name: z.string().catch(""),
  identifier: z.string().catch(""),
});

export const CollaborationRecipientSchema = z.object({
  issue_id: z.string().uuid().nullable(),
  source_workspace_id: z.string().uuid(),
  recipient: accountSummary,
});

export type CollaborationRecipient = z.infer<typeof CollaborationRecipientSchema>;

/** Resolution only confirms an account; invitation and transfer remain explicit writes. */
export type CollaborationRecipientInput = {
  kind: "username" | "email" | "user_id";
  value: string;
} & (
  | { issueId: string; sourceWorkspaceId?: never }
  | { sourceWorkspaceId: string; issueId?: never }
);

const labelSummary = z.object({
  id: z.string().uuid(),
  name: z.string(),
  color: z.string(),
});

const labelSummaries = z.array(labelSummary).refine(
  (items) => new Set(items.map((item) => item.id.toLowerCase())).size === items.length,
  "Label identities must be unique",
);

// Ordinary catalog schemas default missing fields and drop unsupported values.
// Editing a private issue requires a complete, unambiguous definition instead.
const propertyDefinition = z.object({
  id: z.string().uuid(),
  name: z.string(),
  type: z.enum(ISSUE_PROPERTY_TYPES),
  icon: z.string().catch(""),
  config: z.object({
    options: z.array(z.object({ id: z.string().min(1), name: z.string(), color: z.string() })).optional(),
  }),
  archived: z.boolean(),
}).refine(
  (definition) => {
    const options = definition.config.options;
    if (definition.type !== "select" && definition.type !== "multi_select") return options === undefined;
    return options !== undefined && options.length > 0 && new Set(options.map((option) => option.id)).size === options.length;
  },
  "Property options must match the definition type and have unique identities",
);

const propertyDefinitions = z.array(propertyDefinition).refine(
  (items) => new Set(items.map((item) => item.id.toLowerCase())).size === items.length,
  "Property identities must be unique",
);

export type CollaborationPropertyDefinition = z.infer<typeof propertyDefinition>;
export type CollaborationLabel = z.infer<typeof labelSummary>;

const propertyValues = z.record(
  z.string().uuid(), z.union([z.string(), z.number(), z.boolean(), z.array(z.string())]),
).refine(
  (values) => new Set(Object.keys(values).map((id) => id.toLowerCase())).size === Object.keys(values).length,
  "Property value identities must be unique",
);

const actorValue = z.string().refine((value) => {
  const reference = parseActorRef(value);
  return reference !== null && z.uuid().safeParse(reference.id).success;
}, "Actor reference must contain a member UUID");

function propertyValueMatchesDefinition(value: IssuePropertyValue, definition: CollaborationPropertyDefinition): boolean {
  switch (definition.type) {
    case "text":
    case "url": return typeof value === "string";
    case "number": return typeof value === "number";
    case "checkbox": return typeof value === "boolean";
    case "date": return z.iso.date().safeParse(value).success;
    case "select": return typeof value === "string" && definition.config.options?.some((option) => option.id === value) === true;
    case "multi_select": return Array.isArray(value) && value.length > 0 && value.every((id) => definition.config.options?.some((option) => option.id === id) === true);
    case "actor": return actorValue.safeParse(value).success;
    case "multi_actor": return Array.isArray(value) && value.every((item) => actorValue.safeParse(item).success);
    default: return false;
  }
}

export const CollaborationFieldOptionsSchema = z.object({
  issue_id: z.string().uuid(),
  source_workspace_id: z.string().uuid(),
  collaboration_revision: positiveVersion,
  labels: labelSummaries,
  properties: propertyDefinitions.refine((items) => items.every((item) => item.archived === false), "Editing options must be active"),
});

export type CollaborationFieldOptions = z.infer<typeof CollaborationFieldOptionsSchema>;

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
  // Missing metadata is unknown on older servers, never an empty edit baseline.
  labels: labelSummaries.optional(),
  properties: propertyValues.optional(),
  property_definitions: propertyDefinitions.optional(),
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
  // Missing on older servers: human reads/workflow remain usable, execution
  // is explicitly unavailable. A malformed present protocol is never dropped.
  execution: CollaborationExecutionSchema.optional().transform((value) => value ?? null),
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
}).refine((issue) => {
  const values = issue.properties;
  const definitions = issue.property_definitions;
  if (values === undefined || definitions === undefined) return values === undefined && definitions === undefined;
  const byId = new Map(definitions.map((definition) => [definition.id.toLowerCase(), definition]));
  const entries = Object.entries(values);
  return entries.length === definitions.length && entries.every(([id, value]) => {
    const definition = byId.get(id.toLowerCase());
    return definition !== undefined && propertyValueMatchesDefinition(value, definition);
  });
}, "Current property values require matching definitions and value types").refine((issue) => {
  const execution = issue.execution;
  return execution === null || (execution.issue_id === issue.id.toLowerCase() && execution.session_id === issue.session.id.toLowerCase() &&
    execution.control.queue_state === issue.session.queue_state && execution.control.control_version === issue.session.control_version);
}, "Execution must belong to the current issue and control snapshot").transform((issue) => ({
  ...issue,
  // Human controls need a supported snapshot and current participant context.
  // Stop/cancel do not require runtime availability; Resume rechecks admission.
  capabilities: { ...issue.capabilities, control_ai: issue.capabilities.control_ai === true && issue.execution !== null && issue.write_context != null, submit_command: false },
}));

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
  status: z.number().int().min(200).max(599).optional(),
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

/** Omit untouched fields. Dates clear with null/empty; metadata null preserves. */
export interface CollaborationEditInput extends CollaborationWriteInput {
  title?: string;
  description?: string;
  startDate?: string | null;
  dueDate?: string | null;
  priority?: IssuePriority | null;
  /** Replaces the complete label set; [] clears it, null preserves it. */
  labelIds?: string[] | null;
  /** A per-definition patch; null at a key clears it, outer null preserves all. */
  properties?: Record<string, IssuePropertyValue | null> | null;
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

export function parseCollaborationRecipient(data: unknown): CollaborationRecipient | null {
  return parseWithFallback<CollaborationRecipient | null>(data, CollaborationRecipientSchema, null, {
    endpoint: "collaboration recipient resolution",
    redact: true,
  });
}

export function parseCollaborationFieldOptions(data: unknown): CollaborationFieldOptions | null {
  return parseWithFallback<CollaborationFieldOptions | null>(data, CollaborationFieldOptionsSchema, null, {
    endpoint: "collaboration field options",
    redact: true,
  });
}
