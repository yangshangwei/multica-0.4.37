// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  CollaborationStageRequestSchemas as schemas, parseCollaborationStageCodeCandidate, parseCollaborationStageBlockers,
  classifyCollaborationStageResponse, matchesCollaborationStageConfirmation, collaborationStageConfirmationIdentity, parseCollaborationStageView, collaborationStageBodyLimit,
} from "./stage-schema";
import { stageCodeCandidateRefFixture as candidate, stageCodeEvidenceFixture as evidence, stageCodeViewFixture as view,
  stageConfirmations, stageDigests, stageIds, stageObservationsFixture, stagePullRequestFixture, stageViewManagedFixture, stageCodeReviewDecisionFixture } from "./stage-test-fixtures";
import { collaborationStageFrozenRequest, freezeCollaborationStageIntent } from "./stage-intent";

const observed = { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1" };
const code = { ...observed, child_issue_id: stageIds.child1, supersedes: null, pull_request: stagePullRequestFixture, artifact: null };
const decide = { ...observed, expected_stage_revision: "1", step: "code_review", decision: "approve", candidate, evidence, depends_on: [], reason: "Reviewed", expires_at: "2026-11-14T03:25:00Z" };
const bodies = {
  create_code_candidate: code,
  decide_stage: decide,
  revoke_decision: { ...observed, reason: "New issue found" },
  advance_stage: { ...observed, expected_stage_revision: "1", target_stage: "qa", candidate, decision_ids: [stageIds.decision] },
};

describe("R4 code candidate evidence", () => {
  it("parses identity-only discovery and distinguishes an older omitted projection", () => {
    const discovery = { items: [{ ...candidate, state: "current" }], truncated: false };
    expect(parseCollaborationStageView({ ...stageViewManagedFixture, code_candidates: discovery })?.code_candidates).toEqual(discovery);
    expect(parseCollaborationStageView({ ...stageViewManagedFixture, code_candidates: undefined })?.code_candidates).toBeUndefined();
    expect(parseCollaborationStageView({ ...stageViewManagedFixture, code_candidates: { ...discovery, items: [discovery.items[0], discovery.items[0]] } })).toBeNull();
  });
  it("accepts only the exact anonymous source-denied variant and never partial private fields", () => {
    const denied = { ...view, candidate: { ...view.candidate, baseline: null }, blocked_evidence: 1,
      observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
      signable: view.signable.map((v) => ({ ...v, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "qa" }] })) };
    expect(parseCollaborationStageCodeCandidate(denied)?.candidate.baseline).toBeNull();
    expect(parseCollaborationStageCodeCandidate({ ...denied, candidate: view.candidate })).toBeNull();
    expect(parseCollaborationStageCodeCandidate({ ...denied, blocked_evidence: 12 })).toBeNull();
    expect(parseCollaborationStageCodeCandidate({ ...denied, observed: view.observed })).toBeNull();
    expect(parseCollaborationStageCodeCandidate({ ...denied, signable: view.signable })).toBeNull();
  });
  it("retains exact code, CI and signer horizon", () => {
    expect(parseCollaborationStageCodeCandidate(view)).toMatchObject({ candidate: { ...candidate, baseline: { pull_request: { head_sha: stagePullRequestFixture.head_sha } } }, signable: view.signable });
  });
  it.each([
    ["missing protocol", { ...view, protocol_version: undefined }],
    ["numeric candidate version", { ...view, candidate: { ...view.candidate, version: 1 } }],
    ["missing evidence", { ...view, observed: undefined }],
    ["allowed without horizon", { ...view, signable: view.signable.map((v) => ({ ...v, max_expires_at: null })) }],
    ["missing signer step", { ...view, signable: view.signable.slice(1) }],
    ["duplicate signer step", { ...view, signable: [view.signable[0], ...view.signable.slice(0, 3)] }],
    ["negative blocked count", { ...view, blocked_evidence: -1 }],
    ["empty successful check set", { ...view, observed: { ...view.observed, ci: { ...view.observed.ci, check_count: 0 } } }],
    ["future successful check", { ...view, observed: { ...view.observed, ci: { ...view.observed.ci, observed_at: "2026-11-15T00:00:00Z" } } }],
  ])("rejects %s", (_name, input) => { expect(parseCollaborationStageCodeCandidate(input)).toBeNull(); });
  it("preserves unfamiliar states as unknown rather than current or passed", () => {
    expect(parseCollaborationStageCodeCandidate({ ...view, candidate: { ...view.candidate, state: "future" } })?.candidate.state).toBe("unknown");
    expect(parseCollaborationStageCodeCandidate({ ...view, observed: { ...view.observed, ci: { ...view.observed.ci, state: "neutral" } } })?.observed.ci.state).toBe("unknown");
  });
});

describe("A6-A9 frozen intent and exact confirmations", () => {
  it.each(Object.keys(bodies) as (keyof typeof bodies)[])("freezes and recovers %s with its route identity", (action) => {
    const intent = freezeCollaborationStageIntent({ scope: { server: "https://server.example", userId: stageIds.reviewer }, target: { kind: "issue", issueId: stageIds.issue },
      action, requestId: stageIds.request, ...(action === "revoke_decision" ? { decisionId: stageIds.decision } : {}), body: bodies[action], createdAt: 1 });
    const frozen = collaborationStageFrozenRequest(intent)!;
    const confirmation = action === "decide_stage" ? { ...stageConfirmations.decide_stage, step: "code_review" as const, expires_at: decide.expires_at } : stageConfirmations[action];
    const codes = { create_code_candidate: "stage_code_candidate_created", decide_stage: "stage_decision_recorded", revoke_decision: "stage_decision_revoked", advance_stage: "stage_advanced" };
    const success = classifyCollaborationStageResponse(200, { code: codes[action], collaboration_revision: "12", stage_confirmation: confirmation }, "recovery", "issue");
    expect(success.type).toBe("success");
    if (success.type !== "success") return;
    expect(matchesCollaborationStageConfirmation(frozen, success)).toBe(true);
    expect(matchesCollaborationStageConfirmation(collaborationStageConfirmationIdentity(frozen), success)).toBe(true);
    expect(matchesCollaborationStageConfirmation(frozen, { ...success, confirmation: { ...confirmation, issue_id: stageIds.child2 } })).toBe(false);
  });
  it("requires explicit evidence and expiry and forbids caller-supplied implementers", () => {
    expect(schemas.create_code_candidate.safeParse({ ...code, implementer_user_id: stageIds.reviewer }).success).toBe(false);
    expect(schemas.create_code_candidate.safeParse({ ...code, pull_request: { ...stagePullRequestFixture, head_sha: "abc123" } }).success).toBe(false);
    expect(schemas.decide_stage.safeParse({ ...decide, evidence: { ...evidence, ci_digest: null } }).success).toBe(false);
    expect(schemas.decide_stage.safeParse({ ...decide, expires_at: null }).success).toBe(false);
    expect(schemas.decide_stage.safeParse({ ...decide, decision: "return", expires_at: null, reason: "Needs changes", evidence: { ci_digest: null, test_report_id: null, build: null, deployment_id: null, rollback_target_digest: null } }).success).toBe(true);
    expect(schemas.decide_stage.safeParse({ ...decide, expires_at: "2026-11-14T03:25:00.0000001Z" }).success).toBe(false);
    expect(schemas.decide_stage.safeParse({ ...decide, decision: "return", expires_at: null, reason: "" }).success).toBe(false);
    expect(schemas.decide_stage.safeParse({ ...decide, step: "release", evidence: { ...evidence, ci_digest: null, build: { artifact_digest: stageDigests.child1, environment: "production", target_digest: stageDigests.content2 } }, depends_on: [stageIds.decision] }).success).toBe(true);
    expect(schemas.advance_stage.safeParse({ ...bodies.advance_stage, decision_ids: [stageIds.decision, stageIds.decision] }).success).toBe(false);
  });
  it("matches exact decision content, step, role/policy and sub-millisecond expiry", () => {
    const request = { action: "decide_stage" as const, issueId: stageIds.issue, body: schemas.decide_stage.parse({ ...decide, expires_at: "2026-11-14T03:25:00.000001Z" }) };
    const confirmation = { ...stageConfirmations.decide_stage, step: "code_review" as const, expires_at: request.body.expires_at };
    const result = classifyCollaborationStageResponse(200, { code: "stage_decision_recorded", collaboration_revision: "12", stage_confirmation: confirmation }, "recovery", "issue");
    expect(result.type).toBe("success");
    if (result.type !== "success") return;
    expect(matchesCollaborationStageConfirmation(request, result)).toBe(true);
    for (const change of [{ content_digest: stageDigests.child1 }, { step: "qa" as const }, { role_revision: "2" }, { policy_revision: "2" },
      { expires_at: "2026-11-14T03:25:00.000002Z" }, { decision: "returned" as const, expires_at: null }]) {
      expect(matchesCollaborationStageConfirmation(request, { ...result, confirmation: { ...confirmation, ...change } })).toBe(false);
    }
    expect(collaborationStageBodyLimit("create_code_candidate")).toBe(256 * 1024);
    expect(collaborationStageBodyLimit("decide_stage")).toBe(256 * 1024);
    expect(schemas.decide_stage.safeParse({ ...decide, expires_at: "2026-11-14T04:25:00+01:00" }).success).toBe(false);
  });
  it("matches a code revision to its parent and exact next version", () => {
    const request = { action: "create_code_candidate" as const, issueId: stageIds.issue, body: schemas.create_code_candidate.parse({ ...code, supersedes: candidate }) };
    const result = classifyCollaborationStageResponse(200, { code: "stage_code_candidate_created", collaboration_revision: "12",
      stage_confirmation: { ...stageConfirmations.create_code_candidate, candidate_id: stageIds.candidate2, parent_candidate_id: candidate.candidate_id, version: "2" } }, "write", "issue");
    expect(result.type).toBe("success");
    if (result.type !== "success") return;
    expect(matchesCollaborationStageConfirmation(request, result)).toBe(true);
    expect(matchesCollaborationStageConfirmation(request, { ...result, confirmation: { ...stageConfirmations.create_code_candidate } })).toBe(false);
  });
});

describe("stage blockers", () => {
  const blockers = [{ code: "stage_evidence_unavailable", stage: "qa" }];
  it("keeps source-denied evidence anonymous and treats unknown codes as blocking", () => {
    expect(parseCollaborationStageBlockers(blockers)).toEqual(blockers);
    expect(parseCollaborationStageBlockers([{ ...blockers[0], candidate_id: stageIds.candidate1 }])).toBeNull();
    expect(parseCollaborationStageBlockers([{ code: "future_rule", stage: "qa" }])).toEqual([{ code: "unknown", stage: "qa" }]);
  });
  it("requires the bounded refusal list and preserves it in original-result recovery", () => {
    const refusal = { code: "stage_incomplete", status: 409, collaboration_revision: "12", stage_blockers: blockers, stage_blockers_truncated: false };
    expect(classifyCollaborationStageResponse(200, refusal, "recovery", "issue")).toMatchObject({ type: "refusal", stored: true, blockers, blockersTruncated: false });
    expect(classifyCollaborationStageResponse(200, { ...refusal, stage_blockers: [] }, "recovery", "issue").type).toBe("unknown");
    expect(classifyCollaborationStageResponse(200, { ...refusal, stage_blockers: undefined }, "recovery", "issue").type).toBe("unknown");
  });
});

it("C07 rejects a valid decision carrying a terminal revocation", () => {
  expect(parseCollaborationStageCodeCandidate({ ...view, decisions: [{ ...stageCodeReviewDecisionFixture, revocation: {
    revocation_id: stageIds.scopeDecision, actor_user_id: stageIds.reviewer, created_at: "2026-11-14T03:03:00Z", reason: "Withdrawn" } }] })).toBeNull();
});
