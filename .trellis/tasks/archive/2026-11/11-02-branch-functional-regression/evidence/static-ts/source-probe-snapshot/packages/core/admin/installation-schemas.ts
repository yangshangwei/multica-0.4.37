import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { adminVersionSchema } from "./control-schema";
export const installationStates = ["active", "inactive", "reachable", "unreachable", "ready", "stopped", "environment_unavailable", "no_permission", "unknown", "unavailable"] as const;
const instant = z.iso.datetime({ offset: true });
const optionalInstant = instant.nullish().transform(value => value ?? null);
const optionalId = z.uuid().nullish().transform(value => value ?? null);
const counter = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const axis = z.object({
  state: z.enum(installationStates).catch("unknown"), observed_at: optionalInstant, source: z.string().default("none"), freshness: z.enum(["fresh", "stale", "unknown", "unavailable"]).catch("unknown"), reason_code: z.string().default("unknown")
}).transform(value => ({
  state: value.state, observedAt: value.observed_at, source: value.source, freshness: value.freshness, reasonCode: value.reason_code
}));
function constrainedAxis(allowed: readonly (typeof installationStates[number])[]) {
  return axis.transform(value => allowed.includes(value.state) || value.state === "unknown" || value.state === "unavailable" ? value : { ...value, state: "unknown" as const });
}
const installation = z.object({
  id: z.uuid(), deployment_id: z.uuid(), organization_id: z.uuid(), lifecycle: z.enum(["active", "retired", "unknown"]).catch("unknown"), responsible_user_id: optionalId,
  display_name: z.string(), groups: z.array(z.string()), desktop_version: z.string().nullish(), os: z.string().nullish(), admission: z.enum(["accepting", "stopped", "unknown"]).catch("unknown"), admission_version: adminVersionSchema,
  allowed_actions: z.array(z.string()).nullish().transform(value => value ?? []),
  created_at: instant, updated_at: instant, runtime_count: counter, binding_count: counter, client_activity: constrainedAxis(["active", "inactive"]), daemon_reachability: constrainedAxis(["reachable", "unreachable"]), execution_readiness: constrainedAxis(["ready", "stopped", "environment_unavailable", "no_permission"]),
}).transform(value => ({
  id: value.id, deploymentId: value.deployment_id, organizationId: value.organization_id, lifecycle: value.lifecycle, responsibleUserId: value.responsible_user_id, displayName: value.display_name, groups: value.groups, desktopVersion: value.desktop_version ?? null, os: value.os ?? null, admission: value.admission, admissionVersion: value.admission_version, allowedActions: value.allowed_actions, createdAt: value.created_at, updatedAt: value.updated_at, runtimeCount: value.runtime_count, bindingCount: value.binding_count, clientActivity: value.client_activity, daemonReachability: value.daemon_reachability, executionReadiness: value.execution_readiness
}));
const runtime = z.object({
  id: z.uuid(), workspace_id: z.uuid(), binding_id: optionalId, principal_user_id: optionalId, provider: z.string(), status: z.string(), last_seen_at: optionalInstant, running_tasks: counter.nullish().transform(value => value ?? null)
}).transform(value => ({
  id: value.id, workspaceId: value.workspace_id, bindingId: value.binding_id, principalUserId: value.principal_user_id, provider: value.provider, status: value.status, lastSeenAt: value.last_seen_at, runningTasks: value.running_tasks
}));
const binding = z.object({
  id: z.uuid(), workspace_id: z.uuid(), principal_user_id: z.uuid(), binding_epoch: counter, state: z.enum(["active", "revoked", "unknown"]).catch("unknown"), capability_version: z.string(), authenticated_at: instant, last_seen_at: optionalInstant, revoked_at: optionalInstant
}).transform(value => ({
  id: value.id, workspaceId: value.workspace_id, principalUserId: value.principal_user_id, bindingEpoch: value.binding_epoch, state: value.state, capabilityVersion: value.capability_version, authenticatedAt: value.authenticated_at, lastSeenAt: value.last_seen_at, revokedAt: value.revoked_at
}));
const user = z.object({
  user_id: z.uuid(), first_seen_at: instant, last_seen_at: instant
}).transform(value => ({
  userId: value.user_id, firstSeenAt: value.first_seen_at, lastSeenAt: value.last_seen_at
}));
const page = {
  as_of: instant, scope: z.uuid(), next_cursor: z.string().nullable().default(null), data_quality: z.enum(["complete", "partial", "unavailable"]).catch("unavailable")
};
const list = z.object({ items: z.array(installation), ...page }).transform(value => ({
  items: value.items, asOf: value.as_of, scope: value.scope, nextCursor: value.next_cursor, dataQuality: value.data_quality
}));
const detail = z.object({
  installation, runtimes: z.array(runtime), bindings: z.array(binding), users: z.array(user), as_of: instant, scope: z.uuid(), details_truncated: z.boolean().default(false)
}).transform(value => ({
  installation: value.installation, runtimes: value.runtimes, bindings: value.bindings, users: value.users, asOf: value.as_of, scope: value.scope, detailsTruncated: value.details_truncated
}));
const unassociated = z.object({
  items: z.array(z.object({
    id: z.uuid(), workspace_id: z.uuid(), owner_id: optionalId, provider: z.string(), status: z.string(), last_seen_at: optionalInstant, created_at: instant, association: z.literal("unassociated")
  }).transform(value => ({
    id: value.id, workspaceId: value.workspace_id, ownerId: value.owner_id, provider: value.provider, status: value.status, lastSeenAt: value.last_seen_at, createdAt: value.created_at, association: value.association
  }))), ...page
}).transform(value => ({
  items: value.items, asOf: value.as_of, scope: value.scope, nextCursor: value.next_cursor, dataQuality: value.data_quality
}));
export type AdminInstallationAxis = z.output<typeof axis>;
export type AdminInstallation = z.output<typeof installation>;
export type AdminInstallationList = z.output<typeof list>;
export type AdminInstallationDetail = z.output<typeof detail>;
export type AdminUnassociatedRuntimes = z.output<typeof unassociated>;
export const parseAdminInstallationList = (raw: unknown): AdminInstallationList | null => parseWithFallback(raw, list, null, { endpoint: "GET /api/admin/installations", redact: true });
export const parseAdminInstallationDetail = (raw: unknown): AdminInstallationDetail | null => parseWithFallback(raw, detail, null, { endpoint: "GET /api/admin/installations/:id", redact: true });
export const parseAdminUnassociatedRuntimes = (raw: unknown): AdminUnassociatedRuntimes | null => parseWithFallback(raw, unassociated, null, { endpoint: "GET /api/admin/installations/unassociated", redact: true });
