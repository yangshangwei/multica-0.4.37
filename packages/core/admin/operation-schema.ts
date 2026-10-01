import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { adminVersionSchema, executionFenceSchema } from "./control-schema";

const optionalTime = z.iso.datetime({ offset: true }).nullish().transform(value => value ?? null);
export const adminOperationSchema = z.object({
  id: z.uuid(), organization_id: z.uuid(), target_id: z.uuid(),
  actor_id: z.uuid().nullish().transform(value => value ?? null),
  target_kind: z.string().nullish().transform(value => value ?? null),
  kind: z.string(),
  state: z.enum(["applied", "succeeded", "failed", "unknown"]).catch("unknown"),
  result_code: z.string(),
  confirmation: z.enum(["pending", "unconfirmed", "confirmed", "not_required", "unavailable", "unknown"]).catch("unknown"),
  reconciliation_state: z.enum(["pending", "unconfirmed", "complete", "unknown"]).catch("unknown"),
  root_operation_id: z.uuid().nullish().transform(value => value ?? null),
  version: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  accepted_at: optionalTime, updated_at: optionalTime, applied_at: optionalTime,
  confirmed_at: optionalTime, ack_deadline: optionalTime,
}).transform(value => ({
  id: value.id, organizationId: value.organization_id, targetId: value.target_id,
  actorId: value.actor_id, targetKind: value.target_kind, kind: value.kind,
  state: value.state, resultCode: value.result_code, confirmation: value.confirmation,
  reconciliationState: value.reconciliation_state, rootOperationId: value.root_operation_id,
  version: value.version, acceptedAt: value.accepted_at, updatedAt: value.updated_at,
  appliedAt: value.applied_at, confirmedAt: value.confirmed_at, ackDeadline: value.ack_deadline,
}));
export const adminOperationsSchema = z.object({
  items: z.array(adminOperationSchema).max(1), scope: z.uuid(), as_of: z.string(),
}).transform(value => ({ items: value.items, scope: value.scope, asOf: value.as_of }));
const admissionResultSchema = z.object({
  operation: adminOperationSchema,
  target: z.object({ id: z.uuid(), admission: z.enum(["accepting", "stopped"]), admission_version: adminVersionSchema })
    .transform(value => ({ id: value.id, admission: value.admission, admissionVersion: value.admission_version })),
});
const cancellationResultSchema = z.object({
  operation: adminOperationSchema,
  target: z.object({ id: z.uuid(), status: z.string(), state_version: adminVersionSchema, execution_fence: executionFenceSchema })
    .transform(value => ({ id: value.id, status: value.status, stateVersion: value.state_version, executionFence: value.execution_fence })),
});
export type AdminOperation = z.output<typeof adminOperationSchema>;
export type AdminOperations = z.output<typeof adminOperationsSchema>;
export type AdminAdmissionResult = z.output<typeof admissionResultSchema>;
export type AdminCancellationResult = z.output<typeof cancellationResultSchema>;
export const parseAdminOperation = (raw: unknown): AdminOperation | null => parseWithFallback(raw, adminOperationSchema, null, { endpoint: "GET /api/admin/operations/:id", redact: true });
export const parseAdminAdmissionResult = (raw: unknown): AdminAdmissionResult | null => parseWithFallback(raw, admissionResultSchema, null, { endpoint: "POST /api/admin/installations/:id/admission", redact: true });
export const parseAdminCancellationResult = (raw: unknown): AdminCancellationResult | null => parseWithFallback(raw, cancellationResultSchema, null, { endpoint: "POST /api/admin/tasks/:id/cancel", redact: true });

export function operationNeedsPolling(operation: AdminOperation, elapsed: number): boolean {
  return elapsed < 30_000 && operation.state === "applied" && operation.confirmation === "pending" && operation.reconciliationState === "pending";
}
