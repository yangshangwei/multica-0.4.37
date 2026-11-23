import { z } from "zod";
import { parseWithFallback } from "../api/schema";

export const adminResourceKindSchema = z.enum(["skill", "mcp"]);
export type AdminResourceKind = z.infer<typeof adminResourceKindSchema>;
const count = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const resourceSchema = z.object({
  kind: adminResourceKindSchema, key: z.string().regex(/^[A-Za-z0-9_-]+$/), name: z.string(), description: z.string().catch(""),
  source: z.enum(["managed", "deployment", "builtin", "unknown"]).catch("unknown"),
  state: z.enum(["published", "withdrawn", "unknown"]).catch("unknown"),
  version: z.string(), content_digest: z.string().nullable(), file_count: count, byte_count: count,
  updated_at: z.iso.datetime({ offset: true }).nullable(), updated_by: z.string().nullable(),
}).transform(value => ({
  kind: value.kind, key: value.key, name: value.name, description: value.description,
  source: value.source, state: value.state, version: value.version, contentDigest: value.content_digest,
  fileCount: value.file_count, byteCount: value.byte_count, updatedAt: value.updated_at, updatedBy: value.updated_by,
}));
const limitsSchema = z.object({
  max_upload_bytes: count, max_primary_bytes: count, max_file_bytes: count,
  max_supporting_bytes: count, max_total_bytes: count, max_files: count,
  max_archive_entries: count, max_mcp_bytes: count,
}).transform(value => ({
  maxUploadBytes: value.max_upload_bytes, maxPrimaryBytes: value.max_primary_bytes,
  maxFileBytes: value.max_file_bytes, maxSupportingBytes: value.max_supporting_bytes,
  maxTotalBytes: value.max_total_bytes, maxFiles: value.max_files,
  maxArchiveEntries: value.max_archive_entries, maxMcpBytes: value.max_mcp_bytes,
}));
const listSchema = z.object({ enabled: z.boolean(), can_publish: z.boolean(), items: z.array(resourceSchema), limits: limitsSchema })
  .transform(value => ({ enabled: value.enabled, canPublish: value.can_publish, items: value.items, limits: value.limits }));
const previewSchema = z.object({
  resource: resourceSchema, files: z.array(z.object({ path: z.string(), size: count })),
  preview: z.string(), preview_digest: z.string().min(1), expected_version: z.string().nullable(),
}).transform(value => ({ resource: value.resource, files: value.files, preview: value.preview, previewDigest: value.preview_digest, expectedVersion: value.expected_version }));
const resultSchema = z.object({ resource: resourceSchema, replayed: z.boolean(), operation_id: z.uuid() })
  .transform(value => ({ resource: value.resource, replayed: value.replayed, operationId: value.operation_id }));

export type AdminResource = z.output<typeof resourceSchema>;
export type AdminResourceList = z.output<typeof listSchema>;
export type AdminResourceLimits = AdminResourceList["limits"];
export type AdminResourcePreview = z.output<typeof previewSchema>;
export type AdminResourceResult = z.output<typeof resultSchema>;
export type AdminResourceUpload = { key: string; file: Blob; filename: string };
export type AdminResourcePublish = AdminResourceUpload & { previewDigest: string; expectedVersion: string | null; reason: string };
export type AdminResourceWithdraw = { expectedVersion: string; reason: string };

export const parseAdminResourceList = (raw: unknown): AdminResourceList | null => parseWithFallback(raw, listSchema, null, { endpoint: "GET /api/admin/resources", redact: true });
export const parseAdminResourcePreview = (raw: unknown): AdminResourcePreview | null => parseWithFallback(raw, previewSchema, null, { endpoint: "POST /api/admin/resources/:kind/preview", redact: true });
export const parseAdminResourceResult = (raw: unknown): AdminResourceResult | null => parseWithFallback(raw, resultSchema, null, { endpoint: "/api/admin/resources/operation", redact: true });
