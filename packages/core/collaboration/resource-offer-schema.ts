import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { CollaborationResourceRuntimeSchema, CollaborationTargetResourceSchema, collaborationResourceRosterIsValid } from "./resource-schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
const resource = CollaborationTargetResourceSchema.extend({
  agent_name: z.string(), runtime: CollaborationResourceRuntimeSchema.nullable(), owner: CollaborationAccountSummarySchema.nullable(),
}).strip().refine((value) => (value.runtime === null || value.runtime.id === value.runtime_id) &&
  (value.owner === null || value.owner.user_id.toLowerCase() === value.owner_user_id));

export const CollaborationResourceOfferSchema = z.object({
  protocol_version: z.literal(1), issue_id: uuid, offer_id: uuid, version: positiveVersion,
  state: z.enum(["offered", "withdrawn", "adopted"]), offered_by: CollaborationAccountSummarySchema.nullable(),
  kind: z.enum(["agent", "squad"]), resource_id: uuid, execution_workspace_id: uuid, name: z.string(),
  resources: z.array(resource).min(1).max(256), created_at: timestamp, withdrawn_at: timestamp.nullable(), adopted_at: timestamp.nullable(),
  adopted_by: CollaborationAccountSummarySchema.nullable(), target_id: uuid.nullable(),
  available: z.boolean(), reason: z.string(), can_adopt: z.boolean(), can_withdraw: z.boolean(),
}).refine(collaborationResourceRosterIsValid).refine((value) =>
  value.resources.every((item) => item.execution_workspace_id === value.execution_workspace_id) &&
  (value.offered_by === null || value.resources.some((item) => item.owner_user_id === value.offered_by!.user_id.toLowerCase())) &&
  value.available === (value.reason === "") && (!value.can_adopt || value.available) &&
  (!value.available || value.offered_by !== null && value.resources.every((item) => item.owner !== null && item.runtime !== null)) &&
  (!value.can_withdraw || value.offered_by !== null),
).refine((value) => {
  switch (value.state) {
    case "offered": return value.withdrawn_at === null && value.adopted_at === null && value.adopted_by === null && value.target_id === null;
    case "withdrawn": return value.withdrawn_at !== null && value.adopted_at === null && value.adopted_by === null && value.target_id === null &&
      !value.available && !value.can_adopt && !value.can_withdraw;
    case "adopted": return value.withdrawn_at === null && value.adopted_at !== null && value.adopted_by !== null && value.target_id !== null &&
      !value.available && !value.can_adopt && !value.can_withdraw;
    default: return false;
  }
});
export type CollaborationResourceOffer = z.infer<typeof CollaborationResourceOfferSchema>;

export const CollaborationResourceOffersPageSchema = z.object({
  protocol_version: z.literal(1), issue_id: uuid, lifecycle_epoch: positiveVersion, target_set_version: positiveVersion,
  can_offer: z.boolean(), items: z.array(CollaborationResourceOfferSchema).max(100),
  next_cursor: z.string().min(1).max(512).regex(/^[A-Za-z0-9_-]+$/).nullable(),
}).refine((value) => value.items.every((item) => item.issue_id === value.issue_id) &&
  new Set(value.items.map((item) => item.offer_id)).size === value.items.length && (value.next_cursor === null || value.items.length > 0));
export type CollaborationResourceOffersPage = z.infer<typeof CollaborationResourceOffersPageSchema>;

export const parseCollaborationResourceOffer = (value: unknown): CollaborationResourceOffer | null =>
  parseWithFallback<CollaborationResourceOffer | null>(value, CollaborationResourceOfferSchema, null, { endpoint: "collaboration resource offer", redact: true });
export const parseCollaborationResourceOffersPage = (value: unknown): CollaborationResourceOffersPage | null =>
  parseWithFallback<CollaborationResourceOffersPage | null>(value, CollaborationResourceOffersPageSchema, null, { endpoint: "collaboration resource offers", redact: true });
