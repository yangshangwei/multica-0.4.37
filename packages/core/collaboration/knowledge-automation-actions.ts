import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { getApi } from "../api";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { collaborationKeys, CollaborationUnavailableError, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { expireKnowledgeProjections } from "./knowledge-cache";
import { KnowledgeUnusedIntentError } from "./knowledge-actions";
import { KnowledgeRecoveryStorageError } from "./knowledge-recovery";
import { KnowledgeAutomationRetryRequestSchema, ProjectKnowledgePolicyRequestSchema,
  type KnowledgeAutomationJob, type KnowledgeAutomationReceipt, type ProjectKnowledgePolicy } from "./knowledge-automation-schema";

const scopeSchema = z.strictObject({ server: z.string().min(1), userId: z.uuid() });
const baseIdentity = { scope: scopeSchema, requestId: z.uuid(), protocolVersion: z.literal(2) };
const identitySchema = z.discriminatedUnion("kind", [
  z.strictObject({ ...baseIdentity, kind: z.literal("policy"), projectId: z.uuid() }),
  z.strictObject({ ...baseIdentity, kind: z.literal("retry"), issueId: z.uuid(), jobId: z.uuid() }),
]);
export type KnowledgeAutomationIdentity = z.infer<typeof identitySchema>;
export type KnowledgeAutomationIntent = KnowledgeAutomationIdentity & { readonly requestJSON: string; readonly workspaceId?: string };
const prefix = (scope: KnowledgeAutomationIdentity["scope"]) => `multica_knowledge_automation:${encodeURIComponent(JSON.stringify([scope.server, scope.userId]))}:`;
const keyFor = (identity: KnowledgeAutomationIdentity) => `${prefix(identity.scope)}${identity.requestId}`;
const dispatched = new WeakMap<QueryClient, Set<string>>();
function dispatches(client: QueryClient): Set<string> {
  let entries = dispatched.get(client);
  if (!entries) { entries = new Set(); dispatched.set(client, entries); }
  return entries;
}

function identityOf(input: KnowledgeAutomationIdentity): KnowledgeAutomationIdentity {
  const base = { scope: input.scope, requestId: input.requestId, protocolVersion: input.protocolVersion };
  return identitySchema.parse(input.kind === "policy" ? { ...base, kind: input.kind, projectId: input.projectId }
    : { ...base, kind: input.kind, issueId: input.issueId, jobId: input.jobId });
}
function checkScope(scope: CollaborationScope, identity: KnowledgeAutomationIdentity) {
  identitySchema.parse(identityOf(identity));
  if (!isCollaborationScopeCurrent(scope) || identity.scope.server !== scope.server || identity.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
}
function saveIdentity(identity: KnowledgeAutomationIdentity, storage: StorageAdapter): void {
  let previouslyStored: boolean | undefined;
  try {
    const record = JSON.stringify(identityOf(identity)), key = keyFor(identity), previous = storage.getItem(key);
    previouslyStored = previous !== null;
    if (!storage.keys || previous !== null && previous !== record) throw new KnowledgeRecoveryStorageError();
    storage.setItem(key, record);
    if (storage.getItem(key) !== record) throw new KnowledgeRecoveryStorageError();
  } catch { throw new KnowledgeRecoveryStorageError(previouslyStored); }
}
function retireIdentity(identity: KnowledgeAutomationIdentity, storage: StorageAdapter): void {
  try {
    storage.removeItem(keyFor(identity));
    if (storage.getItem(keyFor(identity)) !== null) throw new KnowledgeRecoveryStorageError();
  } catch { throw new KnowledgeRecoveryStorageError(); }
}

/** Only routing identity is retained. Payloads, source material and receipts are not stored. */
export function readPendingKnowledgeAutomationOperations(scope: KnowledgeAutomationIdentity["scope"], storage: StorageAdapter = defaultStorage): KnowledgeAutomationIdentity[] {
  try {
    if (!storage.keys) throw new KnowledgeRecoveryStorageError();
    return storage.keys().filter((key) => key.startsWith(prefix(scope))).map((key) => {
      const value = identitySchema.parse(JSON.parse(storage.getItem(key) ?? "null"));
      if (keyFor(value) !== key) throw new KnowledgeRecoveryStorageError();
      return value;
    });
  } catch { throw new KnowledgeRecoveryStorageError(); }
}

export function prepareProjectKnowledgePolicy(scope: CollaborationScope, policy: ProjectKnowledgePolicy,
  draft: { isEnabled: boolean; maxCandidates: number }): KnowledgeAutomationIntent {
  if (!isCollaborationScopeCurrent(scope) || policy.can_manage !== true) throw new CollaborationUnavailableError();
  const input = ProjectKnowledgePolicyRequestSchema.parse({ request_id: crypto.randomUUID(), expected_revision: policy.revision,
    is_enabled: draft.isEnabled, max_candidates: draft.maxCandidates });
  return Object.freeze({ kind: "policy", scope: Object.freeze({ server: scope.server, userId: scope.userId }), protocolVersion: 2,
    projectId: policy.project_id, workspaceId: policy.workspace_id, requestId: input.request_id, requestJSON: JSON.stringify(input) });
}

export function prepareKnowledgeAutomationRetry(scope: CollaborationScope, job: KnowledgeAutomationJob): KnowledgeAutomationIntent {
  if (!isCollaborationScopeCurrent(scope) || job.can_retry !== true || job.source_current !== true) throw new CollaborationUnavailableError();
  const input = KnowledgeAutomationRetryRequestSchema.parse({ request_id: crypto.randomUUID(), expected_revision: job.revision });
  return Object.freeze({ kind: "retry", scope: Object.freeze({ server: scope.server, userId: scope.userId }), protocolVersion: 2,
    issueId: job.issue_id, jobId: job.id, requestId: input.request_id, requestJSON: JSON.stringify(input) });
}

export class KnowledgeAutomationReceiptCleanupError extends Error {
  constructor(readonly receipt: KnowledgeAutomationReceipt) { super("The knowledge automation result is known but local recovery cleanup failed"); this.name = "KnowledgeAutomationReceiptCleanupError"; }
}

function matches(identity: KnowledgeAutomationIdentity, receipt: KnowledgeAutomationReceipt | null): receipt is KnowledgeAutomationReceipt {
  return receipt !== null && receipt.protocol_version === 2 && receipt.request_id === identity.requestId &&
    (identity.kind === "policy" ? "project_id" in receipt && receipt.project_id === identity.projectId
      : "job_id" in receipt && receipt.job_id === identity.jobId && receipt.issue_id === identity.issueId);
}
function finish(client: QueryClient, scope: CollaborationScope, identity: KnowledgeAutomationIdentity, receipt: KnowledgeAutomationReceipt, storage: StorageAdapter) {
  expireKnowledgeProjections(client, scope);
  try { retireIdentity(identity, storage); } catch { throw new KnowledgeAutomationReceiptCleanupError(receipt); }
  return receipt;
}

export async function executeKnowledgeAutomationIntent(client: QueryClient, scope: CollaborationScope, intent: KnowledgeAutomationIntent,
  storage: StorageAdapter = defaultStorage): Promise<KnowledgeAutomationReceipt> {
  checkScope(scope, intent);
  const raw: unknown = JSON.parse(intent.requestJSON);
  const input = intent.kind === "policy" ? ProjectKnowledgePolicyRequestSchema.parse(raw) : KnowledgeAutomationRetryRequestSchema.parse(raw);
  if (input.request_id !== intent.requestId || intent.kind === "policy" && !z.uuid().safeParse(intent.workspaceId).success) throw new CollaborationUnavailableError();
  try { saveIdentity(intent, storage); }
  catch (error) {
    if (dispatches(client).has(keyFor(intent)) || !(error instanceof KnowledgeRecoveryStorageError) || error.previouslyStored !== false) throw error;
    let cleanupFailed = false;
    try { retireIdentity(intent, storage); } catch { cleanupFailed = true; }
    throw new KnowledgeUnusedIntentError("storage", cleanupFailed);
  }
  let receipt: KnowledgeAutomationReceipt;
  try {
    receipt = await protectCollaborationRead(client, scope, null, async () => {
      const api = getApi();
      dispatches(client).add(keyFor(intent));
      const result = intent.kind === "policy"
        ? await api.updateProjectKnowledgePolicy(intent.workspaceId!, intent.projectId, ProjectKnowledgePolicyRequestSchema.parse(input))
        : await api.retryCollaborationKnowledgeJob(intent.issueId, intent.jobId, KnowledgeAutomationRetryRequestSchema.parse(input));
      return matches(intent, result) ? result : null;
    });
  } catch (error) {
    if (isCollaborationScopeCurrent(scope)) expireKnowledgeProjections(client, scope);
    throw error;
  }
  return finish(client, scope, intent, receipt, storage);
}

/** Reload recovery only reads the original result. It never reconstructs or resends a request. */
export async function recoverKnowledgeAutomationOperation(client: QueryClient, scope: CollaborationScope, identity: KnowledgeAutomationIdentity,
  storage: StorageAdapter = defaultStorage): Promise<KnowledgeAutomationReceipt> {
  checkScope(scope, identity);
  const receipt = await protectCollaborationRead(client, scope, null, async () => {
    const api = getApi();
    const result = identity.kind === "policy" ? await api.getProjectKnowledgePolicyOperation(identity.projectId, identity.requestId)
      : await api.getCollaborationKnowledgeJobOperation(identity.issueId, identity.jobId, identity.requestId);
    return matches(identity, result) ? result : null;
  });
  return finish(client, scope, identity, receipt, storage);
}

export function dismissKnowledgeAutomationOperation(scope: CollaborationScope, identity: KnowledgeAutomationIdentity, storage: StorageAdapter = defaultStorage): void {
  checkScope(scope, identity); retireIdentity(identity, storage);
}

export function useKnowledgeAutomationAction(scope: CollaborationScope) {
  const client = useQueryClient();
  return useMutation({ mutationKey: [...collaborationKeys.scope(scope), "knowledge-automation-action"], retry: false, gcTime: 0,
    mutationFn: ({ identity, intent }: { identity: KnowledgeAutomationIdentity; intent?: KnowledgeAutomationIntent }) => intent
      ? executeKnowledgeAutomationIntent(client, scope, intent) : recoverKnowledgeAutomationOperation(client, scope, identity) });
}
