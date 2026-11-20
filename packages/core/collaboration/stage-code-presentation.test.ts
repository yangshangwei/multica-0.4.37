// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { deriveCollaborationStageCodeState } from "./stage-code-presentation";
import { parseCollaborationStageCodeCandidate, parseCollaborationStageView } from "./stage-schema";
import { stageCodeViewFixture, stageIds, stageIssueContextFixture, stageViewManagedFixture, stageCodeReviewDecisionFixture, stageTestReportSummaryFixture } from "./stage-test-fixtures";
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
const view = parseCollaborationStageView({ ...stageViewManagedFixture, code_candidates: { items: [{ candidate_id: stageIds.candidate1, version: "1", content_digest: stageCodeViewFixture.candidate.content_digest, state: "current" }], truncated: false }, stage: { current: "code_review", stage_revision: "2" } })!;
const code = parseCollaborationStageCodeCandidate(stageCodeViewFixture)!;
const now = Date.parse("2026-11-14T03:05:00Z");

// Retained r03 HTTP shape: current source/CI with no bound build and four
// blocked future signatures. These blockers must remain known without granting
// release, rollback, or the business owner's code-review signature.
const developmentWithFutureBlockers = () => ({
  ...stageCodeViewFixture,
  execution_targets: [], execution_operations: [], rollback_observations: [],
  signable: [
    { step: "code_review", allowed: false, max_expires_at: null, blockers: [
      { code: "stage_evidence_unavailable", stage: "code_review" }, { code: "stage_not_reached", stage: "development" },
    ] },
    { step: "qa", allowed: false, max_expires_at: null, blockers: [
      { code: "stage_evidence_unavailable", stage: "qa" }, { code: "stage_not_reached", stage: "development" },
      { code: "stage_code_review_missing", stage: "qa" }, { code: "stage_test_report_unusable", stage: "qa" },
    ] },
    { step: "release", allowed: false, max_expires_at: null, blockers: [
      { code: "stage_evidence_unavailable", stage: "release" }, { code: "stage_not_reached", stage: "development" },
      { code: "stage_evidence_unavailable", stage: "release" }, { code: "stage_execution_unavailable", stage: "release" },
      { code: "stage_qa_missing", stage: "release" },
    ] },
    { step: "rollback", allowed: false, max_expires_at: null, blockers: [
      { code: "stage_evidence_unavailable", stage: "verification" }, { code: "stage_not_reached", stage: "development" },
      { code: "stage_evidence_unavailable", stage: "verification" }, { code: "stage_execution_unavailable", stage: "verification" },
      { code: "stage_release_approval_missing", stage: "verification" }, { code: "stage_rollback_observation_unavailable", stage: "verification" },
    ] },
  ],
});

describe("current development movement with blocked future execution", () => {
  const development = { ...view, stage: { current: "development" as const, stage_revision: "1" } };
  it("keeps the source-authorized business owner's code-review movement available", () => {
    const record = parseCollaborationStageCodeCandidate(developmentWithFutureBlockers())!;
    expect(record.candidate.baseline?.artifact).toBeNull();
    expect(record.observed.ci.state).toBe("passed");
    expect(record.signable.every((entry) => !entry.allowed)).toBe(true);
    const state = deriveCollaborationStageCodeState(issue, development, record, stageIds.owner, now);
    expect(state.advances).toEqual([{ stage: "code_review", decisionIds: [] }]);
    expect(state.approve).toEqual([]);
    expect(state.returns).toEqual([]);
    expect(deriveCollaborationStageCodeState(issue, development, record, stageIds.qa, now).advances).toEqual([]);
    expect(deriveCollaborationStageCodeState({ ...issue, write_context: null }, development, record, stageIds.owner, now).advances).toEqual([]);
  });
  it("does not grant release or rollback because their refusal codes are recognized", () => {
    const record = parseCollaborationStageCodeCandidate(developmentWithFutureBlockers())!;
    for (const current of ["release", "verification"] as const) {
      const state = deriveCollaborationStageCodeState(issue, { ...view, stage: { current, stage_revision: "4" } }, record, stageIds.release, now);
      expect(state.approve).toEqual([]);
      expect(state.advances).toEqual([]);
    }
  });
  it("still withholds all actions for a genuinely unknown future blocker", () => {
    const raw = developmentWithFutureBlockers();
    raw.signable[2]!.blockers[3]!.code = "stage_future_execution_gate";
    const record = parseCollaborationStageCodeCandidate(raw)!;
    expect(record.signable[2]!.blockers[3]!.code).toBe("unknown");
    const state = deriveCollaborationStageCodeState(issue, development, record, stageIds.owner, now);
    expect(state.advances).toEqual([]);
    expect(state.approve).toEqual([]);
    expect(state.returns).toEqual([]);
  });
});

describe("stage code action authority", () => {
  it("requires the current designated reviewer and a current unexpired signable observation", () => {
    expect(deriveCollaborationStageCodeState(issue, view, code, stageIds.reviewer, now).approve).toEqual(["code_review"]);
    expect(deriveCollaborationStageCodeState(issue, view, code, stageIds.implementer, now).approve).toEqual([]);
    expect(deriveCollaborationStageCodeState(issue, view, code, stageIds.product, now).approve).toEqual([]);
    expect(deriveCollaborationStageCodeState(issue, view, code, stageIds.reviewer, Date.parse("2026-11-14T03:25:00Z")).approve).toEqual([]);
  });
  it("refuses missing, blocked, stale, wrong-issue and unknown evidence", () => {
    for (const record of [null, { ...code, blocked_evidence: 1 }, { ...code, issue_id: stageIds.child1 },
      { ...code, candidate: { ...code.candidate, state: "stale" as const } },
      { ...code, observed: { ...code.observed, ci: { ...code.observed.ci, state: "unknown" as const } } }]) {
      expect(deriveCollaborationStageCodeState(issue, view, record, stageIds.reviewer, now).approve).toEqual([]);
    }
    expect(deriveCollaborationStageCodeState({ ...issue, write_context: null }, view, code, stageIds.reviewer, now).approve).toEqual([]);
  });
  it("keeps failed external execution separate from approval and refuses verification advancement", () => {
    const release = { ...view, stage: { current: "release" as const, stage_revision: "4" } };
    expect(deriveCollaborationStageCodeState(issue, release, code, stageIds.owner, now).advances).toEqual([]);
  });
  it("lets current QA return failed or unsettled reports even when approve is blocked", () => {
    const qa = { ...view, stage: { current: "qa" as const, stage_revision: "3" } };
    const failed = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture,
      test_reports: [{ ...stageTestReportSummaryFixture, reported_result: "failed", failed_case_count: 1, applicability: "execution_failed" }],
      signable: stageCodeViewFixture.signable.map((entry) => ({ ...entry, allowed: false, max_expires_at: null })) })!;
    const state = deriveCollaborationStageCodeState(issue, qa, failed, stageIds.qa, now);
    expect(state.approve).toEqual([]);
    expect(state.returns).toEqual(["qa"]);
    expect(deriveCollaborationStageCodeState(issue, qa, failed, stageIds.release, now).returns).toEqual([]);
  });
  it("allows QA advancement only by the business owner or the current unlocking signer while approval remains valid", () => {
    const approved = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [stageCodeReviewDecisionFixture] })!;
    expect(deriveCollaborationStageCodeState(issue, view, approved, stageIds.reviewer, now).advances).toEqual([{ stage: "qa", decisionIds: [stageIds.decision] }]);
    expect(deriveCollaborationStageCodeState(issue, view, approved, stageIds.qa, now).advances).toEqual([]);
    expect(deriveCollaborationStageCodeState(issue, view, approved, stageIds.owner, Date.parse("2026-11-14T03:26:00Z")).advances).toEqual([]);
    expect(deriveCollaborationStageCodeState(issue, view, { ...approved, decisions: approved.decisions.map((d) => ({ ...d, state: "signer_changed" })) }, stageIds.owner, now).advances).toEqual([]);
  });
  it("requires exact successful deployment before verification and never treats release approval as execution", () => {
    const release = { ...view, stage: { current: "release" as const, stage_revision: "4" } };
    const build = { artifact_digest: "c".repeat(64), environment: "production", target_digest: "d".repeat(64) };
    const decision = { ...stageCodeReviewDecisionFixture, step: "release", role: "release_approver", actor_user_id: stageIds.release,
      evidence: { ...stageCodeReviewDecisionFixture.evidence, ci_digest: null, build } };
    const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [decision] })!;
    const fact = { fact_id: stageIds.scopeDecision, intent_id: stageIds.request, target_digest: build.target_digest, kind: "deployment", source_kind: "integration", environment: "production", operation_id: "synthetic-test-deploy-1",
      result: "succeeded", artifact_digest: build.artifact_digest, previous_artifact_digest: null, approval_decision_id: stageIds.decision, observed_at: "2026-11-14T03:04:00Z" };
    expect(deriveCollaborationStageCodeState(issue, release, record, stageIds.release, now).advances).toEqual([]);
    const deployed = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, baseline: {
      ...stageCodeViewFixture.candidate.baseline, artifact: { approval_id: stageIds.proposal, artifact_digest: build.artifact_digest } } },
      decisions: [decision], execution_facts: [fact], execution_targets: [], rollback_observations: [],
      execution_operations: [{ intent_id: fact.intent_id, kind: fact.kind, approval_decision_id: fact.approval_decision_id, environment: fact.environment,
        target_digest: fact.target_digest, artifact_digest: fact.artifact_digest, state: "succeeded", external_operation_id: fact.operation_id, fact_id: fact.fact_id, created_at: fact.observed_at }] })!;
    expect(deriveCollaborationStageCodeState(issue, release, deployed, stageIds.release, now).advances).toEqual([{ stage: "verification", decisionIds: [stageIds.decision] }]);
    for (const change of [{ result: "failed" as const }, { environment: "staging" }, { artifact_digest: "b".repeat(64) }]) {
      expect(deriveCollaborationStageCodeState(issue, release, { ...deployed, execution_facts: [{ ...deployed.execution_facts[0]!, ...change }] }, stageIds.release, now).advances).toEqual([]);
    }
  });
  it("can revoke a superseded candidate's unconsumed decision as its current signer", () => {
    const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, state: "superseded" }, decisions: [stageCodeReviewDecisionFixture] })!;
    const readonly = { ...view, capabilities: { ...view.capabilities, configure_roles: false } };
    expect(deriveCollaborationStageCodeState(issue, readonly, record, stageIds.reviewer, now).revocable).toEqual([stageIds.decision]);
    expect(deriveCollaborationStageCodeState(issue, readonly, record, stageIds.qa, now).revocable).toEqual([]);
  });

});

it("C04 fresh discovery invalidates an old R4 authorization while preserving revocation", () => {
  const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [stageCodeReviewDecisionFixture] })!;
  const fresh = { ...view, code_candidates: { items: [{ candidate_id: record.candidate.candidate_id, version: "1", content_digest: record.candidate.content_digest, state: "superseded" as const }], truncated: false } };
  const state = deriveCollaborationStageCodeState(issue, fresh, record, stageIds.reviewer, now);
  expect(state.approve).toEqual([]); expect(state.advances).toEqual([]); expect(state.revocable).toEqual([stageIds.decision]);
});
it("C03 reason-only return remains available to the current role with source-denied evidence", () => {
  const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, baseline: null }, blocked_evidence: 1,
    observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
    signable: stageCodeViewFixture.signable.map((entry) => ({ ...entry, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "code_review" }] })) })!;
  const state = deriveCollaborationStageCodeState(issue, view, record, stageIds.reviewer, now);
  expect(state.returns).toEqual(["code_review"]);
  expect(deriveCollaborationStageCodeState(issue, { ...view, roles: { ...view.roles!, implementer: null } }, record, stageIds.reviewer, now).returns).toEqual(["code_review"]);
  expect(state.approve).toEqual([]); expect(state.readable).toBe(false);
  expect(deriveCollaborationStageCodeState(issue, view, record, stageIds.qa, now).returns).toEqual([]);
});
it("C07 a revoked decision cannot authorize even when its state is valid", () => {
  const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [stageCodeReviewDecisionFixture] })!;
  const state = deriveCollaborationStageCodeState(issue, view, { ...record, decisions: record.decisions.map((d) => ({ ...d, revocation: {
    revocation_id: stageIds.scopeDecision, actor_user_id: stageIds.reviewer, created_at: "2026-11-14T03:03:00Z", reason: "Withdrawn" } })) }, stageIds.owner, now);
  expect(state.validDecisions).toEqual([]); expect(state.advances).toEqual([]);
});
