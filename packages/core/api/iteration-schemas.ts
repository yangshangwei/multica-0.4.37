import { z } from "zod";
import { parseWithFallback } from "./schema";
const uuid = z.string().uuid();
const revision = z.number().int().positive().safe();
const count = z.number().int().nonnegative().safe();
const timezone = z
  .string()
  .min(1)
  .refine((value) => {
    try {
      new Intl.DateTimeFormat("en", { timeZone: value });
      return true;
    } catch {
      return false;
    }
  }, "Invalid timezone");
const time = z.iso.datetime({ offset: true });
export const IterationCapabilitiesSchema = z.object({
  workspace_id: uuid,
  schema_version: z.literal(1),
  supported: z.boolean(),
  enabled: z.boolean(),
  manual: z.boolean(),
  atomic_handoff: z.boolean(),
});
export const IterationSettingsSchema = z.object({
  workspace_id: uuid,
  enabled: z.boolean(),
  revision,
  planning_timezone: timezone.nullable(),
  effective_timezone: timezone,
  timezone_configured: z.boolean(),
});
export const IterationSchema = z.object({
  id: uuid,
  workspace_id: uuid,
  name: z.string(),
  description: z.string().nullable(),
  coordinator_user_id: uuid.nullable(),
  status: z.string(),
  mode: z.string(),
  start_date: z.iso.date(),
  end_date: z.iso.date(),
  timezone,
  revision,
  scope_revision: revision,
  started_at: time.nullable(),
  logical_ended_at: time.nullable(),
  processed_at: time.nullable(),
});
export const IterationStatisticsSchema = z.object({
  original: count,
  current: count,
  cancelled: count,
  effective: count,
  completed: count,
  original_completed: count,
  remaining: count,
  added_unique: count,
  removed_events: count,
  reentry_events: count,
  cancel_events: count,
  reopen_events: count,
  started: count,
  initial_effective: count,
  net_effective_change: z.number(),
  net_effective_change_ratio: z.number().nullable(),
  effective_ratio: z.number().nullable(),
  original_ratio: z.number().nullable(),
  chart: z.array(
    z.object({
      date: z.iso.date(),
      effective: count,
      completed: count,
      original: count,
    }),
  ),
  calculated_at: time,
});
export const HistoricalIterationIssueSchema = z.object({
  issue_id: uuid,
  identifier: z.string(),
  title: z.string(),
  project_id: uuid.nullable(),
  project_name: z.string().nullable(),
  assignee_type: z.string().nullable(),
  assignee_id: uuid.nullable(),
  assignee_name: z.string().nullable(),
  status_key: z.string(),
  status_category: z.string(),
  was_completed_at_start: z.boolean(),
  rollover_count: count,
  priority: z.string().nullable().optional(),
  labels: z.array(z.object({ id: uuid, name: z.string() })).nullable().optional(),
});
export const IterationEventSchema = z.object({
  id: uuid,
  sequence: revision,
  iteration_id: uuid,
  issue_id: uuid.nullable(),
  operation_id: uuid,
  kind: z.string(),
  actor: z.unknown(),
  occurred_at: time,
  sampled_at: time,
  before_facts: z.unknown(),
  after_facts: z.unknown(),
  reason: z.string().nullable(),
});
export const IterationSnapshotSchema = z.object({
  schema_version: z.literal(1),
  workspace_id: uuid,
  iteration_id: uuid,
  operation_id: uuid,
  end_type: z.string(),
  reason: z.string(),
  logical_ended_at: time,
  processed_at: time,
  original: z.array(HistoricalIterationIssueSchema),
  scope: z.array(HistoricalIterationIssueSchema),
  events: z.array(IterationEventSchema),
  statistics: IterationStatisticsSchema,
  destinations: z.array(
    z.object({
      issue_id: uuid,
      target_iteration_id: uuid.nullable(),
      rollover_count_before: count,
      rollover_count_after: count,
    }),
  ),
});
export const IterationListSchema = z
  .object({
    workspace_id: uuid,
    items: z.array(IterationSchema),
    next_cursor: z.string().nullable(),
  })
  .refine(
    (value) =>
      value.items.every((item) => item.workspace_id === value.workspace_id),
    "Iteration workspace mismatch",
  );
export const IterationDetailSchema = z
  .object({
    workspace_id: uuid,
    iteration: IterationSchema,
    statistics: IterationStatisticsSchema,
    snapshot: IterationSnapshotSchema.nullable(),
  })
  .refine(
    (value) =>
      value.iteration.workspace_id === value.workspace_id &&
      (!value.snapshot ||
        (value.snapshot.workspace_id === value.workspace_id &&
          value.snapshot.iteration_id === value.iteration.id)),
    "Iteration detail identity mismatch",
  );
export const IterationIssuesSchema = z.object({
  workspace_id: uuid,
  iteration_id: uuid,
  scope_revision: revision,
  items: z.array(HistoricalIterationIssueSchema),
  total: count,
  next_cursor: z.string().nullable(),
});
export const IterationEventsSchema = z.object({
  workspace_id: uuid,
  iteration_id: uuid,
  items: z.array(IterationEventSchema),
  next_cursor: z.string().nullable(),
});
export const IterationDraftSchema = z.object({
  operation: z.enum([
    "start",
    "move",
    "end",
    "cancel",
    "disable",
    "delete",
    "handoff",
  ]),
  iteration_id: uuid.nullable(),
  expected_iteration_revision: revision.nullable(),
  expected_scope_revision: revision.nullable(),
  expected_settings_revision: revision,
  reason: z.string().nullable(),
  moves: z.array(
    z.object({
      issue_id: uuid,
      expected_issue_revision: revision,
      expected_source_id: uuid.nullable(),
      target_id: uuid.nullable(),
      allow_completed: z.boolean(),
    }),
  ),
  start: z
    .object({
      target_id: uuid,
      mode: z.enum(["scheduled", "today"]),
      terminal_choices: z.array(
        z.object({ issue_id: uuid, retain: z.boolean() }),
      ),
    })
    .nullable(),
});
export const IterationPreviewSchema = z.object({
  workspace_id: uuid,
  actor_user_id: uuid,
  draft: IterationDraftSchema,
  preview_hash: z.string().min(1),
  previewed_at: time,
  start_preview: z
    .object({
      reference_date: z.iso.date(),
      effective_start_date: z.iso.date(),
      effective_end_date: z.iso.date(),
      timezone,
    })
    .nullable(),
  iterations: z.array(IterationSchema),
  issues: z.array(
    z.object({
      issue_id: uuid,
      identifier: z.string(),
      revision,
      source_id: uuid.nullable(),
      status_category: z.string(),
      project: z.object({
        id: uuid.nullable(),
        name: z.string().nullable(),
        type: z.string().nullable(),
        available: z.boolean(),
      }),
      assignee: z.object({
        id: uuid.nullable(),
        name: z.string().nullable(),
        type: z.string().nullable(),
        available: z.boolean(),
      }),
      title: z.string(),
      running_execution_count: count,
      rollover_count: count,
    }),
  ),
  statistics: z.record(z.string(), IterationStatisticsSchema),
  recipients: z.array(uuid),
  invalid_items: z.array(
    z.object({ issue_id: uuid.nullable(), code: z.string() }),
  ),
  total_affected: count,
  complete: z.boolean(),
});
export const IterationWriteResultSchema = z.object({
  workspace_id: uuid,
  request_id: uuid,
  operation_id: uuid,
  operation: z.string(),
  replayed: z.boolean(),
  iteration_ids: z.array(uuid),
  result: z.object({
    snapshot_id: uuid.nullable(),
    deleted: z.boolean(),
    settings_revision: revision,
    issue_count: count,
  }),
  committed_at: time,
});
export type Iteration = z.infer<typeof IterationSchema>;
export type IterationDraft = z.infer<typeof IterationDraftSchema>;
export type IterationPreview = z.infer<typeof IterationPreviewSchema>;
export type IterationWriteResult = z.infer<typeof IterationWriteResultSchema>;
export type IterationCreateInput = {
  request_id: string;
  name: string;
  description: string | null;
  coordinator_user_id: string | null;
  start_date: string;
  end_date: string;
  confirmed_timezone: string;
};
export type IterationWriteInput = {
  request_id: string;
  draft: IterationDraft;
  preview_hash: string;
};
export function parseIteration<T extends { workspace_id: string }>(
  raw: unknown,
  schema: z.ZodType<T>,
  wsId: string,
): T {
  const value = parseWithFallback<T | null>(raw, schema, null, {
    endpoint: "iterations",
    redact: true,
  });
  if (!value || value.workspace_id !== wsId)
    throw new Error("Invalid iteration response");
  return value;
}
