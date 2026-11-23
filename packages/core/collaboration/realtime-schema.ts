import { z } from "zod";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((id) => id.toLowerCase());
export const CollaborationChangeSchema = z.strictObject({ issue_id: uuid, event_seq: positiveVersion, collaboration_revision: positiveVersion, authority_epoch: positiveVersion });
export const CollaborationIssueHintSchema = z.strictObject({ issue_id: uuid });
export const CollaborationAccessHintSchema = z.union([CollaborationChangeSchema, CollaborationIssueHintSchema]);
export const CollaborationSourceAccessHintSchema = z.strictObject({ workspace_id: uuid });
