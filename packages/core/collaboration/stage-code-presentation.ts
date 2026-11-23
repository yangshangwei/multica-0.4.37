import type { CollaborationIssue } from "./schema";
import { collaborationStageSourceDenied, deriveCollaborationStageState } from "./stage-presentation";
import { collaborationStageDeployment, collaborationStageExecutionKnown, collaborationStageRollbackOptions } from "./stage-execution";
import type { CollaborationStageCodeCandidate, CollaborationStageCodeDecision, CollaborationStageRole, CollaborationStageStep, CollaborationStageView, COLLABORATION_STAGES } from "./stage-schema";

export const collaborationStageStepRole = { code_review: "code_reviewer", qa: "qa_approver", release: "release_approver", rollback: "release_approver" } as const;
type Stage = typeof COLLABORATION_STAGES[number];
export interface CollaborationStageCodeState {
  writable: boolean;
  readable: boolean;
  approve: CollaborationStageStep[];
  returns: CollaborationStageStep[];
  revocable: string[];
  advances: { stage: Stage; decisionIds: string[] }[];
  validDecisions: CollaborationStageCodeDecision[];
}
export function collaborationStageHoldsRole(view: CollaborationStageView, viewerId: string, role: CollaborationStageRole): boolean {
  return view.roles?.[role]?.user_id === viewerId.toLowerCase();
}

/** All controls combine current identity/roles, exact candidate observations and server signability. */
export function deriveCollaborationStageCodeState(issue: CollaborationIssue, view: CollaborationStageView, record: CollaborationStageCodeCandidate | null,
  viewerId: string, now = Date.now()): CollaborationStageCodeState {
  const state = deriveCollaborationStageState({ state: "available", value: view }, issue);
  const sourceDenied = collaborationStageSourceDenied(view);
  const baseWritable = view.code_candidates !== undefined && state.known && state.inSync && state.managed === true && issue.write_context != null && view.roles !== null && issue.status_category !== "done" && issue.status_category !== "cancelled";
  const writable = baseWritable && view.scope_state === "current" && view.roles?.implementer != null && !sourceDenied;
  const empty = { writable, readable: false, approve: [], returns: [], revocable: [], advances: [], validDecisions: [] };
  if (!record || record.issue_id !== issue.id.toLowerCase()) return empty;
  const readable = record.blocked_evidence === 0 && record.candidate.baseline !== null && !sourceDenied;
  const discovered = view.code_candidates?.items.find((item) => item.candidate_id === record.candidate.candidate_id);
  const current = baseWritable && view.scope_state === "current" && discovered?.state === "current" && discovered.version === record.candidate.version && discovered.content_digest === record.candidate.content_digest && record.candidate.state === "current" && record.candidate.lifecycle_epoch === issue.lifecycle_epoch && record.candidate.role_revision === view.role_revision;
  const known = record.candidate.state !== "unknown" && record.observed.ci.state !== "unknown" && record.observed.pull_request.state !== "unknown" &&
    record.decisions.every((d) => d.state !== "unknown" && d.step !== "unknown" && d.decision !== "unknown" && d.role !== "unknown") &&
    record.test_reports.every((r) => r.reported_result !== "unknown" && r.applicability !== "unknown") &&
    record.execution_facts.every((f) => f.kind !== "unknown" && f.result !== "unknown") &&
    record.signable.every((entry) => entry.blockers.every((b) => b.code !== "unknown" && b.stage !== "unknown"));
  if (!known) return { ...empty, readable };
  const holds = (role: CollaborationStageRole) => collaborationStageHoldsRole(view, viewerId, role);
  const executionKnown = collaborationStageExecutionKnown(record);
  const revocable = baseWritable && readable ? record.decisions.filter((decision) => decision.state !== "revoked" && decision.revocation === null && decision.role !== "unknown" &&
    (state.can.configureRoles || decision.actor_user_id === viewerId.toLowerCase() && holds(decision.role)) &&
    ((decision.step !== "release" && decision.step !== "rollback") || executionKnown && record.execution_operations!.length < 50) &&
    !record.execution_operations?.some((operation) => operation.approval_decision_id === decision.decision_id) &&
    !record.execution_facts.some((fact) => fact.approval_decision_id === decision.decision_id)).map((decision) => decision.decision_id) : [];
  if (!current) return { ...empty, readable, revocable };
  const canSignRole = (step: CollaborationStageStep) => holds(collaborationStageStepRole[step]) &&
    (step !== "code_review" || viewerId.toLowerCase() !== record.candidate.implementer_user_id) &&
    (step !== "release" && step !== "rollback" || !holds("qa_approver"));
  const atStep = (step: CollaborationStageStep) => step === "rollback" ? view.stage?.current === "verification" : view.stage?.current === step;
  const returns = record.signable.filter((entry) => atStep(entry.step) && canSignRole(entry.step)).map((entry) => entry.step);
  if (!readable || record.candidate.baseline === null || !writable || record.candidate.implementer_user_id !== view.roles?.implementer?.user_id) return { ...empty, readable, returns, revocable };
  const validDecisions = record.decisions.filter((decision) => decision.state === "valid" && decision.revocation === null && decision.role_revision === view.role_revision &&
    decision.policy_revision === view.policy.policy_revision && decision.role !== "unknown" && view.roles?.[decision.role]?.user_id === decision.actor_user_id &&
    (decision.decision === "returned" || decision.expires_at !== null && Date.parse(decision.expires_at) > now));
  const baseline = record.candidate.baseline;
  const goodCI = record.observed.pull_request.state === "current" && record.observed.pull_request.head_sha === baseline.pull_request.head_sha &&
    record.observed.ci.state === "passed" && record.observed.ci.head_sha === baseline.pull_request.head_sha && record.observed.ci.observed_at !== null &&
    Date.parse(record.observed.ci.observed_at) <= now && now - Date.parse(record.observed.ci.observed_at) <= 30 * 60_000;
  const approve = record.signable.filter((entry) => atStep(entry.step) && canSignRole(entry.step) && entry.allowed === true &&
    entry.max_expires_at !== null && Date.parse(entry.max_expires_at) > now &&
    record.observed.pull_request.state === "current" && record.observed.pull_request.head_sha === baseline.pull_request.head_sha && (entry.step !== "code_review" || goodCI) &&
    (entry.step !== "release" || executionKnown && record.execution_targets!.length > 0 && baseline.artifact !== null && validDecisions.some((d) => d.step === "qa" &&
      d.decision === "approved" && record.test_reports.some((r) => r.report_id === d.evidence.test_report_id && r.reported_result === "passed" && r.applicability === "usable"))) &&
    (entry.step !== "rollback" || collaborationStageRollbackOptions(record, validDecisions, now).length > 0)).map((entry) => entry.step);

  const advances: CollaborationStageCodeState["advances"] = [];
  const approved = (step: CollaborationStageStep) => validDecisions.find((decision) => decision.step === step && decision.decision === "approved");
  const canAdvance = (decision: CollaborationStageCodeDecision) => holds("business_owner") || decision.actor_user_id === viewerId.toLowerCase() && decision.step !== "unknown" && canSignRole(decision.step);

  if (view.stage?.current === "development" && holds("business_owner") && goodCI) advances.push({ stage: "code_review", decisionIds: [] });
  const next = view.stage?.current === "code_review" ? { step: "code_review" as const, stage: "qa" as const }
    : view.stage?.current === "qa" ? { step: "qa" as const, stage: "release" as const }
    : view.stage?.current === "release" ? { step: "release" as const, stage: "verification" as const } : null;
  if (next) {
    const decision = approved(next.step);
    const deployed = next.stage !== "verification" || decision && collaborationStageDeployment(record, decision) !== null;
    if (decision && canAdvance(decision) && deployed) advances.push({ stage: next.stage, decisionIds: [decision.decision_id] });
  }
  const returned = validDecisions.find((decision) => decision.decision === "returned" && canAdvance(decision));
  if (returned && view.stage?.current !== "development") advances.push({ stage: "development", decisionIds: [returned.decision_id] });
  return { writable, readable, approve, returns, revocable, advances, validDecisions };
}
