import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { getApi } from "../api";
import { collaborationIssueAccessDenied, CollaborationUnavailableError, isCollaborationDenied, isCollaborationScopeCurrent,
  protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import { collaborationMemoryKeys } from "./memory-queries";
import { collaborationMemoryContentValid, collaborationMemoryReasonValid, compareCollaborationMemoryVersions, nextCollaborationMemoryVersion,
  type CollaborationMemory, type CollaborationMemoryReceipt } from "./memory-schema";
import { collaborationTextByteLength } from "./text-limits";
import { nonnegativeVersion, positiveVersion } from "./version";

const base = { domain: z.literal("memory") };
// The saved shape keeps exactly what was sent, so the original key replays it.
const payload = z.discriminatedUnion("action", [
  z.strictObject({ ...base, action: z.literal("memory_update"), input: z.strictObject({
    expected_version: nonnegativeVersion, content: z.string().refine(collaborationMemoryContentValid) }) }),
  z.strictObject({ ...base, action: z.literal("memory_restore"), version: positiveVersion, input: z.strictObject({ expected_version: nonnegativeVersion }) }),
  z.strictObject({ ...base, action: z.literal("memory_purge"), version: positiveVersion, input: z.strictObject({
    reason: z.string().refine(collaborationMemoryReasonValid) }) }),
]);
export type CollaborationMemoryDraftPayload = z.infer<typeof payload>;
/** An edit names the version it was based on, not the version the panel shows now. */
export type CollaborationMemoryChange =
  | { action: "update"; baseVersion: string; content: string }
  | { action: "restore"; version: string }
  | { action: "purge"; version: string; reason: string };

export function parseCollaborationMemoryDraft(draft: CollaborationDraft): CollaborationMemoryDraftPayload {
  const parsed = payload.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success) throw new CollaborationUnavailableError();
  return parsed.data;
}

export class CollaborationMemoryTooLargeError extends RangeError {
  constructor(readonly bytes: number, readonly maxBytes: number) { super("Shared context exceeds its UTF-8 byte limit"); this.name = "CollaborationMemoryTooLargeError"; }
}

/** Capabilities are display hints; the server rechecks every write. */
export function prepareCollaborationMemoryChange(scope: CollaborationScope, memory: CollaborationMemory, requestId: string, change: CollaborationMemoryChange): CollaborationDraft {
  if (!isCollaborationScopeCurrent(scope) || !z.uuid().safeParse(requestId).success) throw new CollaborationUnavailableError();
  let data: CollaborationMemoryDraftPayload;
  if (change.action === "update") {
    if (memory.can_edit !== true || !nonnegativeVersion.safeParse(change.baseVersion).success ||
      compareCollaborationMemoryVersions(change.baseVersion, memory.version) > 0) throw new CollaborationUnavailableError();
    const bytes = collaborationTextByteLength(change.content);
    if (bytes > memory.max_bytes) throw new CollaborationMemoryTooLargeError(bytes, memory.max_bytes);
    if (change.content.includes("\0")) throw new CollaborationUnavailableError();
    data = payload.parse({ domain: "memory", action: "memory_update", input: { expected_version: change.baseVersion, content: change.content } });
  } else if (change.action === "restore") {
    if (memory.can_edit !== true || change.version === memory.version) throw new CollaborationUnavailableError();
    data = payload.parse({ domain: "memory", action: "memory_restore", version: change.version, input: { expected_version: memory.version } });
  } else {
    // The current version is never purged directly; it must be edited first.
    if (memory.can_purge !== true || change.version === memory.version || !collaborationMemoryReasonValid(change.reason)) throw new CollaborationUnavailableError();
    data = payload.parse({ domain: "memory", action: "memory_purge", version: change.version, input: { reason: change.reason.trim() } });
  }
  return { scope: { server: scope.server, userId: scope.userId, issueId: memory.issue_id }, kind: "operation", requestId, payload: data, createdAt: Date.now() };
}

function matches(data: CollaborationMemoryDraftPayload, issueId: string, requestId: string, receipt: CollaborationMemoryReceipt | null): receipt is CollaborationMemoryReceipt {
  if (!receipt || receipt.issue_id !== issueId || receipt.request_id !== requestId || receipt.action !== data.action) return false;
  if (receipt.status >= 400) return true;
  // A success must be the exact next version (write) or the exact purged version.
  return data.action === "memory_purge" ? receipt.version === data.version
    : receipt.version === nextCollaborationMemoryVersion(data.input.expected_version);
}

/**
 * Preserve the original request before sending, retire it only after a
 * validated receipt (success or sealed refusal), and refresh shared-context
 * reads. A stale_memory_version refusal is returned, never thrown, so the
 * editor can keep the user's input and ask for an explicit merge.
 */
export async function executeCollaborationMemoryChange(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter): Promise<CollaborationMemoryReceipt> {
  const data = parseCollaborationMemoryDraft(source);
  const draft: CollaborationDraft = { ...source, scope: { ...source.scope }, payload: data };
  if (draft.scope.server !== scope.server || draft.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const issueId = draft.scope.issueId;
  const denied = collaborationIssueAccessDenied(client, scope, issueId);
  try {
    const receipt = await protectCollaborationRead(client, scope, recovery && denied ? null : issueId, async () => {
      if (!denied) saveCollaborationDraft(draft, storage);
      const api = getApi();
      const result = recovery ? await api.getCollaborationMemoryOperation(issueId, draft.requestId)
        : data.action === "memory_update" ? await api.updateCollaborationMemory(issueId, draft.requestId, data.input)
          : data.action === "memory_restore" ? await api.restoreCollaborationMemoryVersion(issueId, draft.requestId, data.version, data.input)
            : await api.purgeCollaborationMemoryVersion(issueId, draft.requestId, data.version, data.input);
      return matches(data, issueId, draft.requestId, result) ? result : null;
    });
    removeCollaborationDraft(draft, storage);
    if (!denied) void client.invalidateQueries({ queryKey: collaborationMemoryKeys.root(scope, issueId) });
    return receipt;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope);
    throw error;
  }
}
