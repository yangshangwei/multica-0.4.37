import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { CollaborationCreationInputSchema, CollaborationCreationFieldsSchema } from "./creation-schema";
import { CollaborationAttachmentSelectionSchema, collaborationAttachmentIds } from "./attachment-schema";
import { CollaborationAccountSummarySchema } from "./schema";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const prefix = "multica_collaboration_delivery:";
export const CollaborationDeliveryContextSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("creation"), sourceWorkspaceId: uuid }),
  z.strictObject({ kind: z.literal("issue"), issueId: uuid, sourceWorkspaceId: uuid }),
]);
const scopeSchema = z.strictObject({ server: z.string(), userId: uuid, context: CollaborationDeliveryContextSchema });
const base = { scope: scopeSchema, draftId: uuid, createdAt: z.number().int().nonnegative() };
export const CollaborationDeliveryFormSchema = z.strictObject({
  ...base, kind: z.literal("form"), purpose: z.enum(["creation", "comment", "attachment"]),
  title: z.string().max(1000), content: z.string().max(1_000_000), recipientQuery: z.string().max(1000),
  recipient: CollaborationAccountSummarySchema.nullable(), selectedAttachmentIds: collaborationAttachmentIds,
  parentCommentId: uuid.nullable().optional(),
  // A mutable form retains an unfinished numeric value; immutable requests
  // still use the stricter creation schema before any transport begins.
  initialFields: CollaborationCreationFieldsSchema.safeExtend({ stage: z.number().nullable().optional() }).optional(),
}).refine((form) => (form.purpose === "creation") === (form.scope.context.kind === "creation") && (form.parentCommentId == null || form.purpose === "comment"));
const fileSchema = z.strictObject({ name: z.string().min(1), type: z.string(), size: z.number().int().nonnegative().max(104857600), sha256: z.string().regex(/^[a-f0-9]{64}$/) });
export const CollaborationUploadDraftSchema = z.strictObject({ ...base, kind: z.literal("upload"), requestId: uuid, file: fileSchema });
export const CollaborationCreationDraftSchema = z.strictObject({ ...base, kind: z.literal("creation"), requestId: uuid, input: CollaborationCreationInputSchema.nullable() })
  .refine((draft) => draft.scope.context.kind === "creation" && (draft.input === null ||
    draft.input.source_workspace_id === draft.scope.context.sourceWorkspaceId && (draft.input.draft_id === undefined || draft.input.draft_id === draft.draftId)));
export const CollaborationDeliveryRequestSchema = z.strictObject({ ...base, kind: z.literal("delivery"), requestId: uuid, input: CollaborationAttachmentSelectionSchema })
  .refine((draft) => draft.scope.context.kind === "issue" && draft.input.draft_id === draft.draftId);
const recordSchema = z.union([CollaborationDeliveryFormSchema, CollaborationUploadDraftSchema, CollaborationCreationDraftSchema, CollaborationDeliveryRequestSchema]);
export type CollaborationDeliveryScope = z.infer<typeof scopeSchema>;
export type CollaborationDeliveryContext = z.infer<typeof CollaborationDeliveryContextSchema>;
export type CollaborationDeliveryForm = z.infer<typeof CollaborationDeliveryFormSchema>;
export type CollaborationUploadDraft = z.infer<typeof CollaborationUploadDraftSchema>;
export type CollaborationCreationDraft = z.infer<typeof CollaborationCreationDraftSchema>;
export type CollaborationDeliveryRequest = z.infer<typeof CollaborationDeliveryRequestSchema>;
export type CollaborationDeliveryRecord = z.infer<typeof recordSchema>;
export class CollaborationDeliveryDraftError extends Error {
  constructor() { super("Collaboration delivery draft could not be preserved"); this.name = "CollaborationDeliveryDraftError"; }
}
const listeners = new Set<() => void>();
let revision = 0;
export const collaborationDeliveryRevision = () => revision;
export function subscribeCollaborationDelivery(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; }
function notify() { revision += 1; for (const listener of listeners) listener(); }
function scopePrefix(scope: CollaborationDeliveryScope) {
  const context = scope.context;
  return `${prefix}${encodeURIComponent(JSON.stringify([scope.server, scope.userId.toLowerCase(), context.kind,
    context.kind === "issue" ? context.issueId.toLowerCase() : context.sourceWorkspaceId.toLowerCase()]))}:`;
}
function recordKey(record: CollaborationDeliveryRecord) { return `${scopePrefix(record.scope)}${record.draftId}:${record.kind}:${record.kind === "form" ? record.purpose : record.requestId}`; }

export function readCollaborationDeliveryRecords(scope: Pick<CollaborationDeliveryScope, "server" | "userId"> & { context?: CollaborationDeliveryContext }, storage: StorageAdapter = defaultStorage): CollaborationDeliveryRecord[] {
  try {
    if (!storage.keys) throw new CollaborationDeliveryDraftError();
    const records: CollaborationDeliveryRecord[] = [];
    for (const key of storage.keys()) {
      if (!key.startsWith(prefix)) continue;
      const identity = z.tuple([z.string(), uuid, z.enum(["creation", "issue"]), uuid]).parse(JSON.parse(decodeURIComponent(key.slice(prefix.length).split(":", 1)[0]!)));
      if (identity[0] !== scope.server || identity[1] !== scope.userId.toLowerCase()) continue;
      if (scope.context && !key.startsWith(scopePrefix({ ...scope, context: scope.context }))) continue;
      const raw = storage.getItem(key);
      if (raw === null) continue;
      const record = recordSchema.parse(JSON.parse(raw));
      if (recordKey(record) !== key) throw new CollaborationDeliveryDraftError();
      records.push(record);
    }
    return records.sort((a, b) => a.createdAt - b.createdAt || a.draftId.localeCompare(b.draftId));
  } catch { throw new CollaborationDeliveryDraftError(); }
}

/** One key per operation; mutable form content never overwrites a sent payload. */
export function saveCollaborationDeliveryRecord(record: CollaborationDeliveryRecord, storage: StorageAdapter = defaultStorage): void {
  try {
    const parsed = recordSchema.parse(record);
    if (!storage.keys) throw new CollaborationDeliveryDraftError();
    const key = recordKey(parsed);
    const encoded = JSON.stringify(parsed);
    const old = storage.getItem(key);
    if (parsed.kind !== "form" && old !== null && old !== encoded) throw new CollaborationDeliveryDraftError();
    if (parsed.kind === "creation" || parsed.kind === "delivery") {
      const conflicting = readCollaborationDeliveryRecords(parsed.scope, storage).some((entry) => entry.kind === parsed.kind && entry.draftId === parsed.draftId && recordKey(entry) !== key);
      if (conflicting) throw new CollaborationDeliveryDraftError();
    }
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new CollaborationDeliveryDraftError();
    notify();
  } catch { throw new CollaborationDeliveryDraftError(); }
}
export function removeCollaborationDeliveryRecord(record: CollaborationDeliveryRecord, storage: StorageAdapter = defaultStorage) {
  const key = recordKey(record);
  storage.removeItem(key);
  if (storage.getItem(key) !== null) throw new CollaborationDeliveryDraftError();
  notify();
}
export function finishCollaborationDeliveryDraft(scope: CollaborationDeliveryScope, draftId: string, storage: StorageAdapter = defaultStorage) {
  for (const record of readCollaborationDeliveryRecords(scope, storage)) if (record.draftId === draftId) removeCollaborationDeliveryRecord(record, storage);
}

export function clearCollaborationDeliveryDrafts(storage: StorageAdapter = defaultStorage, scope?: { server: string; userId: string; issueId: string }) {
  try {
    for (const key of storage.keys?.() ?? []) {
      if (!key.startsWith(prefix)) continue;
      if (scope) {
        const identity: unknown = JSON.parse(decodeURIComponent(key.slice(prefix.length).split(":", 1)[0]!));
        if (!Array.isArray(identity) || identity[0] !== scope.server || identity[1] !== scope.userId.toLowerCase() || identity[2] !== "issue" || identity[3] !== scope.issueId.toLowerCase()) continue;
      }
      storage.removeItem(key);
    }
  } catch { /* An unreadable store cannot authorize further delivery. */ }
  finally { notify(); }
}
export function retainCollaborationDeliveryScope(scope: { server: string; userId: string }, storage: StorageAdapter = defaultStorage) {
  try {
    for (const key of storage.keys?.() ?? []) {
      if (!key.startsWith(prefix)) continue;
      let owned = false;
      try {
        const identity: unknown = JSON.parse(decodeURIComponent(key.slice(prefix.length).split(":", 1)[0]!));
        owned = Array.isArray(identity) && identity[0] === scope.server && identity[1] === scope.userId.toLowerCase();
      } catch { /* Malformed identity cannot preserve another viewer's content. */ }
      if (!owned) storage.removeItem(key);
    }
  } catch { /* Scope checks still fence an unavailable store. */ }
  finally { notify(); }
}

/** Source loss removes private inputs but preserves content-free creation recovery. */
export function clearCollaborationCreationDrafts(scope: { server: string; userId: string }, sourceId: string, storage: StorageAdapter = defaultStorage) {
  for (const record of readCollaborationDeliveryRecords({ ...scope, context: { kind: "creation", sourceWorkspaceId: sourceId } }, storage)) {
    if (record.kind === "creation") storage.setItem(recordKey(record), JSON.stringify({ ...record, input: null }));
    else storage.removeItem(recordKey(record));
  }
  notify();
}

export function newCollaborationDeliveryForm(scope: CollaborationDeliveryScope, purpose: CollaborationDeliveryForm["purpose"]): CollaborationDeliveryForm {
  return CollaborationDeliveryFormSchema.parse({ scope: { server: scope.server, userId: scope.userId, context: scope.context }, purpose, kind: "form", draftId: crypto.randomUUID(), createdAt: Date.now(),
    title: "", content: "", recipientQuery: "", recipient: null, selectedAttachmentIds: [] });
}
