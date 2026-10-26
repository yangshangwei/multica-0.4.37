import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { collaborationTextByteLength } from "./text-limits";
import { nonnegativeVersion, positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
const nullableVersion = positiveVersion.nullish().transform((value) => value ?? null);
const byteCount = z.number().int().nonnegative().safe();
/** Hard ceiling of the stored document; the deployment limit is always at most this. */
export const COLLABORATION_MEMORY_HARD_MAX_BYTES = 524_288;
const maxBytes = z.number().int().positive().max(COLLABORATION_MEMORY_HARD_MAX_BYTES);
const account = CollaborationAccountSummarySchema.nullish().transform((value) => value ?? null);

/** Server ordering compares exact decimal strings, never JavaScript numbers. */
function versionNumber(value: string): bigint | null {
  return nonnegativeVersion.safeParse(value).success ? BigInt(value) : null;
}
export function compareCollaborationMemoryVersions(left: string, right: string): number {
  const a = versionNumber(left);
  const b = versionNumber(right);
  if (a === null || b === null) return Number.NaN;
  return a === b ? 0 : a < b ? -1 : 1;
}
export function nextCollaborationMemoryVersion(value: string): string {
  const current = versionNumber(value);
  if (current === null) throw new RangeError("Invalid shared-context version");
  return String(current + 1n);
}

// Account and agent lookups can miss after deletion; the author's kind stays a
// plain string so a newer server kind renders as an unknown writer.
export const CollaborationMemoryAuthorSchema = z.object({
  kind: z.string().min(1),
  member: account,
  agent: z.object({
    agent_id: uuid.nullable(), name: z.string().catch(""), run_id: uuid.nullable(),
  }).nullish().transform((value) => value ?? null),
  resource_owner: account,
});

export const CollaborationMemorySchema = z.object({
  issue_id: uuid, version: nonnegativeVersion, content: z.string(),
  bytes: byteCount, max_bytes: maxBytes,
  author: CollaborationMemoryAuthorSchema.nullable(), created_at: timestamp.nullable(),
  restored_from_version: nullableVersion,
  can_edit: z.boolean().catch(false), can_purge: z.boolean().catch(false),
}).refine((value) => value.version === "0"
  ? value.content === "" && value.bytes === 0 && value.author === null && value.created_at === null && value.restored_from_version === null
  : value.author !== null && value.created_at !== null, "An empty document has no writer; every stored version has one");

export const CollaborationMemoryVersionMetaSchema = z.object({
  version: positiveVersion, bytes: byteCount, author: CollaborationMemoryAuthorSchema.nullable(), created_at: timestamp,
  restored_from_version: nullableVersion,
  purged: z.boolean(), purged_at: timestamp.nullish().transform((value) => value ?? null),
  purged_by: account, purged_by_admin: z.boolean().catch(false),
}).refine((value) => value.purged === (value.purged_at !== null), "Purge metadata must match the purge marker");

export const CollaborationMemoryVersionsSchema = z.object({
  issue_id: uuid, current_version: nonnegativeVersion, max_bytes: maxBytes, can_purge: z.boolean().catch(false),
  items: z.array(CollaborationMemoryVersionMetaSchema).max(100),
  next_cursor: nullableVersion,
}).refine((page) => page.items.every((item, index) => {
  const previous = page.items[index - 1];
  return compareCollaborationMemoryVersions(item.version, page.current_version) <= 0 &&
    (!previous || compareCollaborationMemoryVersions(item.version, previous.version) < 0);
}) && (page.next_cursor === null || page.next_cursor === page.items.at(-1)?.version), "History is a strictly descending version page");

export const CollaborationMemoryVersionDetailSchema = CollaborationMemoryVersionMetaSchema.safeExtend({
  issue_id: uuid, current_version: nonnegativeVersion, max_bytes: maxBytes, content: z.string().nullable(),
}).refine((value) => value.purged === (value.content === null) &&
  compareCollaborationMemoryVersions(value.version, value.current_version) <= 0, "Purged text is never returned");

export const COLLABORATION_MEMORY_ACTIONS = ["memory_update", "memory_restore", "memory_purge"] as const;
export type CollaborationMemoryActionName = (typeof COLLABORATION_MEMORY_ACTIONS)[number];
// Known sealed outcomes: [HTTP status, actions that may produce it]. A code
// outside this table is not a confirmed result, so its request stays pending.
const receiptOutcomes: Record<string, [number, readonly CollaborationMemoryActionName[]]> = {
  memory_updated: [200, ["memory_update"]],
  memory_restored: [200, ["memory_restore"]],
  memory_purged: [200, ["memory_purge"]],
  stale_memory_version: [409, ["memory_update", "memory_restore"]],
  memory_too_large: [413, ["memory_update", "memory_restore"]],
  invalid_memory_content: [400, ["memory_update", "memory_restore"]],
  issue_closed: [409, ["memory_update", "memory_restore"]],
  status_unavailable: [409, ["memory_update", "memory_restore"]],
  memory_version_not_found: [404, ["memory_restore", "memory_purge"]],
  memory_version_purged: [409, ["memory_restore", "memory_purge"]],
  memory_version_current: [409, ["memory_purge"]],
};

export const CollaborationMemoryReceiptSchema = z.object({
  request_id: z.string().min(1).max(128), action: z.enum(COLLABORATION_MEMORY_ACTIONS),
  status: z.number().int(), code: z.string().min(1), protocol_version: z.literal(1), issue_id: uuid,
  version: nullableVersion, current_version: nonnegativeVersion,
  bytes: byteCount.optional(), max_bytes: maxBytes, event_seq: positiveVersion.optional(),
}).refine((value) => {
  const outcome = Object.hasOwn(receiptOutcomes, value.code) ? receiptOutcomes[value.code] : undefined;
  if (!outcome || outcome[0] !== value.status || !outcome[1].includes(value.action)) return false;
  if (value.status < 400) {
    if (value.version === null || value.event_seq === undefined || value.bytes === undefined) return false;
    const order = compareCollaborationMemoryVersions(value.version, value.current_version);
    // A purge names the historical version it cleared; writes name the new current version.
    return value.action === "memory_purge" ? order < 0 : order === 0;
  }
  return value.version === null && value.event_seq === undefined &&
    (value.code === "memory_too_large" ? value.bytes !== undefined && value.bytes > value.max_bytes : value.bytes === undefined);
}, "Shared-context receipts must use a known outcome for their action");

export type CollaborationMemoryAuthor = z.infer<typeof CollaborationMemoryAuthorSchema>;
export type CollaborationMemory = z.infer<typeof CollaborationMemorySchema>;
export type CollaborationMemoryVersionMeta = z.infer<typeof CollaborationMemoryVersionMetaSchema>;
export type CollaborationMemoryVersions = z.infer<typeof CollaborationMemoryVersionsSchema>;
export type CollaborationMemoryVersionDetail = z.infer<typeof CollaborationMemoryVersionDetailSchema>;
export type CollaborationMemoryReceipt = z.infer<typeof CollaborationMemoryReceiptSchema>;

/** The stored document must stay valid UTF-8 text without NUL, within the hard ceiling. */
export function collaborationMemoryContentValid(content: string): boolean {
  return !content.includes("\0") && collaborationTextByteLength(content) <= COLLABORATION_MEMORY_HARD_MAX_BYTES;
}
export function collaborationMemoryReasonValid(reason: string): boolean {
  const trimmed = reason.trim();
  return trimmed !== "" && !trimmed.includes("\0") && collaborationTextByteLength(trimmed) <= 1000;
}

function parse<T>(data: unknown, schema: z.ZodType<T>, endpoint: string): T | null {
  return parseWithFallback<T | null>(data, schema, null, { endpoint, redact: true });
}
export const parseCollaborationMemory = (data: unknown) => parse(data, CollaborationMemorySchema, "collaboration shared context");
export const parseCollaborationMemoryVersions = (data: unknown) => parse(data, CollaborationMemoryVersionsSchema, "collaboration shared context versions");
export const parseCollaborationMemoryVersion = (data: unknown) => parse(data, CollaborationMemoryVersionDetailSchema, "collaboration shared context version");
export const parseCollaborationMemoryReceipt = (data: unknown) => parse(data, CollaborationMemoryReceiptSchema, "collaboration shared context receipt");
