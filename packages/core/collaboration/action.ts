import { z } from "zod";
import { getApi, ApiError } from "../api";
import type { CollaborationIssue, CollaborationOperation } from "./schema";
import { positiveVersion } from "./version";
import { CollaborationUnavailableError, isCollaborationScopeCurrent, type CollaborationScope } from "./access";
import type { CollaborationDraft } from "./draft";

const base = {
  requestId: z.uuid(), expectedCollaborationRevision: positiveVersion, expectedLifecycleEpoch: positiveVersion,
  expectedParticipantEpoch: positiveVersion, expectedAccountEpoch: positiveVersion,
};
const directed = { expectedStatus: z.string().min(1), statusOptionsVersion: z.string().regex(/^[a-f0-9]{64}$/) };
export const CollaborationActionSchema = z.discriminatedUnion("action", [
  z.object({ ...base, action: z.literal("invite"), targetUserId: z.uuid() }),
  z.object({ ...base, action: z.literal("transfer"), targetUserId: z.uuid(), note: z.string() }),
  z.object({ ...base, ...directed, action: z.literal("request_information"), targetUserId: z.uuid(), note: z.string() }),
  z.object({ ...base, ...directed, action: z.literal("return_information"), informationRequestId: z.uuid(), note: z.string() }),
  z.object({ ...base, ...directed, action: z.literal("submit_acceptance"), note: z.string() }),
  z.object({ ...base, action: z.enum(["change_status", "complete", "cancel", "reopen"]), status: z.string().min(1), statusOptionsVersion: directed.statusOptionsVersion }),
  z.object({ ...base, action: z.literal("leave") }),
  z.object({ ...base, action: z.literal("remove"), targetUserId: z.uuid() }),
  z.object({ ...base, action: z.literal("edit"), title: z.string().min(1), description: z.string(), startDate: z.iso.date().nullable(), dueDate: z.iso.date().nullable(), priority: z.enum(["none", "low", "medium", "high", "urgent"]) }),
]);
export type CollaborationAction = z.infer<typeof CollaborationActionSchema>;

export function collaborationWriteContext(issue: CollaborationIssue, requestId: string) {
  if (!issue.write_context) throw new CollaborationUnavailableError();
  return {
    requestId,
    expectedCollaborationRevision: issue.collaboration_revision,
    expectedLifecycleEpoch: issue.lifecycle_epoch,
    expectedParticipantEpoch: issue.write_context.participant_epoch,
    expectedAccountEpoch: issue.write_context.account_epoch,
  };
}

export function parseCollaborationActionDraft(draft: CollaborationDraft): CollaborationAction {
  const parsed = CollaborationActionSchema.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success || parsed.data.requestId !== draft.requestId) throw new CollaborationUnavailableError();
  return parsed.data;
}

export async function sendCollaborationAction(scope: CollaborationScope, issueId: string, action: CollaborationAction): Promise<CollaborationOperation> {
  if (!isCollaborationScopeCurrent(scope)) throw new DOMException("Collaboration session changed", "AbortError");
  const api = getApi();
  let result: CollaborationOperation | null;
  switch (action.action) {
    case "invite": result = await api.addCollaborationParticipant(issueId, action); break;
    case "transfer": result = await api.createCollaborationHandoff(issueId, { ...action, purpose: "transfer" }); break;
    case "request_information": result = await api.createCollaborationHandoff(issueId, { ...action, purpose: "information" }); break;
    case "return_information": result = await api.returnCollaborationInformation(issueId, action); break;
    case "submit_acceptance": result = await api.submitCollaborationAcceptance(issueId, action); break;
    case "change_status": result = await api.changeCollaborationStatus(issueId, action); break;
    case "complete": case "cancel": case "reopen": result = await api.changeCollaborationLifecycle(issueId, { ...action, action: action.action }); break;
    case "leave": result = await api.leaveCollaborationIssue(issueId, action); break;
    case "remove": result = await api.removeCollaborationParticipant(issueId, action.targetUserId, action); break;
    case "edit": result = await api.editCollaborationIssue(issueId, action); break;
    default: throw new CollaborationUnavailableError();
  }
  if (!isCollaborationScopeCurrent(scope)) throw new DOMException("Collaboration session changed", "AbortError");
  if (!result) throw new CollaborationUnavailableError();
  return result;
}

/** A missing receipt does not retire an intent; retry its exact original write. */
export async function recoverCollaborationAction(scope: CollaborationScope, draft: CollaborationDraft): Promise<CollaborationOperation> {
  const action = parseCollaborationActionDraft(draft);
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId || !isCollaborationScopeCurrent(scope)) throw new CollaborationUnavailableError();
  try {
    const receipt = await getApi().getCollaborationOperation(draft.scope.issueId, draft.requestId);
    if (!isCollaborationScopeCurrent(scope)) throw new DOMException("Collaboration session changed", "AbortError");
    if (receipt) return receipt;
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 404) throw error;
  }
  return sendCollaborationAction(scope, draft.scope.issueId, action);
}
