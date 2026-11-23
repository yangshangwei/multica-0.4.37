import { z } from "zod";
import { positiveVersion } from "./version";
const uuid = z.uuid().transform((value) => value.toLowerCase()).refine((value) => value !== "00000000-0000-0000-0000-000000000000");
export const CollaborationTestSubjectSchema = z.strictObject({ candidate_id: uuid, candidate_version: positiveVersion, candidate_digest: z.string().regex(/^[a-f0-9]{64}$/) });
export type CollaborationTestSubject = z.infer<typeof CollaborationTestSubjectSchema>;
export const sameCollaborationTestSubject = (a: CollaborationTestSubject, b: CollaborationTestSubject) => a.candidate_id === b.candidate_id && a.candidate_version === b.candidate_version && a.candidate_digest === b.candidate_digest;
