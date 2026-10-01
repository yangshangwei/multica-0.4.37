import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { adminVersionSchema } from "./control-schema";
import { adminOperationSchema } from "./operation-schema";
import { observationCount, observationInstant, observationPage } from "./observability-schema";

export const adminAlertRules = ["installation_unreachable", "queue_timeout", "execution_failed", "unknown"] as const;
export const adminAlertStatuses = ["open", "acknowledged", "resolved", "closed", "unknown"] as const;
export const adminAlertActions = ["acknowledge", "assign", "close"] as const;
export const adminAlertResolutions = ["handled", "no_action_needed", "retry_succeeded"] as const;
const resolutionCodes = [...adminAlertResolutions, "queue_left", "queue_below_threshold", "execution_finished", "daemon_reachable", "task_removed"] as const;
export type AdminAlertAction = typeof adminAlertActions[number];
export interface AdminAlertChange {
  action: AdminAlertAction;
  expectedVersion: string;
  reason: string;
  assigneeId?: string | null;
  resolutionCode?: typeof adminAlertResolutions[number];
  relatedTaskId?: string | null;
}
const changeBase = { expectedVersion: adminVersionSchema, reason: z.string().trim().min(1).max(1000) };
export const adminAlertChangeSchema = z.discriminatedUnion("action", [
  z.object({ ...changeBase, action: z.literal("acknowledge") }),
  z.object({ ...changeBase, action: z.literal("assign"), assigneeId: z.uuid().nullable() }),
  z.object({ ...changeBase, action: z.literal("close"), resolutionCode: z.enum(adminAlertResolutions).optional(), relatedTaskId: z.uuid().nullable().optional() })
    .refine(value => value.resolutionCode === "retry_succeeded" ? !!value.relatedTaskId : !value.relatedTaskId),
]);
const optionalTime = observationInstant.nullish().transform(value => value ?? null);
const optionalId = z.uuid().nullish().transform(value => value ?? null);
export const adminAlertSchema = z.object({
  id: z.uuid(), organization_id: z.uuid(), rule: z.enum(adminAlertRules).catch("unknown"), severity: z.enum(["warning", "critical", "unknown"]).catch("unknown"),
  status: z.enum(adminAlertStatuses).catch("unknown"), subject_kind: z.enum(["installation", "task", "unknown"]).catch("unknown"), subject_id: z.uuid(),
  first_seen_at: observationInstant, last_seen_at: observationInstant, occurrence_count: observationCount, assignee_id: optionalId,
  acknowledged_at: optionalTime, resolved_at: optionalTime, closed_at: optionalTime, version: adminVersionSchema,
  resolution_code: z.enum(resolutionCodes).nullish().catch(null).transform(value => value ?? null), related_task_id: optionalId,
  condition_active: z.boolean().nullish().transform(value => value ?? null), allowed_actions: z.array(z.string()).nullish().transform(value => value ?? []),
}).transform(value => ({
  id: value.id, organizationId: value.organization_id, rule: value.rule, severity: value.severity, status: value.status, subjectKind: value.subject_kind, subjectId: value.subject_id,
  firstSeenAt: value.first_seen_at, lastSeenAt: value.last_seen_at, occurrenceCount: value.occurrence_count, assigneeId: value.assignee_id,
  acknowledgedAt: value.acknowledged_at, resolvedAt: value.resolved_at, closedAt: value.closed_at, version: value.version, resolutionCode: value.resolution_code,
  relatedTaskId: value.related_task_id, conditionActive: value.condition_active, allowedActions: value.allowed_actions,
}));
const list = z.object({ ...observationPage, items: z.array(adminAlertSchema) }).transform(value => ({ items: value.items, scope: value.scope, asOf: value.as_of, dataQuality: value.data_quality, nextCursor: value.next_cursor }));
const result = z.object({ operation: adminOperationSchema, target: adminAlertSchema });
export type AdminAlert = z.output<typeof adminAlertSchema>;
export type AdminAlerts = z.output<typeof list>;
export type AdminAlertResult = z.output<typeof result>;
export const parseAdminAlert = (raw: unknown): AdminAlert | null => parseWithFallback(raw, adminAlertSchema, null, { endpoint: "GET /api/admin/alerts/:id", redact: true });
export const parseAdminAlerts = (raw: unknown): AdminAlerts | null => parseWithFallback(raw, list, null, { endpoint: "GET /api/admin/alerts", redact: true });
export const parseAdminAlertResult = (raw: unknown): AdminAlertResult | null => parseWithFallback(raw, result, null, { endpoint: "POST /api/admin/alerts/:id/action", redact: true });
export function canControlAlert(role: string | undefined, alert: AdminAlert, action: AdminAlertAction): boolean {
  if (role !== "super_admin" || alert.rule === "unknown" || alert.status === "unknown" || alert.status === "closed" || !alert.allowedActions.includes(action)) return false;
  if (action === "acknowledge") return alert.status === "open";
  if (action === "close") return alert.rule === "execution_failed" || alert.status === "resolved" && alert.conditionActive === false;
  return true;
}
