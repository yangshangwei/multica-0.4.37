import type { CollaborationIssue } from "./schema";
import type { CollaborationAction } from "./action";

export type CollaborationDetailAction = CollaborationAction["action"] | "discuss";
export type CollaborationDetailStage = "working" | "waiting_information" | "pending_acceptance" | "legacy_review" | "completed" | "cancelled";

/** Derive the next human action from exact facts and current authority together. */
export function deriveCollaborationDetailState(issue: CollaborationIssue, viewerId: string) {
  const currentSubmission = issue.delivery?.current ?? null;
  const latestSubmission = issue.delivery?.latest ?? null;
  const request = issue.current_information_request ?? null;
  const creator = issue.creator.user_id.toLowerCase() === viewerId.toLowerCase();
  const handler = issue.handler.user_id.toLowerCase() === viewerId.toLowerCase();
  const writable = issue.write_context != null;
  const closed = issue.status_category === "done" || issue.status_category === "cancelled";
  const capabilities = issue.capabilities;
  const canAccept = writable && creator && handler && !closed && request === null && issue.status_category === "in_review" &&
    capabilities.complete === true && currentSubmission?.evidence_state === "available" && currentSubmission.decision === null &&
    currentSubmission.lifecycle_epoch === issue.lifecycle_epoch;
  const canSelfComplete = writable && creator && handler && !closed && request === null && currentSubmission === null &&
    capabilities.complete === true && (issue.status_category !== "in_review" || issue.delivery !== undefined && issue.delivery.current === null);
  let stage: CollaborationDetailStage;
  if (issue.status_category === "done") stage = "completed";
  else if (issue.status_category === "cancelled") stage = "cancelled";
  else if (request !== null) stage = "waiting_information";
  else if (currentSubmission?.evidence_state === "available") stage = "pending_acceptance";
  else if (currentSubmission !== null || issue.status_category === "in_review") stage = "legacy_review";
  else stage = "working";

  const available: CollaborationDetailAction[] = [];
  const add = (action: CollaborationDetailAction, enabled: boolean) => { if (enabled && !available.includes(action)) available.push(action); };
  if (writable) {
    add("return_information", !closed && request !== null && request.recipient.user_id.toLowerCase() === viewerId.toLowerCase() && capabilities.return_information === true);
    add("complete", canAccept || canSelfComplete);
    add("return_acceptance", !closed && creator && handler && request === null && currentSubmission !== null && capabilities.return_acceptance === true);
    add("submit_acceptance", !closed && handler && request === null && capabilities.submit_acceptance === true &&
      (currentSubmission === null || currentSubmission.evidence_state !== "available") && !(creator && currentSubmission === null));
    add("request_information", !closed && handler && capabilities.request_information === true);
    add("transfer", !closed && currentSubmission === null && capabilities.transfer === true);
    add("change_status", !closed && request === null && currentSubmission === null && capabilities.change_status === true);
    add("invite", !closed && capabilities.invite === true);
    add("edit", capabilities.edit === true);
    add("edit_source_structure", capabilities.edit_source_structure === true);
    add("change_iteration", capabilities.edit_source_structure === true);
    add("change_ai_policy", creator && capabilities.change_ai_policy === true);
    add("cancel", !closed && creator && capabilities.cancel === true);
    add("reopen", closed && creator && capabilities.reopen === true);
    add("remove", capabilities.remove_participants === true);
    add("leave", capabilities.leave === true);
  }
  add("discuss", capabilities.comment === true);

  let primary: CollaborationDetailAction | null = null;
  if (available.includes("return_information")) primary = "return_information";
  else if (canAccept || canSelfComplete) primary = "complete";
  else if (stage === "legacy_review" && available.includes("return_acceptance")) primary = "return_acceptance";
  else if (stage === "legacy_review" && creator && currentSubmission === null && available.includes("change_status")) primary = "change_status";
  else if (handler && ["backlog", "todo"].includes(issue.status_category) && available.includes("change_status")) primary = "change_status";
  else if (handler && available.includes("submit_acceptance")) primary = "submit_acceptance";
  else if (available.includes("discuss")) primary = "discuss";
  return { stage, primary, secondary: available.filter((action) => action !== primary), currentSubmission, latestSubmission, canAccept, canSelfComplete };
}

/** A compact empty console is safe only with a supported empty factual read. */
export function collaborationHasAIActivity(issue: CollaborationIssue, observations: {
  pending?: boolean | number | readonly unknown[]; selectedRunId?: string | null; error?: unknown; history?: boolean | "unknown";
} = {}) {
  const { pending, selectedRunId, error, history } = observations;
  const unresolved = Array.isArray(pending) ? pending.length > 0 : Boolean(pending);
  if (unresolved || selectedRunId || error || history === true || history === "unknown") return true;
  const execution = issue.execution;
  if (!execution) return true;
  return execution.channels.length > 0 || execution.targets.length > 0 || execution.commands.length > 0 || execution.groups.length > 0 || execution.runs.length > 0;
}
