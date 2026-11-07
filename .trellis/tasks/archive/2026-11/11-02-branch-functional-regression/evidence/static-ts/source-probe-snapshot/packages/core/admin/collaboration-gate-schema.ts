import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { adminCollaborationGateActions, adminVersionSchema } from "./control-schema";
import { adminOperationSchema } from "./operation-schema";

// The switches stay strict: guessing a closed or open gate misstates a
// server-wide policy, so a malformed state fails the read instead.
const gateSchema = z.object({
  scope: z.uuid(), as_of: z.string(),
  creation_enabled: z.boolean(), execution_enabled: z.boolean(), host_execution_enabled: z.boolean(),
  execution_epoch: adminVersionSchema,
  // Unknown actions from a newer server are dropped, never rendered as controls.
  allowed_actions: z.array(z.string()).catch([]).transform(actions => adminCollaborationGateActions.filter(action => actions.includes(action))),
}).transform(value => ({
  scope: value.scope, asOf: value.as_of, creationEnabled: value.creation_enabled, executionEnabled: value.execution_enabled,
  hostExecutionEnabled: value.host_execution_enabled, executionEpoch: value.execution_epoch, allowedActions: value.allowed_actions,
}));
const gateResultSchema = z.object({ operation: adminOperationSchema, target: gateSchema })
  .transform(value => ({ operation: value.operation, target: { ...value.target, id: value.target.scope } }));
export type AdminCollaborationGate = z.output<typeof gateSchema>;
export type AdminCollaborationGateResult = z.output<typeof gateResultSchema>;
export const parseAdminCollaborationGate = (raw: unknown): AdminCollaborationGate | null => parseWithFallback(raw, gateSchema, null, { endpoint: "GET /api/admin/collaboration/gate", redact: true });
export const parseAdminCollaborationGateResult = (raw: unknown): AdminCollaborationGateResult | null => parseWithFallback(raw, gateResultSchema, null, { endpoint: "POST /api/admin/collaboration/gate", redact: true });
