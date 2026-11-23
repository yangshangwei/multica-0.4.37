/**
 * Wire-shaped fixtures of `collaboration-stage-v1` (research/wire-contract.md, frozen v1).
 * Self-contained on purpose: `test-fixtures.ts` re-exports this file, so it must
 * not import that entry back. The three values below mirror
 * `collaborationIssueFixture`; stage-schema.test.ts fails if they drift.
 */
const id = (value: number) => `00000000-0000-4000-8000-${value.toString(16).padStart(12, "0")}`;
const digestOf = (seed: string) => seed.repeat(64).slice(0, 64);
const collaborationIssueFixture = { id: id(1), collaboration_revision: "9007199254740993", lifecycle_epoch: "1" };

export const stageIds = {
  issue: collaborationIssueFixture.id, project: id(0x101), workspace: collaborationIssueFixture.id,
  owner: id(0x201), implementer: id(0x202), product: id(0x203), reviewer: id(0x204), qa: id(0x205), release: id(0x206),
  proposal: id(0x301), candidate1: id(0x311), candidate2: id(0x312), decision: id(0x321), scopeDecision: id(0x322),
  child1: id(0x401), child2: id(0x402), comment: id(0x501), run: id(0x502), agent: id(0x503),
  request: id(0x601), request2: id(0x602),
};
export const stageDigests = { content1: digestOf("a"), content2: digestOf("b"), child1: digestOf("c"), child2: digestOf("d"), comment: digestOf("e") };
export const stagePullRequestFixture = { provider: "github" as const, pull_request_id: id(0x701), head_sha: "a".repeat(40) };

const account = (userId: string, name: string) => ({ user_id: userId, name, avatar_url: "", external: false });

export const stagePolicyFixture = {
  contract: "collaboration-stage-v1", project_id: stageIds.project, policy_revision: "1", enabled: true,
  reason: "Pilot project", created_by: stageIds.owner, created_at: "2026-11-14T00:00:00Z", activation_available: true,
};
export const stagePolicyVirtualFixture = {
  contract: "collaboration-stage-v1", project_id: stageIds.project, policy_revision: "0", enabled: false,
  reason: "", created_by: null, created_at: null, activation_available: false,
};

export const stageRolesFixture = {
  role_revision: "1", configured_by: stageIds.owner, configured_at: "2026-11-14T00:10:00Z",
  business_owner: account(stageIds.owner, "Olivia Owner"), implementer: account(stageIds.implementer, "Ivan Implementer"),
  product_approver: account(stageIds.product, "Priya Product"), code_reviewer: account(stageIds.reviewer, "Rui Reviewer"),
  qa_approver: account(stageIds.qa, "Quinn QA"), release_approver: account(stageIds.release, "Rae Release"),
};

export const stageChildFixture = {
  issue_id: stageIds.child1, identifier: "SRC-12", title: "Existing checkout child", can_open: true, creator_type: "member",
  origin: "none", attached: true, content_digest: stageDigests.child1, semantic_version: null, approved_lifecycle_epoch: null,
  approval_current: false, execution_gated: false, proposal_id: null, ordinal: null,
  depth: 1, code: null, code_state: "none", pull_requests: [stagePullRequestFixture],
};

/** Enabled project, nothing configured yet: the first-enablement state. */
export const stageViewNeedsConfirmationFixture = {
  protocol_version: 1, issue_id: stageIds.issue,
  collaboration_revision: collaborationIssueFixture.collaboration_revision, lifecycle_epoch: collaborationIssueFixture.lifecycle_epoch,
  policy: { contract: "collaboration-stage-v1", project_id: stageIds.project, state: "enabled", policy_revision: "1" },
  managed: true, needs_confirmation: true, missing: ["roles", "scope"],
  stage: { current: "development", stage_revision: "1" },
  role_revision: "0", roles: null, scope_revision: "0", scope_state: "none", scope: null,
  children: { items: [stageChildFixture], truncated: false },
  code_candidates: { items: [], truncated: false },
  capabilities: { configure_roles: true, create_proposal: true, decide_proposal: false, confirm_scope: false },
};

export const stageScopeFixture = {
  scope_revision: "1", decision_id: stageIds.scopeDecision, lifecycle_epoch: collaborationIssueFixture.lifecycle_epoch, policy_revision: "1",
  role_revision: "1", confirmed_by: stageIds.product, confirmed_at: "2026-11-14T00:20:00Z", child_issue_ids: [stageIds.child1],
};

/** Roles configured and the existing-child scope confirmed: proposals can be decided. */
export const stageViewManagedFixture = {
  ...stageViewNeedsConfirmationFixture,
  needs_confirmation: false, missing: [] as string[],
  role_revision: "1", roles: stageRolesFixture, scope_revision: "1", scope_state: "current", scope: stageScopeFixture,
  children: { items: [
    { ...stageChildFixture, origin: "confirmed", semantic_version: "1", approved_lifecycle_epoch: "1", approval_current: true },
    { ...stageChildFixture, issue_id: stageIds.child2, identifier: "SRC-13", title: "Materialized refund child", creator_type: "member",
      origin: "materialized", content_digest: stageDigests.child2, semantic_version: "1", approved_lifecycle_epoch: "1", approval_current: true,
      execution_gated: false, proposal_id: stageIds.proposal, ordinal: 1 },
  ], truncated: false },
  capabilities: { configure_roles: true, create_proposal: true, decide_proposal: true, confirm_scope: true },
};

/** Root without an enabled policy: history is projected, nothing is offered. */
export const stageViewUnmanagedFixture = {
  ...stageViewNeedsConfirmationFixture,
  policy: { contract: "collaboration-stage-v1", project_id: stageIds.project, state: "disabled", policy_revision: "2" },
  managed: false, needs_confirmation: false, missing: [] as string[], stage: null,
  capabilities: { configure_roles: false, create_proposal: false, decide_proposal: false, confirm_scope: false },
};

export const stageProposalContentFixture = {
  summary: "Split the refund flow",
  children: [
    { title: "Refund API", description: "Expose the refund endpoint", priority: "high" as const },
    { title: "Refund ledger", description: "", priority: "none" as const },
  ],
};

export const stageCandidateV1Fixture = {
  candidate_id: stageIds.candidate1, version: "1", content_digest: stageDigests.content1, parent_candidate_id: null, lifecycle_epoch: "1",
  state: "superseded", submitted_by: stageIds.implementer, created_at: "2026-11-14T01:00:00Z",
  content: stageProposalContentFixture,
  source: { kind: "comment", comment_id: stageIds.comment, comment_revision: "3", author_type: "agent", author_id: stageIds.agent, digest: stageDigests.comment },
};
export const stageCandidateV2Fixture = {
  ...stageCandidateV1Fixture, candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2,
  parent_candidate_id: stageIds.candidate1, state: "pending", submitted_by: stageIds.product, created_at: "2026-11-14T02:00:00Z",
  content: { ...stageProposalContentFixture, summary: "Split the refund flow (approver edit)" }, source: { kind: "manual" },
};

export const stageDecisionFixture = {
  decision_id: stageIds.decision, step: "proposal", decision: "accepted", candidate_id: stageIds.candidate2, version: "2",
  content_digest: stageDigests.content2, role: "product_approver", role_revision: "1", policy_revision: "1", lifecycle_epoch: "1",
  actor_user_id: stageIds.product, reason: "Scope agreed", created_at: "2026-11-14T03:00:00Z", accepted_with_modification: true,
};

export const stageProposalListItemFixture = {
  proposal_id: stageIds.proposal, state: "pending", version_count: "2", created_at: "2026-11-14T01:00:00Z",
  current: { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2, parent_candidate_id: stageIds.candidate1,
    lifecycle_epoch: "1", stale: false, submitted_by: stageIds.product, created_at: "2026-11-14T02:00:00Z",
    summary: "Split the refund flow (approver edit)", child_count: 2 },
  decision: null,
};
export const stageProposalPageFixture = { issue_id: stageIds.issue, items: [stageProposalListItemFixture], next_cursor: null };

export const stageProposalDetailFixture = {
  issue_id: stageIds.issue, proposal_id: stageIds.proposal, state: "pending",
  versions: [stageCandidateV2Fixture, stageCandidateV1Fixture], versions_truncated: false,
  decisions: [] as unknown[], children: [] as unknown[],
};
export const stageProposalAcceptedDetailFixture = {
  ...stageProposalDetailFixture, state: "accepted",
  versions: [{ ...stageCandidateV2Fixture, state: "accepted" }, stageCandidateV1Fixture],
  decisions: [stageDecisionFixture],
  children: [
    { ordinal: 1, issue_id: stageIds.child2, identifier: "SRC-13", title: "Refund API", can_open: true, attached: true },
    { ordinal: 2, issue_id: id(0x403), identifier: "SRC-14", title: "Refund ledger", can_open: true, attached: true },
  ],
};

/** The four workflow observations every A-action carries (contract 3.1). */
export const stageObservationsFixture = {
  expected_collaboration_revision: collaborationIssueFixture.collaboration_revision, expected_lifecycle_epoch: collaborationIssueFixture.lifecycle_epoch,
  expected_participant_epoch: "1", expected_account_epoch: "1",
};
/** Spread over `collaborationIssueFixture`: a detail whose viewer may write and may open source children. */
export const stageIssueContextFixture = {
  write_context: { participant_epoch: "1", account_epoch: "1" },
  ordinary_children: { items: [{ id: stageIds.child1, identifier: "SRC-12", title: "Existing checkout child", can_open: true }], source_workspace_slug: "source" },
};

export const stageConfirmations = {
  revise_policy: { action: "revise_policy", project_id: stageIds.project, policy_revision: "1", enabled: false },
  configure_roles: { action: "configure_roles", issue_id: stageIds.issue, role_revision: "1" },
  create_proposal: { action: "create_proposal", issue_id: stageIds.issue, proposal_id: stageIds.proposal, candidate_id: stageIds.candidate1, version: "1", content_digest: stageDigests.content1 },
  revise_proposal: { action: "revise_proposal", issue_id: stageIds.issue, proposal_id: stageIds.proposal, candidate_id: stageIds.candidate2, version: "2",
    content_digest: stageDigests.content2, parent_candidate_id: stageIds.candidate1 },
  decide_proposal: { action: "decide_proposal", issue_id: stageIds.issue, proposal_id: stageIds.proposal, candidate_id: stageIds.candidate2, version: "2",
    content_digest: stageDigests.content2, decision: "accepted", decision_id: stageIds.decision, role_revision: "1", policy_revision: "1" },
  confirm_scope: { action: "confirm_scope", issue_id: stageIds.issue, decision_id: stageIds.scopeDecision, scope_revision: "1", lifecycle_epoch: "1" },
  create_code_candidate: { action: "create_code_candidate", issue_id: stageIds.issue, candidate_id: stageIds.candidate1, version: "1", content_digest: stageDigests.content1, parent_candidate_id: null },
  decide_stage: { action: "decide_stage", issue_id: stageIds.issue, step: "qa", decision: "approved", candidate_id: stageIds.candidate1, version: "1",
    content_digest: stageDigests.content1, decision_id: stageIds.decision, role_revision: "1", policy_revision: "1", expires_at: "2026-11-21T00:00:00Z" },
  revoke_decision: { action: "revoke_decision", issue_id: stageIds.issue, decision_id: stageIds.decision, revocation_id: id(0x331) },
  advance_stage: { action: "advance_stage", issue_id: stageIds.issue, stage: "qa", stage_revision: "2" },
} as const;

export const stageCodeCandidateRefFixture = { candidate_id: stageIds.candidate1, version: "1", content_digest: stageDigests.content1 };
export const stageCodeEvidenceFixture = { ci_digest: stageDigests.content2, test_report_id: null, build: null, deployment_id: null, rollback_target_digest: null };
export const stageCodeViewFixture = {
  protocol_version: 1, issue_id: stageIds.issue, checked_at: "2026-11-14T03:00:00Z",
  candidate: {
    ...stageCodeCandidateRefFixture, parent_candidate_id: null, lifecycle_epoch: "1", state: "current", submitted_by: stageIds.implementer,
    implementer_user_id: stageIds.implementer, role_revision: "1", created_at: "2026-11-14T02:00:00Z",
    baseline: { requirement_digest: stageDigests.content1, child_issue_id: stageIds.child1, child_semantic_version: "1",
      pull_request: { ...stagePullRequestFixture, trust_revision: "1" }, artifact: null },
  },
  observed: { pull_request: { state: "current", head_sha: stagePullRequestFixture.head_sha },
    ci: { state: "passed", ci_digest: stageDigests.content2, head_sha: stagePullRequestFixture.head_sha, observed_at: "2026-11-14T02:55:00Z", check_count: 2 } },
  test_reports: [], decisions: [], execution_facts: [],
  signable: ["code_review", "qa", "release", "rollback"].map((step) => ({ step, allowed: step === "code_review",
    max_expires_at: step === "code_review" ? "2026-11-14T03:25:00Z" : null, blockers: [] })),
  blocked_evidence: 0,
};
export const stageCodeReviewDecisionFixture = {
  decision_id: stageIds.decision, step: "code_review", decision: "approved", state: "valid", actor_user_id: stageIds.reviewer, role: "code_reviewer",
  role_revision: "1", policy_revision: "1", created_at: "2026-11-14T03:00:00Z", expires_at: "2026-11-14T03:25:00Z", reason: "Reviewed exact candidate",
  evidence: stageCodeEvidenceFixture, depends_on: [], revocation: null,
};
export const stageTestReportSummaryFixture = { report_id: id(0x801), run_id: stageIds.run, attempt: "1", source_kind: "runtime_reported",
  reported_result: "passed", applicability: "usable", case_count: 3, failed_case_count: 0, received_at: "2026-11-14T03:02:00Z" };

/** Synthetic wire evidence for the frozen 2c contract; no provider is invoked. */
export const stageExecutionIds = { intent: id(0xa01), fact: id(0xa02), observation: id(0xa03), qa: id(0xa04), release: id(0xa05) };
export const stageExecutionTargetFixture = { environment: "staging", target_digest: "d".repeat(64), source_kind: "integration" };
export const stageExecutionBuildFixture = { artifact_digest: "c".repeat(64), environment: stageExecutionTargetFixture.environment, target_digest: stageExecutionTargetFixture.target_digest };
export const stageExecutionQADecisionFixture = { ...stageCodeReviewDecisionFixture, decision_id: stageExecutionIds.qa, step: "qa", role: "qa_approver", actor_user_id: stageIds.qa,
  evidence: { ...stageCodeEvidenceFixture, ci_digest: null, test_report_id: stageTestReportSummaryFixture.report_id }, depends_on: [stageIds.decision] };
export const stageExecutionReleaseDecisionFixture = { ...stageCodeReviewDecisionFixture, decision_id: stageExecutionIds.release, step: "release", role: "release_approver", actor_user_id: stageIds.release,
  evidence: { ...stageCodeEvidenceFixture, ci_digest: null, build: stageExecutionBuildFixture }, depends_on: [stageExecutionIds.qa] };
export const stageExecutionFactFixture = { fact_id: stageExecutionIds.fact, intent_id: stageExecutionIds.intent, kind: "deployment", source_kind: "integration",
  environment: stageExecutionTargetFixture.environment, target_digest: stageExecutionTargetFixture.target_digest, operation_id: "deploy/native-42", result: "succeeded",
  artifact_digest: stageExecutionBuildFixture.artifact_digest, previous_artifact_digest: "b".repeat(64), approval_decision_id: stageExecutionIds.release, observed_at: "2026-11-14T03:04:00Z" };
export const stageExecutionOperationFixture = { intent_id: stageExecutionIds.intent, kind: "deployment", approval_decision_id: stageExecutionIds.release,
  environment: stageExecutionTargetFixture.environment, target_digest: stageExecutionTargetFixture.target_digest, artifact_digest: stageExecutionBuildFixture.artifact_digest,
  state: "succeeded", external_operation_id: stageExecutionFactFixture.operation_id, fact_id: stageExecutionIds.fact, created_at: "2026-11-14T03:03:00Z" };
export const stageRollbackObservationFixture = { observation_id: stageExecutionIds.observation, version: "1", observation_digest: "f".repeat(64), deployment_id: stageExecutionIds.fact,
  environment: stageExecutionTargetFixture.environment, target_digest: stageExecutionTargetFixture.target_digest, artifact_digest: stageExecutionBuildFixture.artifact_digest,
  condition: "anomaly", state: "current", window_start: "2026-11-14T03:03:00Z", window_end: "2026-11-14T03:04:00Z", observed_at: "2026-11-14T03:04:00Z", valid_until: "2026-11-14T03:20:00Z", summary: "Synthetic anomaly from the registered test rule" };
export const stageExecutionViewFixture = { ...stageCodeViewFixture,
  candidate: { ...stageCodeViewFixture.candidate, baseline: { ...stageCodeViewFixture.candidate.baseline,
    artifact: { approval_id: stageIds.proposal, artifact_digest: stageExecutionBuildFixture.artifact_digest } } },
  test_reports: [stageTestReportSummaryFixture], decisions: [stageExecutionReleaseDecisionFixture, stageExecutionQADecisionFixture, stageCodeReviewDecisionFixture],
  execution_targets: [stageExecutionTargetFixture], execution_operations: [stageExecutionOperationFixture], execution_facts: [stageExecutionFactFixture], rollback_observations: [stageRollbackObservationFixture],
  signable: stageCodeViewFixture.signable.map((entry) => ({ ...entry, allowed: true, max_expires_at: "2026-11-14T03:20:00Z" })),
};
