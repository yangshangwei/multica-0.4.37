import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
export const CollaborationTargetResourceSchema = z.strictObject({
  role: z.enum(["agent", "leader", "member"]), agent_id: uuid, runtime_id: uuid,
  execution_workspace_id: uuid, owner_user_id: uuid,
});
export const CollaborationResourceIdentitySchema = z.strictObject({
  kind: z.enum(["managed", "enrolled"]), installation_id: uuid, binding_id: uuid, binding_epoch: positiveVersion,
});
export type CollaborationTargetResource = z.infer<typeof CollaborationTargetResourceSchema>;
export const CollaborationResourceRuntimeSchema = z.object({ id: uuid, name: z.string(), custom_name: z.string().nullable(), provider: z.string() });

const resourceView = CollaborationTargetResourceSchema.extend({
  agent_name: z.string(), runtime: CollaborationResourceRuntimeSchema.nullable(),
  owner: CollaborationAccountSummarySchema.nullable(), consented: z.boolean(), grant_id: uuid.nullable(), grant_epoch: positiveVersion.nullable(),
  can_consent: z.boolean(), can_revoke: z.boolean(),
}).strip().refine((value) => (value.grant_id === null) === (value.grant_epoch === null) &&
  (!value.can_revoke || value.grant_id !== null) && !(value.can_consent && (value.consented || value.can_revoke)) &&
  (value.runtime === null || value.runtime.id === value.runtime_id) && (value.owner === null || value.owner.user_id.toLowerCase() === value.owner_user_id));
export type CollaborationResourceView = z.infer<typeof resourceView>;

export function collaborationResourceRosterIsValid(value: { kind: "agent" | "squad"; resource_id: string; resources: CollaborationTargetResource[] }) {
  const roster = value.resources;
  if (new Set(roster.map((item) => item.agent_id)).size !== roster.length) return false;
  return value.kind === "agent"
    ? roster.length === 1 && roster[0]!.role === "agent" && roster[0]!.agent_id === value.resource_id
    : roster.filter((item) => item.role === "leader").length === 1 && roster.every((item) => item.role !== "agent");
}
const targetResources = z.object({
  target_id: uuid, kind: z.enum(["agent", "squad"]), resource_id: uuid, name: z.string(), target_epoch: positiveVersion, config_version: positiveVersion,
  authorized: z.boolean(), reason: z.string(), resources: z.array(resourceView).min(1).max(256),
}).refine(collaborationResourceRosterIsValid).refine((value) => !value.authorized || (value.reason === "" && value.resources.every((item) => item.consented)));
export const CollaborationResourcesSchema = z.object({
  protocol_version: z.literal(1), issue_id: uuid, lifecycle_epoch: positiveVersion, target_set_version: positiveVersion, can_organize: z.boolean(),
  targets: z.array(targetResources),
}).refine((value) => new Set(value.targets.map((item) => item.target_id)).size === value.targets.length &&
  new Set(value.targets.map((item) => `${item.kind}:${item.resource_id}`)).size === value.targets.length);
export type CollaborationResources = z.infer<typeof CollaborationResourcesSchema>;

const option = z.object({ kind: z.enum(["agent", "squad"]), resource_id: uuid, name: z.string(), available: z.boolean(), reason: z.string(),
  resources: z.array(resourceView).min(1).max(256),
}).refine(collaborationResourceRosterIsValid).refine((value) => value.available === (value.reason === ""));
export const CollaborationResourceOptionsSchema = z.object({ protocol_version: z.literal(1), issue_id: uuid, execution_workspace_id: uuid, items: z.array(option) })
  .refine((value) => value.items.every((item) => item.resources.every((resource) => resource.execution_workspace_id === value.execution_workspace_id)) &&
    new Set(value.items.map((item) => `${item.kind}:${item.resource_id}`)).size === value.items.length);
export type CollaborationResourceOptions = z.infer<typeof CollaborationResourceOptionsSchema>;
export type CollaborationResourceOption = z.infer<typeof option>;
export const CollaborationResourceIdentityViewSchema = z.object({ issue_id: uuid, target_id: uuid, agent_id: uuid, proposal_id: uuid.nullable(),
  identity: CollaborationResourceIdentitySchema.nullable(), reason: z.string(),
}).refine((value) => (value.identity !== null) === (value.reason === ""));
export type CollaborationResourceIdentityView = z.infer<typeof CollaborationResourceIdentityViewSchema>;

const authority = { expected_lifecycle_epoch: positiveVersion, expected_participant_epoch: positiveVersion, expected_account_epoch: positiveVersion };
export const CollaborationResourceInputSchema = z.discriminatedUnion("action", [
  z.strictObject({ ...authority, action: z.literal("register_target"), kind: z.enum(["agent", "squad"]), resource_id: uuid,
    expected_target_set_version: positiveVersion, roster: z.array(CollaborationTargetResourceSchema).min(1).max(256) })
    .refine((value) => collaborationResourceRosterIsValid({ ...value, resources: value.roster })),
  z.strictObject({ ...authority, action: z.literal("consent_resource"), target_id: uuid, agent_id: uuid,
    expected_target_epoch: positiveVersion, expected_config_version: positiveVersion, identity: CollaborationResourceIdentitySchema }),
  z.strictObject({ ...authority, action: z.literal("revoke_grant"), grant_id: uuid, expected_grant_epoch: positiveVersion }),
]);
export type CollaborationResourceInput = z.infer<typeof CollaborationResourceInputSchema>;

const action = z.enum(["register_target", "consent_resource", "revoke_grant"]);
const failure = z.enum(["not_found", "forbidden", "stale_authority", "stale_scope", "stale_grant", "stale_target", "target_exists", "target_not_found",
  "issue_closed", "owner_not_participant", "resource_owner_not_participant", "resource_owner_unavailable", "target_configuration_changed", "runtime_registration_required", "invalid_target_configuration"]);
const code = z.union([z.enum(["target_registered", "resource_consented", "grant_revoked", "grant_already_revoked"]), failure]);
const receiptBase = { protocol_version: z.literal(2), issue_id: uuid, request_id: z.string().min(1).max(128), action, status: z.number().int().min(200).max(599), code };
function validOutcome(value: { action: z.infer<typeof action>; code: string; status: number }) {
  if (failure.safeParse(value.code).success) return value.status >= 400;
  if (value.status !== 200 && value.status !== 201) return false;
  return value.action === "register_target" ? value.code === "target_registered" : value.action === "consent_resource" ? value.code === "resource_consented"
    : value.code === "grant_revoked" || value.code === "grant_already_revoked";
}
const fullReceipt = z.object({ ...receiptBase, redacted: z.literal(false).optional(), scope: z.enum(["issue", "target"]), target_id: uuid.nullable(), grant_id: uuid.nullable() })
  .refine(validOutcome).refine((value) => value.status >= 400 || value.action === "revoke_grant" ||
    (value.scope === "target" && value.target_id !== null && (value.action !== "consent_resource" || value.grant_id !== null)));
const redactedReceipt = z.strictObject({ ...receiptBase, redacted: z.literal(true) }).refine(validOutcome);
export const CollaborationResourceReceiptSchema = z.union([redactedReceipt, fullReceipt]);
export type CollaborationResourceReceipt = z.infer<typeof CollaborationResourceReceiptSchema>;

const parse = <T>(value: unknown, schema: z.ZodType<T>, endpoint: string): T | null => parseWithFallback<T | null>(value, schema, null, { endpoint, redact: true });
export const parseCollaborationResources = (value: unknown) => parse(value, CollaborationResourcesSchema, "collaboration resources");
export const parseCollaborationResourceOptions = (value: unknown) => parse(value, CollaborationResourceOptionsSchema, "collaboration resource options");
export const parseCollaborationResourceIdentity = (value: unknown) => parse(value, CollaborationResourceIdentityViewSchema, "collaboration resource identity");
export const parseCollaborationResourceReceipt = (value: unknown) => parse(value, CollaborationResourceReceiptSchema, "collaboration resource receipt");

export function matchesCollaborationResourceReceipt(receipt: CollaborationResourceReceipt, issueId: string, requestId: string, input: CollaborationResourceInput) {
  return receipt.issue_id === issueId.toLowerCase() && receipt.request_id === requestId && receipt.action === input.action &&
    (receipt.redacted === true || input.action !== "consent_resource" || receipt.target_id === input.target_id || receipt.status >= 400 && receipt.target_id === null);
}
export function collaborationResourceRequest(issueId: string, input: CollaborationResourceInput) {
  const root = `/api/collaboration/issues/${encodeURIComponent(issueId)}`;
  const { action: operation, ...data } = input;
  switch (operation) {
    case "register_target": return { path: `${root}/targets`, body: data };
    case "consent_resource": {
      const { target_id, action: _action, ...body } = input;
      return { path: `${root}/targets/${encodeURIComponent(target_id)}/consents`, body };
    }
    case "revoke_grant": {
      const { grant_id, action: _action, ...body } = input;
      return { path: `${root}/grants/${encodeURIComponent(grant_id)}/revoke`, body };
    }
    default: throw new TypeError("Invalid collaboration resource intent");
  }
}
