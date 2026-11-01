import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { nonnegativeVersion, positiveVersion } from "./version";
import { collaborationTextByteLength } from "./text-limits";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const digest = z.string().regex(/^[a-f0-9]{64}$/);
const HANDOFF_MAX_MATERIAL_BYTES = 256 * 1024;
export const COLLABORATION_HANDOFF_MAX_JSON_BYTES = 2 * 1024 * 1024;

/** Match encoding/json.Marshal, including HTML and line-separator escaping. */
function handoffJSONBytes(value: unknown): number {
  return collaborationTextByteLength(JSON.stringify(value).replace(/[<>&\u2028\u2029]/g,
    (character) => `\\u${character.charCodeAt(0).toString(16).padStart(4, "0")}`));
}
const role = z.enum(["agent", "leader", "member", "continuation"]);
const cursor = z.string().min(1).max(2048);
const pageFields = { total: z.number().int().nonnegative().safe(), returned: z.number().int().min(0).max(32), truncated: z.boolean(), next_cursor: cursor.optional() };
const pageValid = (page: { total: number; returned: number; truncated: boolean; next_cursor?: string; items: unknown[] }) =>
  page.returned === page.items.length && page.total >= page.returned && page.truncated === (page.next_cursor !== undefined);
const transfer = z.object({ id: uuid, author_id: uuid, from_id: uuid, to_id: uuid, purpose: z.string().min(1), requester_id: uuid.optional(), event_seq: nonnegativeVersion, source_id: uuid })
  .refine((value) => value.source_id === value.id);
const source = z.object({
  kind: z.enum(["handoff", "delegation", "provider_result", "squad_activity"]), id: uuid, run_id: uuid.optional(), version: nonnegativeVersion.optional(),
  sha256: digest, content: z.string(), truncated: z.boolean(), availability: z.enum(["available", "missing", "purged"]), trust: z.enum(["data_only", "admitted_assignment"]),
}).refine((value) => (value.trust === "data_only" || value.kind === "delegation") &&
  (value.availability === "available" || value.content === "") && !value.content.includes("\0"));

const handoffEnvelope = z.object({
  protocol_version: z.literal(1), issue_id: uuid, run_id: uuid, snapshot_id: uuid.optional(), mode: z.enum(["live", "snapshot"]), event_seq: nonnegativeVersion,
  authority: z.object({ status: z.string().min(1), handler_id: uuid.optional(), creator_id: uuid.optional() }),
  assignment: z.object({ role, step_key: z.string(), parent_run_id: uuid.optional(), delegation_id: uuid.optional(), source_id: uuid.optional() }),
  handoff: transfer.nullable(), information_request: transfer.nullable(),
  // A live read's `version` is the latest; `base_version` is the version this
  // run actually started from, when it has read one.
  memory: z.object({ version: nonnegativeVersion, sha256: digest.optional(), availability: z.enum(["available", "absent", "unsupported", "purged"]), base_version: nonnegativeVersion.optional() })
    .refine((value) => value.availability !== "available" || value.sha256 !== undefined),
  inputs: z.object({ ...pageFields, items: z.array(z.object({ command_id: uuid, author_id: uuid, state: z.string(),
    reader_run_ids: z.array(uuid).max(32).refine((ids) => new Set(ids).size === ids.length), readers_truncated: z.boolean(), readers_cursor: cursor.optional(),
  }).refine((value) => !value.readers_truncated || value.readers_cursor !== undefined)).max(32) }).refine(pageValid)
    .refine((value) => new Set(value.items.map((item) => item.command_id)).size === value.items.length),
  results: z.object({ ...pageFields, items: z.array(z.object({ run_id: uuid, parent_run_id: uuid.optional(), role, step_key: z.string(), leader_turn: positiveVersion.optional(),
    progress: z.string(), outcome: z.string().optional(), proof_state: z.string(), recording_incomplete: z.boolean(),
    result_state: z.enum(["present", "absent"]), final_answer_state: z.enum(["unknown", "absent"]), source_id: uuid.optional(), source_kind: z.string().optional(),
  })).max(32) }).refine(pageValid).refine((value) => new Set(value.items.map((item) => item.run_id)).size === value.items.length),
  wakeup_causes: z.object({ ...pageFields, completeness: z.enum(["recorded", "unknown"]),
    items: z.array(z.object({ id: uuid, source_kind: z.enum(["comment", "input", "proof"]), source_id: uuid, source_run_id: uuid, sequence: positiveVersion })).max(32),
  }).refine(pageValid).refine((value) => new Set(value.items.map((item) => item.id)).size === value.items.length),
  sources: z.array(source), missing: z.array(z.string().min(1)),
}).refine((value) => (value.mode !== "snapshot" || value.snapshot_id !== undefined) &&
  new Set(value.sources.map((item) => `${item.kind}:${item.id}`)).size === value.sources.length);

export const CollaborationRunHandoffSchema = z.unknown().transform((raw, context) => {
  const parsed = handoffEnvelope.safeParse(raw);
  if (!parsed.success) {
    context.addIssue({ code: "custom", message: "Invalid handoff envelope" });
    return z.NEVER;
  }
  // Size the original envelope before normalization/unknown-field stripping.
  // Charge all metadata and JSON framing, including the HTTP newline; only
  // source content gets the protocol's decoded UTF-8 material accounting.
  const wire = handoffJSONBytes(raw) + 1;
  const material = parsed.data.sources.reduce((bytes, source) =>
    bytes - (handoffJSONBytes(source.content) - 2 - collaborationTextByteLength(source.content)), wire);
  if (wire > COLLABORATION_HANDOFF_MAX_JSON_BYTES || material > HANDOFF_MAX_MATERIAL_BYTES) {
    context.addIssue({ code: "custom", message: "Handoff exceeds its byte budget" });
    return z.NEVER;
  }
  return parsed.data;
});

export type CollaborationRunHandoff = z.infer<typeof CollaborationRunHandoffSchema>;
export function parseCollaborationRunHandoff(raw: unknown): CollaborationRunHandoff | null {
  return parseWithFallback<CollaborationRunHandoff | null>(raw, CollaborationRunHandoffSchema, null, { endpoint: "collaboration run handoff", redact: true });
}
