import { z } from "zod";
import { parseWithFallback } from "../api/schema";

export const observationInstant = z.iso.datetime({ offset: true });
export const observationCount = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
export const observationQuality = z.enum(["complete", "partial", "stale", "unavailable", "unknown"]).catch("unknown");
const optionalCount = observationCount.nullish().transform(value => value ?? null);
export const observationWindowSchema = z.object({
  time_from: observationInstant, time_to: observationInstant, timezone: z.string().min(1),
}).refine(value => Date.parse(value.time_from) < Date.parse(value.time_to))
  .transform(value => ({ timeFrom: value.time_from, timeTo: value.time_to, timezone: value.timezone }));
export type AdminObservationWindow = z.output<typeof observationWindowSchema>;
export const observationEnvelope = { scope: z.uuid(), as_of: observationInstant, data_quality: observationQuality };
export const observationPage = { ...observationEnvelope, next_cursor: z.string().nullable().default(null) };
const duration = z.object({ p50: z.number().nonnegative().nullable(), p95: z.number().nonnegative().nullable(), samples: observationCount });
const queueDuration = duration.extend({ lower_bound_samples: observationCount, unknown_samples: observationCount });
const overviewSchema = z.object({
  ...observationEnvelope, window: observationWindowSchema, rule_version: z.string(),
  installations: z.object({ total: optionalCount, retired: optionalCount, client_active: optionalCount, daemon_reachable: optionalCount, ready: optionalCount, unassociated: optionalCount }),
  executions: z.object({ completed: observationCount, failed: observationCount, cancelled: observationCount, unfinished: optionalCount, queued: optionalCount, running: optionalCount,
    dispatched: optionalCount, waiting_local_directory: optionalCount, deferred: optionalCount,
    success_rate: z.number().min(0).max(1).nullable(), queue_seconds: queueDuration, run_seconds: duration,
  }).refine(value => value.completed + value.failed !== 0 || value.success_rate === null),
  usage: z.object({ input_tokens: optionalCount, output_tokens: optionalCount, total_tokens: optionalCount, missing_tasks: observationCount, unpriced_tasks: observationCount,
    cache_read_tokens: optionalCount, cache_write_tokens: optionalCount, quality: observationQuality, billing: z.literal("tokens_only") }),
  alerts: z.object({ open: optionalCount, acknowledged: optionalCount, resolved: optionalCount, closed: optionalCount }),
}).transform(value => ({
  scope: value.scope, asOf: value.as_of, dataQuality: value.data_quality, window: value.window, ruleVersion: value.rule_version,
  installations: { total: value.installations.total, retired: value.installations.retired, clientActive: value.installations.client_active, daemonReachable: value.installations.daemon_reachable, ready: value.installations.ready, unassociated: value.installations.unassociated },
  executions: { completed: value.executions.completed, failed: value.executions.failed, cancelled: value.executions.cancelled, unfinished: value.executions.unfinished,
    queued: value.executions.queued, dispatched: value.executions.dispatched, running: value.executions.running, waitingLocalDirectory: value.executions.waiting_local_directory, deferred: value.executions.deferred,
    successRate: value.executions.success_rate, queueSeconds: { p50: value.executions.queue_seconds.p50, p95: value.executions.queue_seconds.p95, samples: value.executions.queue_seconds.samples, lowerBoundSamples: value.executions.queue_seconds.lower_bound_samples, unknownSamples: value.executions.queue_seconds.unknown_samples }, runSeconds: value.executions.run_seconds },
  usage: { inputTokens: value.usage.input_tokens, outputTokens: value.usage.output_tokens, cacheReadTokens: value.usage.cache_read_tokens, cacheWriteTokens: value.usage.cache_write_tokens, totalTokens: value.usage.total_tokens, missingTasks: value.usage.missing_tasks, unpricedTasks: value.usage.unpriced_tasks, quality: value.usage.quality, billing: value.usage.billing },
  alerts: value.alerts,
}));
export type AdminOverview = z.output<typeof overviewSchema>;
export const parseAdminOverview = (raw: unknown): AdminOverview | null => parseWithFallback(raw, overviewSchema, null, { endpoint: "GET /api/admin/overview", redact: true });

/** Pin defaults once per view; every drilldown uses the returned server window. */
export function observationParams(params: URLSearchParams, now: Date): URLSearchParams {
  const result = new URLSearchParams(params);
  if (!result.has("time_to")) result.set("time_to", now.toISOString());
  if (!result.has("time_from")) result.set("time_from", new Date(now.getTime() - 86_400_000).toISOString());
  if (!result.has("timezone")) result.set("timezone", "UTC");
  return result;
}
export function observationDrilldown(path: string, window: AdminObservationWindow, filters: Record<string, string> = {}): string {
  const params = new URLSearchParams({ ...filters, time_from: window.timeFrom, time_to: window.timeTo, timezone: window.timezone });
  return `${path}?${params}`;
}

export const adminHealthStates = ["healthy", "degraded", "unavailable", "stale", "unknown"] as const;
export const adminHealthSources = ["api", "database", "liveness", "task_coordinator", "cancellation_coordinator", "alert_detector", "unknown"] as const;
const healthState = z.enum(adminHealthStates).catch("unknown");
const healthSchema = z.object({ ...observationEnvelope, detector_state: healthState, sources: z.array(z.object({
  name: z.enum(adminHealthSources).catch("unknown"), state: healthState, checked_at: observationInstant.nullable(), code: z.string(),
}).transform(value => ({ name: value.name, state: value.state, checkedAt: value.checked_at, code: value.code }))) })
  .transform(value => ({ scope: value.scope, asOf: value.as_of, dataQuality: value.data_quality, detectorState: value.detector_state, sources: value.sources }));
const optionalBoolean = z.boolean().nullable();
const settingsSchema = z.object({ ...observationEnvelope,
  configuration: z.object({ auth_mode: z.string().nullable(), registration_enabled: optionalBoolean, registration_policy: z.string().nullable(), managed_installations_enabled: optionalBoolean, workspace_creation_enabled: optionalBoolean, read_only: z.literal(true), source: z.enum(["deployment", "unknown"]).catch("unknown") }),
  retention: z.object({ confirmed_operations_days: optionalCount, alerts_days: optionalCount, audit_days: optionalCount, automatic_deletion_enabled: z.literal(false), policy_source: z.enum(["configured", "unknown"]).catch("unknown") }),
  refresh_intervals_seconds: z.object({ list: observationCount, detail: observationCount }),
}).transform(value => ({ scope: value.scope, asOf: value.as_of, dataQuality: value.data_quality,
  configuration: { authMode: value.configuration.auth_mode, registrationEnabled: value.configuration.registration_enabled, registrationPolicy: value.configuration.registration_policy, managedInstallationsEnabled: value.configuration.managed_installations_enabled, workspaceCreationEnabled: value.configuration.workspace_creation_enabled, readOnly: value.configuration.read_only, source: value.configuration.source },
  retention: { confirmedOperationsDays: value.retention.confirmed_operations_days, alertsDays: value.retention.alerts_days, auditDays: value.retention.audit_days, automaticDeletionEnabled: value.retention.automatic_deletion_enabled, policySource: value.retention.policy_source },
  refreshIntervalsSeconds: value.refresh_intervals_seconds,
}));
const workspacesSchema = z.object({ ...observationPage, window: observationWindowSchema, items: z.array(z.object({ id: z.uuid(), name: z.string(), slug: z.string(), member_count: observationCount, execution_count: observationCount, created_at: observationInstant })
  .transform(value => ({ id: value.id, name: value.name, slug: value.slug, memberCount: value.member_count, executionCount: value.execution_count, createdAt: value.created_at }))) })
  .transform(value => ({ items: value.items, scope: value.scope, asOf: value.as_of, dataQuality: value.data_quality, nextCursor: value.next_cursor, window: value.window }));
const snapshotCode = z.string().regex(/^[a-z][a-z0-9_.:-]{0,127}$/).optional();
const snapshotVersion = z.union([z.string().regex(/^[0-9]{1,19}$/), observationCount.transform(String)]).optional();
const snapshotId = z.uuid().nullable().optional();
const auditSnapshot = z.object({
  role: z.enum(["super_admin", "platform_observer"]).nullable().optional(),
  disabled: z.boolean().optional(), access_revoked: z.boolean().optional(), requires_password_change: z.boolean().optional(), condition_active: z.boolean().optional(),
  auth_version: snapshotVersion, version: snapshotVersion.or(z.uuid()), admission_version: snapshotVersion, binding_epoch: snapshotVersion,
  state: snapshotCode, status: snapshotCode, admission: snapshotCode, result_code: snapshotCode, confirmation: snapshotCode, reconciliation_state: snapshotCode, rule: snapshotCode, severity: snapshotCode, resolution_code: snapshotCode.nullable(),
  assignee_id: snapshotId, assigned_to: snapshotId, responsible_user_id: snapshotId, related_task_id: snapshotId,
}).nullish().transform(value => value ?? {});
const auditSchema = z.object({ ...observationPage, items: z.array(z.object({
  id: z.uuid(), operation_id: z.uuid().nullable(), actor_kind: z.string(), actor_user_id: z.uuid().nullable(), actor_display_name: z.string().nullable(), actor_snapshot_quality: z.enum(["captured", "unknown"]).catch("unknown"),
  target_kind: z.string(), target_id: z.uuid(), action: z.string(), phase: z.string(), result_code: z.string(), request_id: z.string(), reason: z.string(), before_state: auditSnapshot, after_state: auditSnapshot, created_at: observationInstant,
}).transform(value => ({ id: value.id, operationId: value.operation_id, actorKind: value.actor_kind, actorUserId: value.actor_user_id, actorDisplayName: value.actor_display_name, actorSnapshotQuality: value.actor_snapshot_quality, targetKind: value.target_kind, targetId: value.target_id, action: value.action, phase: value.phase, resultCode: value.result_code, requestId: value.request_id, reason: value.reason, beforeState: value.before_state, afterState: value.after_state, createdAt: value.created_at }))) })
  .transform(value => ({ items: value.items, scope: value.scope, asOf: value.as_of, dataQuality: value.data_quality, nextCursor: value.next_cursor }));
export type AdminHealth = z.output<typeof healthSchema>;
export type AdminSettings = z.output<typeof settingsSchema>;
export type AdminWorkspaces = z.output<typeof workspacesSchema>;
export type AdminAudit = z.output<typeof auditSchema>;
export const parseAdminHealth = (raw: unknown): AdminHealth | null => parseWithFallback(raw, healthSchema, null, { endpoint: "GET /api/admin/health", redact: true });
export const parseAdminSettings = (raw: unknown): AdminSettings | null => parseWithFallback(raw, settingsSchema, null, { endpoint: "GET /api/admin/settings", redact: true });
export const parseAdminWorkspaces = (raw: unknown): AdminWorkspaces | null => parseWithFallback(raw, workspacesSchema, null, { endpoint: "GET /api/admin/workspaces", redact: true });
export const parseAdminAudit = (raw: unknown): AdminAudit | null => parseWithFallback(raw, auditSchema, null, { endpoint: "GET /api/admin/audit", redact: true });
