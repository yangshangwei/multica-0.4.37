import { invalidateCollaborationSourceViews } from "./invalidation";
import { z } from "zod";
import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import type { StorageAdapter } from "../types/storage";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import { CollaborationUploadDraftSchema, CollaborationDeliveryRequestSchema, finishCollaborationDeliveryDraft, readCollaborationDeliveryRecords,
  removeCollaborationDeliveryRecord, saveCollaborationDeliveryRecord, type CollaborationDeliveryForm, type CollaborationDeliveryRequest,
  type CollaborationDeliveryScope, type CollaborationUploadDraft } from "./delivery-draft";
import { collaborationDeliveryKey, protectCollaborationDelivery } from "./delivery-access";
import { COLLABORATION_ATTACHMENT_MAX_BYTES, type CollaborationAttachment } from "./attachment-schema";

export class CollaborationAttachmentError extends Error {
  constructor(readonly code: string) { super("Collaboration attachment is unavailable"); this.name = "CollaborationAttachmentError"; }
}
async function attachmentRequest<T>(request: () => Promise<T>) {
  try { return await request(); }
  catch (error) {
    const body = error instanceof ApiError ? z.object({ code: z.string() }).safeParse(error.body) : null;
    if (body?.success && (body.data.code.startsWith("attachment_") || body.data.code === "preview_too_large")) throw new CollaborationAttachmentError(body.data.code);
    throw error;
  }
}
function matchesDraft(file: CollaborationAttachment, scope: CollaborationDeliveryScope, draftId: string) {
  return file.draft_id === draftId && file.uploader_user_id === scope.userId.toLowerCase() && file.source_workspace_id === scope.context.sourceWorkspaceId &&
    (scope.context.kind === "creation" ? file.issue_id === null && file.intended_issue_id === null
      : file.intended_issue_id === scope.context.issueId && (file.issue_id === null || file.issue_id === scope.context.issueId));
}
const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchInterval: 15_000 };
export function collaborationAttachmentsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return queryOptions({ queryKey: [...collaborationKeys.issue(scope, issueId), "attachments"], ...freshness,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationAttachments(issueId, { signal })) });
}
export function collaborationDraftAttachmentsOptions(client: QueryClient, scope: CollaborationScope, form: CollaborationDeliveryForm) {
  return queryOptions({ queryKey: [...collaborationDeliveryKey(scope, form.scope.context), "draft", form.draftId], ...freshness,
    queryFn: ({ signal }) => protectCollaborationDelivery(client, scope, form.scope.context, async () => {
      const result = await attachmentRequest(() => getApi().getCollaborationDraftAttachments(form.draftId, { signal }));
      return result && result.attachments.every((file) => matchesDraft(file, form.scope, form.draftId)) ? result : null;
    }) });
}
async function fingerprint(file: File, signal?: AbortSignal) {
  signal?.throwIfAborted();
  const bytes = await file.arrayBuffer();
  signal?.throwIfAborted();
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  signal?.throwIfAborted();
  return { name: file.name, type: file.type, size: file.size, sha256: [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, "0")).join("") };
}
export async function prepareCollaborationUpload(form: CollaborationDeliveryForm, requestId: string, file: File, signal?: AbortSignal): Promise<CollaborationUploadDraft> {
  if (file.size > COLLABORATION_ATTACHMENT_MAX_BYTES) throw new CollaborationAttachmentError("attachment_too_large");
  return CollaborationUploadDraftSchema.parse({ scope: form.scope, draftId: form.draftId, kind: "upload", requestId, file: await fingerprint(file, signal), createdAt: Date.now() });
}
export async function stageCollaborationUpload(client: QueryClient, scope: CollaborationScope, form: CollaborationDeliveryForm, requestId: string, file: File, storage?: StorageAdapter, signal?: AbortSignal) {
  return protectCollaborationDelivery(client, scope, form.scope.context, async (assertCurrent) => {
    const draft = await prepareCollaborationUpload(form, requestId, file, signal);
    assertCurrent();
    const existing = readCollaborationDeliveryRecords(form.scope, storage).find((record) => record.kind === "form" && record.draftId === form.draftId);
    if (!existing) saveCollaborationDeliveryRecord(form, storage);
    saveCollaborationDeliveryRecord(draft, storage);
    return draft;
  }, false, storage);
}
export async function executeCollaborationUpload(client: QueryClient, scope: CollaborationScope, value: CollaborationUploadDraft, file?: File, storage?: StorageAdapter, signal?: AbortSignal) {
  const draft = CollaborationUploadDraftSchema.parse(value);
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const context = draft.scope.context;
  const result = await protectCollaborationDelivery(client, scope, context, async (assertCurrent) => {
    signal?.throwIfAborted();
    if (file && JSON.stringify(await fingerprint(file, signal)) !== JSON.stringify(draft.file)) throw new CollaborationAttachmentError("attachment_file_changed");
    assertCurrent();
    saveCollaborationDeliveryRecord(draft, storage);
    const attachment = await attachmentRequest(() => file
      ? getApi().uploadCollaborationAttachment(context.kind === "creation" ? { draft_id: draft.draftId, source_workspace_id: context.sourceWorkspaceId }
        : { draft_id: draft.draftId, issue_id: context.issueId }, draft.requestId, file, { signal })
      : getApi().getCollaborationAttachmentUpload(draft.draftId, draft.requestId, { signal }));
    signal?.throwIfAborted();
    assertCurrent();
    if (!attachment || !matchesDraft(attachment, draft.scope, draft.draftId) || attachment.size_bytes !== draft.file.size) return null;
    const form = readCollaborationDeliveryRecords(draft.scope, storage).find((record): record is CollaborationDeliveryForm => record.kind === "form" && record.draftId === draft.draftId);
    if (!form) throw new CollaborationUnavailableError();
    saveCollaborationDeliveryRecord({ ...form, selectedAttachmentIds: [...new Set([...form.selectedAttachmentIds, attachment.id])] }, storage);
    removeCollaborationDeliveryRecord(draft, storage);
    return attachment;
  }, false, storage);
  void client.invalidateQueries({ queryKey: [...collaborationDeliveryKey(scope, context), "draft", draft.draftId] });
  return result;
}
export function prepareCollaborationAttachmentDelivery(form: CollaborationDeliveryForm, requestId: string): CollaborationDeliveryRequest {
  return CollaborationDeliveryRequestSchema.parse({ scope: form.scope, draftId: form.draftId, kind: "delivery", requestId,
    input: { draft_id: form.draftId, attachment_ids: form.selectedAttachmentIds }, createdAt: Date.now() });
}
export async function executeCollaborationAttachmentDelivery(client: QueryClient, scope: CollaborationScope, value: CollaborationDeliveryRequest, recovery = false, storage?: StorageAdapter) {
  const draft = CollaborationDeliveryRequestSchema.parse(value);
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId.toLowerCase() || draft.scope.context.kind !== "issue") throw new CollaborationUnavailableError();
  const issueId = draft.scope.context.issueId;
  const result = await protectCollaborationDelivery(client, scope, draft.scope.context, async () => {
    saveCollaborationDeliveryRecord(draft, storage);
    const receipt = await attachmentRequest(() => recovery ? getApi().getCollaborationAttachmentDeliveryOperation(issueId, draft.requestId)
      : getApi().deliverCollaborationAttachments(issueId, draft.requestId, draft.input));
    return receipt && receipt.attachment_ids.length === draft.input.attachment_ids.length && receipt.attachment_ids.every((id) => draft.input.attachment_ids.includes(id)) ? receipt : null;
  }, false, storage);
  finishCollaborationDeliveryDraft(draft.scope, draft.draftId, storage);
  invalidateCollaborationSourceViews(client, scope, issueId);
    void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issueId) });
  return result;
}
async function readIssueAttachment<T>(client: QueryClient, scope: CollaborationScope, issueId: string, request: () => Promise<T | null>, signal?: AbortSignal) {
  return protectCollaborationRead(client, scope, issueId, async () => {
    try { return await attachmentRequest(request); }
    catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        // File absence and issue denial share this response. Recheck the issue
        // before allowing a file-specific failure to clear all its content.
        await protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationIssue(issueId, { signal }));
        throw new CollaborationAttachmentError("attachment_not_found");
      }
      throw error;
    }
  });
}
export async function loadCollaborationAttachmentPreview(client: QueryClient, scope: CollaborationScope, file: CollaborationAttachment, signal?: AbortSignal) {
  if (!file.issue_id) throw new CollaborationUnavailableError();
  return readIssueAttachment(client, scope, file.issue_id, () => getApi().getCollaborationAttachmentBytes(file.issue_id!, file.id, "content", { signal }), signal);
}
export async function loadCollaborationAttachmentDownload(client: QueryClient, scope: CollaborationScope, file: CollaborationAttachment) {
  if (!file.issue_id) throw new CollaborationUnavailableError();
  return readIssueAttachment(client, scope, file.issue_id, () => getApi().getCollaborationAttachmentCapability(file.issue_id!, file.id, file.version, "download"));
}
