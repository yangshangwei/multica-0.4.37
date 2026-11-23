import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import {
  collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope,
} from "./access";
import { invalidateCollaborationSourceViews, waitForCollaborationIssueRefresh } from "./invalidation";
import type { CollaborationIssue } from "./schema";
import {
  collaborationStageFrozenRequest, freezeCollaborationStageIntent, retireCollaborationStageIntent, saveCollaborationStageIntent,
  type CollaborationStageIntent, type CollaborationStageStoredIntent, isCollaborationStageSendable, collaborationStageIntentIdentity,
} from "./stage-intent";
import { deriveCollaborationStageState } from "./stage-presentation";
import { deriveCollaborationStageCodeState } from "./stage-code-presentation";
import { collaborationStageExecutionKnown, collaborationStageRollbackOptions } from "./stage-execution";
import { collaborationTestReportMatchesCandidate } from "./stage-test-presentation";
import type { CollaborationTestReport } from "./stage-test-report-schema";
import { collaborationStageKeys, type CollaborationStagePolicyAuthority } from "./stage-queries";
import {
  CollaborationStageRequestSchemas, collaborationStageBodyLimit, collaborationStageRoleConflicts, matchesCollaborationStageConfirmation,
  type CollaborationStageCandidateRef, type CollaborationStageConfirmation, type CollaborationStageFrozenRequest, type CollaborationStagePolicy,
  type CollaborationStageProposalContentInput, type CollaborationStageProposalSourceInput, type CollaborationStageRolesInput,
  type CollaborationStageSettledResult, type CollaborationStageView, type CollaborationStageRequestBody, type CollaborationStageCodeCandidate, type CollaborationStageBlocker, type CollaborationStageConfirmationIdentity,
} from "./stage-schema";
import { collaborationTextByteLength } from "./text-limits";
import type { StorageAdapter } from "../types/storage";

/** Why an intent could not be built. Nothing was stored or sent, and no key was spent. */
export class CollaborationStagePrepareError extends Error {
  constructor(readonly reason: "unavailable" | "stale_view" | "role_conflict" | "invalid" | "too_large" | "activation_unavailable") {
    super("The stage request could not be prepared");
    this.name = "CollaborationStagePrepareError";
  }
}

type StageSpecificBody<A extends "create_code_candidate" | "decide_stage" | "revoke_decision" | "advance_stage"> = Omit<CollaborationStageRequestBody<A>,
  "expected_collaboration_revision" | "expected_lifecycle_epoch" | "expected_participant_epoch" | "expected_account_epoch" | "expected_policy_revision" | "expected_role_revision" | "expected_stage_revision">;

export type CollaborationStageActionInput =
  | { action: "configure_roles"; roles: CollaborationStageRolesInput }
  | { action: "create_proposal"; content: CollaborationStageProposalContentInput; source: CollaborationStageProposalSourceInput }
  | { action: "revise_proposal"; proposalId: string; expectedCandidate: CollaborationStageCandidateRef; content: CollaborationStageProposalContentInput; source: CollaborationStageProposalSourceInput }
  | { action: "decide_proposal"; proposalId: string; candidate: CollaborationStageCandidateRef; decision: "accept" | "reject"; reason: string }
  | { action: "confirm_scope"; children: CollaborationStageRequestBody<"confirm_scope">["children"]; reason: string }
  | ({ action: "create_code_candidate" } & StageSpecificBody<"create_code_candidate">)
  | ({ action: "decide_stage"; record: CollaborationStageCodeCandidate; evidenceReport?: CollaborationTestReport | null } & StageSpecificBody<"decide_stage">)
  | ({ action: "revoke_decision"; decisionId: string; record: CollaborationStageCodeCandidate } & StageSpecificBody<"revoke_decision">)
  | ({ action: "advance_stage"; record: CollaborationStageCodeCandidate } & StageSpecificBody<"advance_stage">);

/**
 * How an explicit attempt or an original-key recovery ended.
 * - `confirmed`: the exact confirmation of this frozen request; the intent is retired.
 * - `refused`: a refusal committed with the operation; final for this key. The
 *   form keeps its input and a new attempt needs a new key from fresh state.
 * - `not_written`: the only dispatch of a fresh intent was refused before any
 *   operation row existed; nothing was written.
 * - `unknown`: nothing is proven. The intent stays stored; only a GET may ask
 *   again, and only the user may resend the same key or discard the record.
 */
export type CollaborationStageOutcome =
  | { kind: "confirmed"; code: string; confirmation: CollaborationStageConfirmation; cleanupFailed: boolean }
  | { kind: "refused"; status: number; code: string; cleanupFailed: boolean; blockers?: CollaborationStageBlocker[]; blockersTruncated?: boolean }
  | { kind: "not_written"; status: number; code: string; cleanupFailed: boolean }
  | { kind: "unknown"; reason: "transport" | "malformed" | "unavailable" | "incomplete" | "not_found" | "unproven_refusal"; status: number | null; code: string | null };

function freeze(scope: CollaborationScope, input: Omit<Parameters<typeof freezeCollaborationStageIntent>[0], "scope" | "requestId" | "createdAt">): CollaborationStageIntent {
  const parsed = CollaborationStageRequestSchemas[input.action].safeParse(input.body);
  if (!parsed.success) throw new CollaborationStagePrepareError("invalid");
  if (collaborationTextByteLength(JSON.stringify(parsed.data)) > collaborationStageBodyLimit(input.action)) throw new CollaborationStagePrepareError("too_large");
  try {
    return freezeCollaborationStageIntent({ ...input, scope: { server: scope.server, userId: scope.userId }, requestId: crypto.randomUUID(), createdAt: Date.now() });
  } catch { throw new CollaborationStagePrepareError("invalid"); }
}

const sameIds = (a: readonly string[], b: readonly string[]) => a.length === b.length && [...a].sort().every((id, index) => id === [...b].sort()[index]);
const sameCandidate = (a: CollaborationStageCandidateRef, b: CollaborationStageCandidateRef) => a.candidate_id === b.candidate_id && a.version === b.version && a.content_digest === b.content_digest;
export function collaborationStageEvidenceMatches(input: Extract<CollaborationStageActionInput, { action: "decide_stage" }>, validDecisions: CollaborationStageCodeCandidate["decisions"], now = Date.now()): boolean {
  const { record, evidence, depends_on } = input;
  if (input.decision === "return" && depends_on.length === 0 && Object.values(evidence).every((value) => value == null)) return true;
  const baseline = record.candidate.baseline;
  if (!baseline || depends_on.some((id) => !record.decisions.some((d) => d.decision_id === id))) return false;
  if (evidence.ci_digest !== null && evidence.ci_digest !== record.observed.ci.ci_digest) return false;
  const report = record.test_reports.find((r) => r.report_id === evidence.test_report_id);
  if (evidence.test_report_id !== null && !report) return false;
  if (evidence.build !== null && evidence.build.artifact_digest !== baseline.artifact?.artifact_digest) return false;
  if (input.step === "release" && evidence.build !== null && (!collaborationStageExecutionKnown(record) ||
    !record.execution_targets!.some((target) => target.target_digest === evidence.build?.target_digest && target.environment === evidence.build.environment))) return false;
  const deployment = record.execution_facts.find((f) => f.fact_id === evidence.deployment_id && f.kind === "deployment");
  if (evidence.deployment_id !== null && !deployment) return false;
  if (evidence.rollback_target_digest !== null && (!deployment || evidence.rollback_target_digest !== deployment.previous_artifact_digest)) return false;
  if (evidence.rollback_observation != null && (!deployment || !record.rollback_observations?.some((observation) =>
    observation.observation_id === evidence.rollback_observation?.observation_id && observation.observation_digest === evidence.rollback_observation.observation_digest &&
    observation.deployment_id === deployment.fact_id && observation.target_digest === deployment.target_digest && observation.environment === deployment.environment &&
    observation.artifact_digest === deployment.artifact_digest))) return false;
  if (input.decision === "return") return true;
  if (depends_on.some((id) => !validDecisions.some((d) => d.decision_id === id && d.decision === "approved"))) return false;
  const preceding = input.step === "qa" ? "code_review" : input.step === "release" ? "qa" : input.step === "rollback" ? "release" : null;
  if (preceding && !validDecisions.some((d) => d.step === preceding && d.decision === "approved" && depends_on.includes(d.decision_id))) return false;
  if (input.step === "qa" && (!report || report.reported_result !== "passed" || report.applicability !== "usable")) return false;
  if (input.step === "release") {
    const qa = validDecisions.find((d) => d.decision_id === depends_on[0] && d.step === "qa" && d.decision === "approved");
    const detail = input.evidenceReport;
    if (depends_on.length !== 1 || !qa || !detail || detail.report_id !== qa.evidence.test_report_id || !collaborationTestReportMatchesCandidate(record, detail) ||
      detail.reported_result !== "passed" || detail.applicability !== "usable" || detail.subject.artifact_digest === null || detail.subject.artifact_digest !== evidence.build?.artifact_digest) return false;
  }
  if (input.step === "rollback" && (depends_on.length !== 1 || !collaborationStageRollbackOptions(record, validDecisions, now).some((option) =>
    option.approval.decision_id === depends_on[0] && option.deployment.fact_id === evidence.deployment_id && option.observation.observation_id === evidence.rollback_observation?.observation_id &&
    option.observation.observation_digest === evidence.rollback_observation.observation_digest && option.deployment.previous_artifact_digest === evidence.rollback_target_digest))) return false;
  return true;
}

/**
 * Build the frozen intent of one A-action from what the human is looking at:
 * the four workflow observations come from the collaboration detail, the stage
 * revisions from the stage view. Both must describe the same state.
 */
export function prepareCollaborationStageAction(scope: CollaborationScope, issue: CollaborationIssue, view: CollaborationStageView, input: CollaborationStageActionInput): CollaborationStageIntent {
  if (!isCollaborationScopeCurrent(scope)) throw new CollaborationStagePrepareError("unavailable");
  const state = deriveCollaborationStageState({ state: "available", value: view }, issue);
  if (state.availability !== "available" || !issue.write_context) throw new CollaborationStagePrepareError("unavailable");
  if (!state.inSync) throw new CollaborationStagePrepareError("stale_view");
  const codeState = deriveCollaborationStageCodeState(issue, view, "record" in input ? input.record : null, scope.userId);
  const allowed = input.action === "create_code_candidate" ? codeState.writable : input.action === "decide_stage" ?
    (input.decision === "approve" ? codeState.approve : codeState.returns).includes(input.step) : input.action === "revoke_decision" ? codeState.revocable.includes(input.decisionId) :
    input.action === "advance_stage" ? codeState.advances.some((move) => move.stage === input.target_stage && sameIds(move.decisionIds, input.decision_ids)) : input.action === "configure_roles" ? state.can.configureRoles : input.action === "decide_proposal" ? state.can.decideProposal
    : input.action === "confirm_scope" ? state.can.confirmScope : state.can.createProposal;
  if (!allowed) throw new CollaborationStagePrepareError("unavailable");
  const observed = {
    expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch,
    expected_policy_revision: view.policy.policy_revision,
  };
  const target = { kind: "issue", issueId: issue.id } as const;
  switch (input.action) {
    case "configure_roles":
      if (collaborationStageRoleConflicts(input.roles).length > 0) throw new CollaborationStagePrepareError("role_conflict");
      return freeze(scope, { target, action: input.action, body: { ...observed, expected_role_revision: view.role_revision, roles: input.roles } });
    case "create_proposal":
      return freeze(scope, { target, action: input.action, body: { ...observed, content: input.content, source: input.source } });
    case "revise_proposal":
      return freeze(scope, { target, action: input.action, proposalId: input.proposalId,
        body: { ...observed, expected_candidate: input.expectedCandidate, content: input.content, source: input.source } });
    case "decide_proposal":
      return freeze(scope, { target, action: input.action, proposalId: input.proposalId,
        body: { ...observed, expected_role_revision: view.role_revision, candidate: input.candidate, decision: input.decision, reason: input.reason } });
    case "confirm_scope":
      if (input.children.some((selected) => !view.children.items.some((child) => child.issue_id === selected.issue_id.toLowerCase() && child.attached === true &&
        child.can_open === true && child.content_digest !== null && child.content_digest === selected.content_digest))) throw new CollaborationStagePrepareError("stale_view");
      return freeze(scope, { target, action: input.action,
        body: { ...observed, expected_role_revision: view.role_revision, expected_scope_revision: view.scope_revision, children: input.children, reason: input.reason } });
    case "create_code_candidate": {
      const child = view.children.items.find((c) => c.issue_id === input.child_issue_id && c.attached === true && c.can_open === true && c.content_digest !== null && c.approval_current === true);
      if (!child || !child.pull_requests.some((pr) => pr.provider === input.pull_request.provider && pr.pull_request_id === input.pull_request.pull_request_id && pr.head_sha === input.pull_request.head_sha) ||
        input.supersedes !== null && !view.code_candidates?.items.some((c) => (c.state === "current" || c.state === "stale") && sameCandidate(c, input.supersedes!))) throw new CollaborationStagePrepareError("stale_view");
      return freeze(scope, { target, action: input.action, body: { ...observed, expected_role_revision: view.role_revision,
        child_issue_id: input.child_issue_id, supersedes: input.supersedes, pull_request: input.pull_request, artifact: input.artifact } });
    }
    case "decide_stage": {
      if (!sameCandidate(input.candidate, input.record.candidate) || !collaborationStageEvidenceMatches(input, codeState.validDecisions)) throw new CollaborationStagePrepareError("invalid");
      if (input.decision === "approve") {
        const limit = input.record.signable.find((item) => item.step === input.step)?.max_expires_at;
        if (input.expires_at === null || !limit || Date.parse(input.expires_at) <= Date.now() || Date.parse(input.expires_at) > Date.parse(limit)) throw new CollaborationStagePrepareError("invalid");
      }
      return freeze(scope, { target, action: input.action, body: { ...observed, expected_role_revision: view.role_revision, expected_stage_revision: view.stage?.stage_revision,
        step: input.step, decision: input.decision, candidate: input.candidate, evidence: input.evidence, depends_on: input.depends_on, reason: input.reason, expires_at: input.expires_at } });
    }
    case "revoke_decision":
      return freeze(scope, { target, action: input.action, decisionId: input.decisionId, body: { ...observed, expected_role_revision: view.role_revision, reason: input.reason } });
    case "advance_stage":
      if (!sameCandidate(input.candidate, input.record.candidate)) throw new CollaborationStagePrepareError("stale_view");
      return freeze(scope, { target, action: input.action, body: { ...observed, expected_role_revision: view.role_revision, expected_stage_revision: view.stage?.stage_revision,
        target_stage: input.target_stage, candidate: input.candidate, decision_ids: input.decision_ids } });
    default: throw new CollaborationStagePrepareError("invalid");
  }
}

/** P2. Enabling is offered only while the server states it can currently be written. */
export function prepareCollaborationStagePolicyRevision(scope: CollaborationScope, workspaceId: string, policy: CollaborationStagePolicy, input: { enabled: boolean; reason: string }, authority: CollaborationStagePolicyAuthority | null = null): CollaborationStageIntent {
  if (!isCollaborationScopeCurrent(scope)) throw new CollaborationStagePrepareError("unavailable");
  if (!authority || authority.workspaceId !== workspaceId.toLowerCase() || authority.userId !== scope.userId.toLowerCase() || (authority.role !== "owner" && authority.role !== "admin")) throw new CollaborationStagePrepareError("unavailable");
  if (input.enabled === true && policy.activation_available !== true) throw new CollaborationStagePrepareError("activation_unavailable");
  return freeze(scope, { target: { kind: "project", workspaceId, projectId: policy.project_id }, action: "revise_policy",
    body: { expected_policy_revision: policy.policy_revision, enabled: input.enabled, reason: input.reason } });
}

// Dispatch facts of this client only. The stored intent covers a reload; this
// counter also covers overlapping calls and a replaced storage adapter.
const attempts = new WeakMap<QueryClient, Map<string, { count: number }>>();
function attemptState(client: QueryClient, intent: CollaborationStageIntent) {
  let entries = attempts.get(client);
  if (!entries) { entries = new Map(); attempts.set(client, entries); }
  const key = JSON.stringify([intent.scope.server, intent.scope.userId, intent.target, intent.requestId]);
  let state = entries.get(key);
  if (!state) { state = { count: 0 }; entries.set(key, state); }
  return state;
}

function checkIntent(scope: CollaborationScope, intent: CollaborationStageIntent): CollaborationStageFrozenRequest {
  if (!isCollaborationScopeCurrent(scope) || intent.scope.server !== scope.server || intent.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const request = collaborationStageFrozenRequest(intent);
  if (request === null) throw new CollaborationUnavailableError();
  return request;
}

function checkRecoveryIntent(scope: CollaborationScope, intent: CollaborationStageStoredIntent): CollaborationStageConfirmationIdentity {
  if (!isCollaborationScopeCurrent(scope) || intent.scope.server !== scope.server || intent.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const identity = collaborationStageIntentIdentity(intent);
  if (identity === null) throw new CollaborationUnavailableError();
  return identity;
}

const issueOf = (intent: CollaborationStageStoredIntent) => intent.target.kind === "issue" ? intent.target.issueId : null;
type Transport = CollaborationStageSettledResult;

/** Access, session and cancellation answers propagate; any other failure after sending is an unknown result. */
async function observe(request: () => Promise<Transport>): Promise<Transport | { type: "transport" }> {
  try { return await request(); }
  catch (error) {
    if (error instanceof ApiError || error instanceof CollaborationUnavailableError || error instanceof DOMException && error.name === "AbortError") throw error;
    return { type: "transport" };
  }
}

function retire(intent: CollaborationStageStoredIntent, storage?: StorageAdapter): boolean {
  try { retireCollaborationStageIntent(intent, storage); return false; } catch { return true; }
}

function settle(request: CollaborationStageFrozenRequest | CollaborationStageConfirmationIdentity, intent: CollaborationStageStoredIntent, result: Transport | { type: "transport" },
  provenFresh: boolean, storage?: StorageAdapter): CollaborationStageOutcome {
  switch (result.type) {
    case "success":
      // Contract 3.2: only the exact confirmation of the frozen request retires it.
      if (!matchesCollaborationStageConfirmation(request, result)) return { kind: "unknown", reason: "malformed", status: 200, code: result.code };
      return { kind: "confirmed", code: result.code, confirmation: result.confirmation, cleanupFailed: retire(intent, storage) };
    case "refusal":
      if (result.stored) return { kind: "refused", status: result.status, code: result.code, cleanupFailed: retire(intent, storage), ...(result.blockers ? { blockers: result.blockers, blockersTruncated: result.blockersTruncated } : {}) };
      // An unstored refusal proves "nothing written" only for the sole dispatch
      // of an intent that was never stored before. After any earlier attempt the
      // original may still have been applied, whatever this answer says.
      if (provenFresh) return { kind: "not_written", status: result.status, code: result.code, cleanupFailed: retire(intent, storage) };
      return { kind: "unknown", reason: "unproven_refusal", status: result.status, code: result.code };
    case "incomplete": return { kind: "unknown", reason: "incomplete", status: 409, code: "operation_incomplete" };
    case "not_found": return { kind: "unknown", reason: "not_found", status: 404, code: null };
    case "transport": return { kind: "unknown", reason: "transport", status: null, code: null };
    default: return { kind: "unknown", reason: result.reason, status: result.status, code: result.code };
  }
}

async function waitForStageView(client: QueryClient, scope: CollaborationScope, issueId: string): Promise<void> {
  const cache = client.getQueryCache();
  const query = cache.find({ queryKey: collaborationStageKeys.view(scope, issueId), exact: true });
  let seen: Promise<unknown> | undefined;
  while (query && cache.get(query.queryHash) === query && query.state.fetchStatus === "fetching" && query.promise && query.promise !== seen) {
    seen = query.promise;
    await seen.catch(() => undefined);
  }
}

/** A settled write changed server state: re-read the authoritative projections before the next action captures versions. */
async function refresh(client: QueryClient, scope: CollaborationScope, intent: CollaborationStageStoredIntent, outcome: CollaborationStageOutcome): Promise<void> {
  if (outcome.kind === "unknown" || outcome.kind === "not_written" || !isCollaborationScopeCurrent(scope)) return;
  const issueId = issueOf(intent);
  // An accepted proposal creates ordinary issues in the source workspace.
  if (issueId !== null) invalidateCollaborationSourceViews(client, scope, issueId);
  void client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
  if (issueId === null || collaborationIssueAccessDenied(client, scope, issueId)) return;
  await Promise.all([waitForCollaborationIssueRefresh(client, scope, issueId), waitForStageView(client, scope, issueId)]);
}

function send(request: CollaborationStageFrozenRequest, intent: CollaborationStageIntent): Promise<Transport> {
  const api = getApi();
  if (request.action === "revise_policy") {
    return api.reviseProjectStagePolicy({ workspaceId: request.workspaceId, projectId: request.projectId, requestId: intent.requestId, bodyJSON: intent.bodyJSON });
  }
  return api.sendCollaborationStageRequest({ action: request.action, issueId: request.issueId,
    ...("proposalId" in request ? { proposalId: request.proposalId } : {}), ...("decisionId" in request ? { decisionId: request.decisionId } : {}), requestId: intent.requestId, bodyJSON: intent.bodyJSON });
}

/**
 * Explicitly send (or explicitly resend) the frozen key and body. The intent is
 * stored before the request leaves; it is retired only by a proven outcome.
 */
export async function executeCollaborationStageIntent(client: QueryClient, scope: CollaborationScope, intent: CollaborationStageStoredIntent, storage?: StorageAdapter): Promise<CollaborationStageOutcome> {
  if (!isCollaborationStageSendable(intent) || intent.target.kind === "issue" && collaborationIssueAccessDenied(client, scope, intent.target.issueId)) throw new CollaborationUnavailableError();
  const request = checkIntent(scope, intent);
  const attempt = attemptState(client, intent);
  const hadLocalAttempt = attempt.count > 0;
  // Throws before anything is sent; `previouslyStored === false` then proves the key is unused.
  const previouslyStored = saveCollaborationStageIntent(intent, storage);
  const result = await protectCollaborationRead(client, scope, issueOf(intent), () => {
    attempt.count += 1;
    return observe(() => send(request, intent));
  });
  const provenFresh = !hadLocalAttempt && !previouslyStored && attempt.count === 1;
  const outcome = settle(request, intent, result, provenFresh, storage);
  if (outcome.kind === "not_written") attempt.count = 0;
  await refresh(client, scope, intent, outcome);
  return outcome;
}

/**
 * Original-key recovery (O1 / P3). GET only: a missing, incomplete, malformed
 * or contradictory answer keeps the intent and never creates, replays or
 * retries a write.
 */
export async function recoverCollaborationStageIntent(client: QueryClient, scope: CollaborationScope, intent: CollaborationStageStoredIntent, storage?: StorageAdapter): Promise<CollaborationStageOutcome> {
  const request = checkRecoveryIntent(scope, intent);
  const issueId = issueOf(intent);
  // Recovery needs only the account. With current access the issue guard still
  // applies, so a fresh denial clears the protected cache.
  const guard = issueId !== null && !collaborationIssueAccessDenied(client, scope, issueId) ? issueId : null;
  const result = await protectCollaborationRead(client, scope, guard, () => observe(() => {
    const api = getApi();
    return request.action === "revise_policy" ? api.getProjectStagePolicyOperation(request.workspaceId, request.projectId, intent.requestId)
      : api.getCollaborationStageOperation(request.issueId, intent.requestId);
  }));
  const outcome = settle(request, intent, result, false, storage);
  await refresh(client, scope, intent, outcome);
  return outcome;
}

/**
 * Explicit human removal of one stored intent. It sends nothing and is never
 * evidence that the server did not apply the request: a late original write
 * still lands once and shows up in the authoritative reads.
 */
export function dismissCollaborationStageIntent(scope: CollaborationScope, intent: CollaborationStageStoredIntent, storage?: StorageAdapter): void {
  checkRecoveryIntent(scope, intent);
  retireCollaborationStageIntent(intent, storage);
}

export type CollaborationStageOperation = { intent: CollaborationStageStoredIntent; mode: "send" | "recover" };

/** One stage write or recovery at a time per issue; removed with the issue's mutations on access loss. */
export function useCollaborationStageOperation(scope: CollaborationScope, issueId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: [...collaborationKeys.issue(scope, issueId), "stage-operation"], retry: false, networkMode: "always", gcTime: 0,
    mutationFn: ({ intent, mode }: CollaborationStageOperation) => mode === "recover"
      ? recoverCollaborationStageIntent(client, scope, intent) : executeCollaborationStageIntent(client, scope, intent),
  });
}
