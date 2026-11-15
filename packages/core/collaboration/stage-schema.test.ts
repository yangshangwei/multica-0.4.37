// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  CollaborationStageConfirmationSchema, CollaborationStageRequestSchemas, COLLABORATION_STAGE_ROUTE_UNAVAILABLE, COLLABORATION_STAGE_SUCCESS_CODES,
  classifyCollaborationStageError, classifyCollaborationStageResponse, collaborationStageBodyLimit, collaborationStageRefusalStatus, collaborationStageRoleConflicts,
  matchesCollaborationStageConfirmation, collaborationStageConfirmationIdentity, parseCollaborationStagePolicy, parseCollaborationStageProposalDetail, parseCollaborationStageProposalPage, parseCollaborationStageView,
  type CollaborationStageFrozenRequest, type CollaborationStageWriteResult,
} from "./stage-schema";
import {
  stageCandidateV2Fixture, stageConfirmations, stageDecisionFixture, stageDigests, stageIds, stageObservationsFixture, stagePolicyFixture, stagePolicyVirtualFixture,
  stageProposalAcceptedDetailFixture, stageProposalContentFixture, stageProposalDetailFixture, stageProposalListItemFixture, stageProposalPageFixture,
  stageRolesFixture, stageScopeFixture, stageViewManagedFixture, stageViewNeedsConfirmationFixture, stageViewUnmanagedFixture, stageChildFixture, stagePullRequestFixture,
} from "./stage-test-fixtures";

import { collaborationIssueFixture } from "./test-fixtures";

const ZERO = "00000000-0000-0000-0000-000000000000";

it("keeps the self-contained stage fixtures aligned with the collaboration issue fixture", () => {
  expect(stageIds.issue).toBe(collaborationIssueFixture.id);
  expect(stageIds.workspace).toBe(collaborationIssueFixture.source_workspace.id);
  expect(stageViewManagedFixture.collaboration_revision).toBe(collaborationIssueFixture.collaboration_revision);
  expect(stageViewManagedFixture.lifecycle_epoch).toBe(collaborationIssueFixture.lifecycle_epoch);
  expect(stageObservationsFixture.expected_collaboration_revision).toBe(collaborationIssueFixture.collaboration_revision);
});
const roles = {
  business_owner_user_id: stageIds.owner, implementer_user_id: stageIds.implementer, product_approver_user_id: stageIds.product,
  code_reviewer_user_id: stageIds.reviewer, qa_approver_user_id: stageIds.qa, release_approver_user_id: stageIds.release,
};
const candidate2 = { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 };
const candidate1 = { candidate_id: stageIds.candidate1, version: "1", content_digest: stageDigests.content1 };

describe("P1 project policy", () => {
  it("parses a revision and the virtual disabled revision", () => {
    expect(parseCollaborationStagePolicy(stagePolicyFixture)).toMatchObject({ policy_revision: "1", enabled: true, activation_available: true });
    expect(parseCollaborationStagePolicy(stagePolicyVirtualFixture)).toMatchObject({ policy_revision: "0", enabled: false, created_by: null });
  });
  it("keeps a missing activation hint unknown instead of offering it", () => {
    const { activation_available: _omitted, ...older } = stagePolicyFixture;
    expect(parseCollaborationStagePolicy(older)?.activation_available).toBeUndefined();
  });
  it.each([
    ["another contract", { ...stagePolicyFixture, contract: "collaboration-stage-v2" }],
    ["a numeric revision", { ...stagePolicyFixture, policy_revision: 1 }],
    ["a leading-zero revision", { ...stagePolicyFixture, policy_revision: "01" }],
    ["a revision above int64", { ...stagePolicyFixture, policy_revision: "9223372036854775808" }],
    ["a truthy enabled", { ...stagePolicyFixture, enabled: "true" }],
    ["an enabled virtual revision", { ...stagePolicyVirtualFixture, enabled: true }],
    ["a zero project", { ...stagePolicyFixture, project_id: ZERO }],
    ["a string activation hint", { ...stagePolicyFixture, activation_available: "true" }],
    ["nothing", undefined],
  ])("refuses %s", (_name, value) => { expect(parseCollaborationStagePolicy(value)).toBeNull(); });
});

describe("R1 stage view", () => {
  const withChild = (change: Record<string, unknown>) => ({ ...stageViewManagedFixture, children: { items: [{ ...stageChildFixture, ...change }], truncated: false } });
  it("retains descendant depth and confirmed code separately from scope and execution approval", () => {
    const code = { pull_request: stagePullRequestFixture };
    const view = parseCollaborationStageView(withChild({ depth: 2, code, code_state: "changed", pull_requests: [{ ...stagePullRequestFixture, head_sha: "b".repeat(40) }] }))!;
    expect(view.children.items[0]).toMatchObject({ depth: 2, code, code_state: "changed", execution_gated: false });
    expect(view.scope_state).toBe("current");
    expect(parseCollaborationStageView(withChild({ attached: false, depth: null }))?.children.items[0]?.depth).toBeNull();
    expect(parseCollaborationStageView(withChild({ code_state: "future" }))?.children.items[0]?.code_state).toBe("unknown");
  });
  it.each([
    ["absent depth", { depth: undefined }], ["zero depth", { depth: 0 }], ["over-depth", { depth: 65 }],
    ["attached without depth", { depth: null }], ["detached with depth", { attached: false }],
    ["absent code", { code: undefined }], ["absent code state", { code_state: undefined }],
    ["absent pull requests", { pull_requests: undefined }], ["null pull requests", { pull_requests: null }],
    ["short observed head", { pull_requests: [{ ...stagePullRequestFixture, head_sha: "abc" }] }],
    ["uppercase observed head", { pull_requests: [{ ...stagePullRequestFixture, head_sha: "A".repeat(40) }] }],
    ["unknown provider", { pull_requests: [{ ...stagePullRequestFixture, provider: "other" }] }],
    ["missing code reference", { code: {} }], ["current without code", { code_state: "current" }],
    ["duplicate pull request", { pull_requests: [stagePullRequestFixture, stagePullRequestFixture] }],
  ])("rejects a child with %s", (_name, change) => { expect(parseCollaborationStageView(withChild(change))).toBeNull(); });
  it("parses the first-enablement, managed and unmanaged projections", () => {
    expect(parseCollaborationStageView(stageViewNeedsConfirmationFixture)).toMatchObject({ managed: true, needs_confirmation: true, missing: ["roles", "scope"], roles: null });
    const managed = parseCollaborationStageView(stageViewManagedFixture)!;
    expect(managed.roles?.implementer?.name).toBe("Ivan Implementer");
    expect(managed.children.items.map((child) => child.origin)).toEqual(["confirmed", "materialized"]);
    expect(parseCollaborationStageView(stageViewUnmanagedFixture)).toMatchObject({ managed: false, stage: null, policy: { state: "disabled" } });
  });
  it("normalizes uppercase identifiers and keeps exact decimal versions above 2^53", () => {
    const view = parseCollaborationStageView({ ...stageViewNeedsConfirmationFixture, issue_id: stageIds.issue.toUpperCase() })!;
    expect(view.issue_id).toBe(stageIds.issue);
    expect(view.collaboration_revision).toBe("9007199254740993");
  });
  it("turns unfamiliar enum strings into unknown instead of a known state", () => {
    const view = parseCollaborationStageView({ ...stageViewManagedFixture, stage: { current: "hotfix", stage_revision: "4" }, scope_state: "expired_elsewhere",
      children: { items: [{ ...stageViewManagedFixture.children.items[0], origin: "imported" }], truncated: false } })!;
    expect(view.stage?.current).toBe("unknown");
    expect(view.scope_state).toBe("unknown");
    expect(view.children.items[0]?.origin).toBe("unknown");
    expect(parseCollaborationStageView({ ...stageViewManagedFixture, policy: { ...stageViewManagedFixture.policy, state: "paused" } })?.policy.state).toBe("unknown");
  });
  it("projects a role whose account no longer resolves as null and keeps `external` a strict boolean (contract 13.1)", () => {
    const view = parseCollaborationStageView({ ...stageViewManagedFixture, roles: { ...stageRolesFixture, qa_approver: null, release_approver: { ...stageRolesFixture.release_approver, external: true } } })!;
    expect(view.roles?.qa_approver).toBeNull();
    expect(view.roles?.release_approver?.external).toBe(true);
    expect(parseCollaborationStageView({ ...stageViewManagedFixture, roles: { ...stageRolesFixture, qa_approver: { ...stageRolesFixture.qa_approver, external: null } } })).toBeNull();
    const { qa_approver: _omitted, ...withoutRole } = stageRolesFixture;
    expect(parseCollaborationStageView({ ...stageViewManagedFixture, roles: withoutRole })).toBeNull();
  });
  it("treats absent or malformed capability hints as not offered", () => {
    const view = parseCollaborationStageView({ ...stageViewManagedFixture, capabilities: { configure_roles: "yes", create_proposal: 1 } })!;
    expect(view.capabilities).toEqual({ configure_roles: false, create_proposal: false, decide_proposal: false, confirm_scope: false });
  });
  it.each([
    ["protocol 2", { ...stageViewManagedFixture, protocol_version: 2 }],
    ["a numeric collaboration revision", { ...stageViewManagedFixture, collaboration_revision: 12 }],
    ["a non-string stage", { ...stageViewManagedFixture, stage: { current: 3, stage_revision: "1" } }],
    ["managed while the policy is disabled", { ...stageViewManagedFixture, policy: { ...stageViewManagedFixture.policy, state: "disabled" } }],
    ["unmanaged while the policy is enabled", { ...stageViewUnmanagedFixture, policy: { ...stageViewUnmanagedFixture.policy, state: "enabled" } }],
    ["needs_confirmation contradicting missing", { ...stageViewManagedFixture, needs_confirmation: true }],
    ["missing in the wrong order", { ...stageViewNeedsConfirmationFixture, missing: ["scope", "roles"] }],
    ["an unknown missing item", { ...stageViewNeedsConfirmationFixture, missing: ["roles", "ci"] }],
    ["a managed root without a stage", { ...stageViewManagedFixture, stage: null }],
    ["an unmanaged root with a stage", { ...stageViewUnmanagedFixture, stage: { current: "qa", stage_revision: "1" } }],
    ["an unmanaged root that needs confirmation", { ...stageViewUnmanagedFixture, needs_confirmation: true, missing: ["roles"] }],
    ["roles under another revision", { ...stageViewManagedFixture, role_revision: "2" }],
    ["a role revision without roles", { ...stageViewNeedsConfirmationFixture, role_revision: "3" }],
    ["a scope under another revision", { ...stageViewManagedFixture, scope_revision: "4" }],
    ["a current scope state without a confirmation", { ...stageViewManagedFixture, scope: null, scope_revision: "0" }],
    ["a confirmation with scope state none", { ...stageViewManagedFixture, scope_state: "none", missing: ["scope"], needs_confirmation: true }],
    ["a stale scope that is not reported missing", { ...stageViewManagedFixture, scope_state: "signer_changed" }],
    ["a current scope reported missing", { ...stageViewManagedFixture, missing: ["scope"], needs_confirmation: true }],
    ["a duplicated child", { ...stageViewManagedFixture, children: { items: [stageViewManagedFixture.children.items[0], stageViewManagedFixture.children.items[0]], truncated: false } }],
    ["a child without a content digest", { ...stageViewManagedFixture, children: { items: [{ ...stageViewManagedFixture.children.items[0], content_digest: "abc" }], truncated: false } }],
    ["a role holder without a UUID", { ...stageViewManagedFixture, roles: { ...stageRolesFixture, qa_approver: { ...stageRolesFixture.qa_approver, user_id: "quinn" } } }],
    ["a scope for a zero decision", { ...stageViewManagedFixture, scope: { ...stageScopeFixture, decision_id: ZERO } }],
    ["a missing capabilities object", { ...stageViewManagedFixture, capabilities: undefined }],
    ["an empty object", {}],
  ])("refuses %s", (_name, value) => { expect(parseCollaborationStageView(value)).toBeNull(); });
});

describe("R2 proposal list and R3 proposal detail", () => {
  it("parses a page and keeps the newest-version decision", () => {
    expect(parseCollaborationStageProposalPage(stageProposalPageFixture)?.items[0]).toMatchObject({ state: "pending", current: { version: "2", stale: false }, decision: null });
    const decided = { ...stageProposalPageFixture, items: [{ ...stageProposalListItemFixture, state: "accepted", decision: stageDecisionFixture }] };
    expect(parseCollaborationStageProposalPage(decided)?.items[0]?.decision?.accepted_with_modification).toBe(true);
  });
  it.each([
    ["a repeated proposal", { ...stageProposalPageFixture, items: [stageProposalListItemFixture, stageProposalListItemFixture] }],
    ["more than fifty items", { ...stageProposalPageFixture, items: Array.from({ length: 51 }, (_unused, index) => ({ ...stageProposalListItemFixture, proposal_id: `00000000-0000-4000-8000-0000000009${String(index).padStart(2, "0")}` })) }],
    ["a decision for another version", { ...stageProposalPageFixture, items: [{ ...stageProposalListItemFixture, decision: { ...stageDecisionFixture, candidate_id: stageIds.candidate1 } }] }],
    ["an empty cursor", { ...stageProposalPageFixture, next_cursor: "" }],
    ["a numeric version", { ...stageProposalPageFixture, items: [{ ...stageProposalListItemFixture, current: { ...stageProposalListItemFixture.current, version: 2 } }] }],
    ["a missing item list", { issue_id: stageIds.issue, next_cursor: null }],
  ])("refuses a page with %s", (_name, value) => { expect(parseCollaborationStageProposalPage(value)).toBeNull(); });

  it("parses versions, server-stated provenance and the materialized mapping", () => {
    const detail = parseCollaborationStageProposalDetail(stageProposalAcceptedDetailFixture)!;
    expect(detail.versions.map((version) => version.version)).toEqual(["2", "1"]);
    expect(detail.versions[1]?.source).toMatchObject({ kind: "comment", author_type: "agent", author_id: stageIds.agent });
    expect(detail.children.map((child) => child.ordinal)).toEqual([1, 2]);
    expect(parseCollaborationStageProposalDetail({ ...stageProposalDetailFixture, versions: [{ ...stageCandidateV2Fixture, source: { kind: "webhook", id: "x" } }] })?.versions[0]?.source).toEqual({ kind: "unknown" });
  });
  it.each([
    ["versions oldest first", { ...stageProposalDetailFixture, versions: [...stageProposalDetailFixture.versions].reverse() }],
    ["no version", { ...stageProposalDetailFixture, versions: [] }],
    ["children on a pending proposal", { ...stageProposalDetailFixture, children: stageProposalAcceptedDetailFixture.children }],
    ["a repeated ordinal", { ...stageProposalAcceptedDetailFixture, children: [stageProposalAcceptedDetailFixture.children[0], { ...stageProposalAcceptedDetailFixture.children[1], ordinal: 1 }] }],
    ["a repeated decision", { ...stageProposalAcceptedDetailFixture, decisions: [stageDecisionFixture, stageDecisionFixture] }],
    ["content without children", { ...stageProposalDetailFixture, versions: [{ ...stageCandidateV2Fixture, content: { summary: "x", children: [] } }] }],
  ])("refuses a detail with %s", (_name, value) => { expect(parseCollaborationStageProposalDetail(value)).toBeNull(); });
});

describe("increment-1 request bodies", () => {
  const schemas = CollaborationStageRequestSchemas;
  it("emits the exact wire bodies of contract sections 4.2 and 7", () => {
    expect(schemas.configure_roles.parse({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "0", roles }))
      .toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "0", roles });
    expect(schemas.create_proposal.parse({ ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture,
      source: { kind: "comment", comment_id: stageIds.comment.toUpperCase(), comment_revision: "3" } }).source)
      .toEqual({ kind: "comment", comment_id: stageIds.comment, comment_revision: "3" });
    expect(schemas.decide_proposal.parse({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", candidate: candidate2, decision: "accept", reason: "" }).reason).toBe("");
    expect(schemas.confirm_scope.parse({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_scope_revision: "0", children: [], reason: "" }).children).toEqual([]);
    expect(schemas.revise_policy.parse({ expected_policy_revision: "0", enabled: false, reason: "  Pause the pilot  " })).toEqual({ expected_policy_revision: "0", enabled: false, reason: "Pause the pilot" });
  });
  it("trims titles and refuses content outside the contract limits", () => {
    const content = (children: unknown[], summary = "s") => schemas.create_proposal.safeParse({ ...stageObservationsFixture, expected_policy_revision: "1", content: { summary, children }, source: { kind: "manual" } });
    const child = { title: " Refund API ", description: "", priority: "none" };
    expect(content([child]).data?.content.children[0]?.title).toBe("Refund API");
    expect(content([]).success).toBe(false);
    expect(content(Array.from({ length: 21 }, () => child)).success).toBe(false);
    expect(content([{ ...child, title: "   " }]).success).toBe(false);
    expect(content([{ ...child, title: "界".repeat(500) }]).success).toBe(true);
    expect(content([{ ...child, title: "界".repeat(501) }]).success).toBe(false);
    expect(content([{ ...child, description: "界".repeat(21_846) }]).success).toBe(false);
    expect(content([{ ...child, priority: "critical" }]).success).toBe(false);
    expect(content([{ ...child, assignee_id: stageIds.owner }]).success).toBe(false);
    expect(content([child], "s".repeat(16_385)).success).toBe(false);
    expect(content([{ ...child, description: `a${String.fromCharCode(0)}b` }]).success).toBe(false);
    expect(content([{ ...child, description: String.fromCharCode(0xd800) }]).success).toBe(false);
    expect(content(Array.from({ length: 9 }, () => ({ ...child, description: "d".repeat(65_536) }))).success).toBe(false);
  });
  it("refuses a source that states an author, mixes shapes or names nothing", () => {
    const source = (value: unknown) => schemas.create_proposal.safeParse({ ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture, source: value }).success;
    expect(source({ kind: "manual" })).toBe(true);
    expect(source({ kind: "run", run_id: stageIds.run })).toBe(true);
    expect(source({ kind: "run", run_id: stageIds.run, agent_id: stageIds.agent })).toBe(false);
    expect(source({ kind: "comment", comment_id: stageIds.comment, comment_revision: "3", author_type: "agent" })).toBe(false);
    expect(source({ kind: "comment", comment_id: stageIds.comment })).toBe(false);
    expect(source({ kind: "manual", run_id: stageIds.run })).toBe(false);
    expect(source({ kind: "agent" })).toBe(false);
  });
  it("requires a reason to reject and bounds every reason by characters", () => {
    const decide = (decision: string, reason: unknown) => schemas.decide_proposal.safeParse({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", candidate: candidate2, decision, reason }).success;
    expect(decide("reject", "Out of scope")).toBe(true);
    expect(decide("reject", "   ")).toBe(false);
    expect(decide("accept", undefined)).toBe(false);
    expect(decide("accept", "界".repeat(2000))).toBe(true);
    expect(decide("accept", "界".repeat(2001))).toBe(false);
    expect(decide("approve", "x")).toBe(false);
    expect(schemas.revise_policy.safeParse({ expected_policy_revision: "0", enabled: true, reason: " " }).success).toBe(false);
    expect(schemas.revise_policy.safeParse({ expected_policy_revision: "0", enabled: true, reason: "r".repeat(1001) }).success).toBe(false);
    expect(schemas.revise_policy.safeParse({ expected_policy_revision: "0", enabled: "true", reason: "r" }).success).toBe(false);
  });
  it("refuses malformed observations, unknown fields, zero identifiers and role conflicts before a key is spent", () => {
    const base = { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "0", roles };
    expect(schemas.configure_roles.safeParse({ ...base, expected_collaboration_revision: "0" }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, expected_policy_revision: "0" }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, expected_account_epoch: 1 }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, note: "extra" }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, roles: { ...roles, qa_approver_user_id: ZERO } }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, roles: { ...roles, code_reviewer_user_id: stageIds.implementer } }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, roles: { ...roles, release_approver_user_id: stageIds.qa } }).success).toBe(false);
    expect(schemas.configure_roles.safeParse({ ...base, roles: { ...roles, business_owner_user_id: stageIds.product, release_approver_user_id: stageIds.product } }).success).toBe(true);
    expect(collaborationStageRoleConflicts({ implementer_user_id: stageIds.implementer, code_reviewer_user_id: stageIds.implementer.toUpperCase(),
      qa_approver_user_id: stageIds.qa, release_approver_user_id: stageIds.qa })).toEqual(["self_review", "qa_release"]);
    expect(collaborationStageRoleConflicts({ implementer_user_id: "", code_reviewer_user_id: "" })).toEqual([]);
    const scope = { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_scope_revision: "0", reason: "" };
    const child = { issue_id: stageIds.child1, content_digest: stageDigests.child1, code: null };
    expect(schemas.confirm_scope.safeParse({ ...scope, children: [child, child] }).success).toBe(false);
    expect(schemas.confirm_scope.safeParse({ ...scope, children: [{ ...child, content_digest: stageDigests.child1.toUpperCase() }] }).success).toBe(false);
    expect(schemas.confirm_scope.safeParse({ ...scope, children: Array.from({ length: 201 }, (_unused, index) => ({ ...child, issue_id: `00000000-0000-4000-8000-00000000a${String(index).padStart(3, "0")}` })) }).success).toBe(false);
  });
  it("uses the per-action body limits of contract 2.8", () => {
    expect(collaborationStageBodyLimit("create_proposal")).toBe(2 * 1024 * 1024);
    expect(collaborationStageBodyLimit("revise_proposal")).toBe(2 * 1024 * 1024);
    for (const action of ["configure_roles", "decide_proposal", "confirm_scope", "revise_policy"] as const) expect(collaborationStageBodyLimit(action)).toBe(64 * 1024);
  });
});

describe("A5 explicit code reference (contract 13.3)", () => {
  const scope = { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_scope_revision: "0", reason: "" };
  const child = { issue_id: stageIds.child1, content_digest: stageDigests.child1 };
  const parse = (code: unknown) => CollaborationStageRequestSchemas.confirm_scope.safeParse({ ...scope, children: [{ ...child, code }] }).success;
  it("accepts explicit none, full SHA-1 and full SHA-256 references", () => {
    expect(parse(null)).toBe(true);
    expect(parse({ pull_request: stagePullRequestFixture })).toBe(true);
    expect(parse({ pull_request: { ...stagePullRequestFixture, provider: "vcs", head_sha: "b".repeat(64) } })).toBe(true);
    expect(CollaborationStageRequestSchemas.confirm_scope.safeParse({ ...scope, children: [] }).success).toBe(true);
    expect(CollaborationStageRequestSchemas.confirm_scope.safeParse({ ...scope, children: [child] }).success).toBe(false);
  });
  it.each([
    ["missing", undefined], ["empty object", {}], ["extra code field", { pull_request: stagePullRequestFixture, verified: true }],
    ["extra PR field", { pull_request: { ...stagePullRequestFixture, verified: true } }],
    ["missing provider", { pull_request: { pull_request_id: stagePullRequestFixture.pull_request_id, head_sha: stagePullRequestFixture.head_sha } }],
    ["unknown provider", { pull_request: { ...stagePullRequestFixture, provider: "other" } }],
    ["short head", { pull_request: { ...stagePullRequestFixture, head_sha: "abc1234" } }],
    ["uppercase head", { pull_request: { ...stagePullRequestFixture, head_sha: "A".repeat(40) } }],
    ["zero PR id", { pull_request: { ...stagePullRequestFixture, pull_request_id: ZERO } }],
  ])("rejects %s code", (_name, code) => { expect(parse(code)).toBe(false); });
  it("recognizes the exact stored code-unavailable refusal status", () => {
    expect(collaborationStageRefusalStatus("stage_scope_code_unavailable")).toBe(409);
  });
});

describe("stage_confirmation closed union (contract section 11)", () => {
  it("parses all ten frozen variants and maps each to its success codes", () => {
    for (const [action, confirmation] of Object.entries(stageConfirmations)) {
      expect(CollaborationStageConfirmationSchema.safeParse(confirmation).success, action).toBe(true);
      expect(COLLABORATION_STAGE_SUCCESS_CODES[action as keyof typeof stageConfirmations].length, action).toBeGreaterThan(0);
    }
    expect(Object.keys(COLLABORATION_STAGE_SUCCESS_CODES).sort()).toEqual(Object.keys(stageConfirmations).sort());
  });
  it.each([
    ["an unknown action", { action: "approve_everything", issue_id: stageIds.issue }],
    ["a field of another variant", { ...stageConfirmations.configure_roles, decision_id: stageIds.decision }],
    ["a null placeholder", { ...stageConfirmations.configure_roles, candidate_id: null }],
    ["a zero server identifier", { ...stageConfirmations.create_proposal, candidate_id: ZERO }],
    ["a policy confirmation with an issue", { ...stageConfirmations.revise_policy, issue_id: stageIds.issue }],
    ["a missing variant field", { action: "confirm_scope", issue_id: stageIds.issue, decision_id: stageIds.scopeDecision, scope_revision: "1" }],
    ["an unknown decision", { ...stageConfirmations.decide_proposal, decision: "approved" }],
    ["a numeric version", { ...stageConfirmations.create_proposal, version: 1 }],
    ["prose", { ...stageConfirmations.decide_proposal, reason: "PRIVATE reason" }],
  ])("refuses %s", (_name, value) => { expect(CollaborationStageConfirmationSchema.safeParse(value).success).toBe(false); });
});

describe("write and recovery classification (contract 3.2-3.5, 4.2, 4.3)", () => {
  const ok = (code: string, confirmation: unknown) => ({ code, collaboration_revision: "13", stage_confirmation: confirmation });
  it("accepts a success only with its own variant, code and owner", () => {
    expect(classifyCollaborationStageResponse(200, ok("stage_roles_configured", stageConfirmations.configure_roles), "write", "issue"))
      .toEqual({ type: "success", code: "stage_roles_configured", collaborationRevision: "13", confirmation: stageConfirmations.configure_roles });
    expect(classifyCollaborationStageResponse(200, { code: "stage_policy_revised", stage_confirmation: stageConfirmations.revise_policy }, "write", "policy"))
      .toMatchObject({ type: "success", collaborationRevision: null });
    for (const body of [
      ok("stage_proposal_created", stageConfirmations.configure_roles),
      ok("stage_policy_revised", stageConfirmations.revise_policy),
      { ...ok("stage_roles_configured", stageConfirmations.configure_roles), status: 409 },
      { ...ok("stage_roles_configured", stageConfirmations.configure_roles), collaboration_revision: 13 },
      ok("stage_roles_configured", { ...stageConfirmations.configure_roles, role_revision: "01" }),
      { code: "stage_roles_configured", collaboration_revision: "13" },
      undefined, null, "ok", [],
    ]) expect(classifyCollaborationStageResponse(200, body, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageResponse(200, { code: "stage_policy_revised", stage_confirmation: stageConfirmations.configure_roles }, "write", "policy")).toMatchObject({ type: "unknown" });
    expect(classifyCollaborationStageResponse(201, ok("stage_roles_configured", stageConfirmations.configure_roles), "write", "issue")).toMatchObject({ type: "unknown" });
  });
  it("reads a stored refusal from a recovery 200 only, and never trusts a contradictory status", () => {
    const refusal = { code: "stage_candidate_changed", collaboration_revision: "12", status: 409 };
    expect(classifyCollaborationStageResponse(200, refusal, "recovery", "issue")).toEqual({ type: "refusal", stored: true, status: 409, code: "stage_candidate_changed", collaborationRevision: "12" });
    expect(classifyCollaborationStageResponse(200, refusal, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageResponse(200, { ...refusal, status: 403 }, "recovery", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageResponse(200, { ...refusal, status: 200 }, "recovery", "issue")).toMatchObject({ type: "unknown" });
    expect(classifyCollaborationStageResponse(200, { ...refusal, code: "stage_future_refusal", status: 422 }, "recovery", "issue")).toMatchObject({ type: "refusal", stored: true, status: 422 });
  });
  it("settles a committed refusal by its own envelope, including role refusals that are not access loss", () => {
    for (const [code, status] of [["stage_role_required", 403], ["stage_configuration_denied", 403], ["stage_source_creation_denied", 403], ["forbidden", 403],
      ["stage_proposal_not_found", 404], ["stale_revision", 409], ["stage_child_creation_refused", 409]] as const) {
      expect(collaborationStageRefusalStatus(code)).toBe(status);
      expect(classifyCollaborationStageError(status, { code, error: code, status, collaboration_revision: "12" }, "write", "issue"))
        .toEqual({ type: "refusal", stored: true, status, code, collaborationRevision: "12" });
    }
    expect(classifyCollaborationStageError(409, { code: "stage_role_required", error: "x", status: 409, collaboration_revision: "12" }, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageError(409, { code: "stale_revision", status: 403, collaboration_revision: "12" }, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageError(409, { code: "stale_revision", collaboration_revision: "12" }, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageError(409, { code: "stale_revision", status: 409, collaboration_revision: "12", stage_confirmation: stageConfirmations.configure_roles }, "write", "issue")).toMatchObject({ type: "unknown" });
    expect(collaborationStageRefusalStatus("stage_future_refusal")).toBeNull();
  });
  it("separates not-written refusals, unknown outcomes and access answers", () => {
    for (const [code, status] of [["invalid_stage_request", 400], ["invalid_version", 400], ["idempotency_conflict", 409], ["request_too_large", 413], ["json_required", 415],
      ["collaboration_busy", 503], ["stage_schema_unavailable", 503]] as const) {
      expect(classifyCollaborationStageError(status, { code, error: code }, "write", "issue")).toEqual({ type: "refusal", stored: false, status, code, collaborationRevision: null });
    }
    expect(classifyCollaborationStageError(503, { code: "collaboration_unavailable", error: "x" }, "write", "issue")).toEqual({ type: "unknown", reason: "unavailable", status: 503, code: "collaboration_unavailable" });
    expect(classifyCollaborationStageError(502, undefined, "write", "issue")).toMatchObject({ type: "unknown", reason: "unavailable" });
    expect(classifyCollaborationStageError(409, { code: "invalid_stage_request" }, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    for (const status of [404, 405]) expect(classifyCollaborationStageError(status, undefined, "write", "issue"))
      .toEqual({ type: "refusal", stored: false, status, code: COLLABORATION_STAGE_ROUTE_UNAVAILABLE, collaborationRevision: null });
    expect(classifyCollaborationStageError(404, { code: "not_found", error: "not_found" }, "write", "issue")).toEqual({ type: "denied" });
    expect(classifyCollaborationStageError(403, { code: "forbidden", error: "forbidden" }, "write", "issue")).toEqual({ type: "denied" });
    expect(classifyCollaborationStageError(401, { error: "unauthorized" }, "write", "issue")).toEqual({ type: "denied" });
    // A coded 403/404 without the committed envelope proves neither an outcome nor access loss.
    expect(classifyCollaborationStageError(404, { code: "stage_proposal_not_found" }, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageError(403, { code: "stage_role_required" }, "write", "issue")).toMatchObject({ type: "unknown", reason: "malformed" });
  });
  it("keeps recovery read-only: incomplete, not found and malformed stay unsettled", () => {
    expect(classifyCollaborationStageError(409, { code: "operation_incomplete", error: "x" }, "recovery", "issue")).toEqual({ type: "incomplete" });
    expect(classifyCollaborationStageError(404, { code: "not_found", error: "x" }, "recovery", "issue")).toEqual({ type: "not_found" });
    expect(classifyCollaborationStageError(404, undefined, "recovery", "issue")).toEqual({ type: "not_found" });
    expect(classifyCollaborationStageError(404, { code: "stage_policy_operation_not_found" }, "recovery", "policy")).toEqual({ type: "not_found" });
    expect(classifyCollaborationStageError(409, { code: "operation_incomplete" }, "recovery", "policy")).toMatchObject({ type: "unknown" });
    expect(classifyCollaborationStageError(500, undefined, "recovery", "issue")).toMatchObject({ type: "unknown", reason: "unavailable" });
    expect(classifyCollaborationStageError(403, { code: "forbidden" }, "recovery", "issue")).toEqual({ type: "denied" });
  });
  it("classifies project policy refusals as never stored and the temporary gating as its own code", () => {
    for (const [code, status] of [["stage_policy_unavailable", 503], ["stage_policy_management_denied", 403], ["stage_policy_denied", 403], ["stage_policy_project_not_found", 404],
      ["stage_policy_revision_conflict", 409], ["stage_policy_busy", 409], ["invalid_stage_policy_request", 400], ["stage_schema_unavailable", 503]] as const) {
      expect(classifyCollaborationStageError(status, { code, error: code }, "write", "policy")).toEqual({ type: "refusal", stored: false, status, code, collaborationRevision: null });
    }
    expect(classifyCollaborationStageError(404, undefined, "write", "policy")).toMatchObject({ type: "refusal", stored: false, code: COLLABORATION_STAGE_ROUTE_UNAVAILABLE });
    expect(classifyCollaborationStageError(404, { code: "workspace_access_denied" }, "write", "policy")).toMatchObject({ type: "unknown" });
    expect(classifyCollaborationStageError(409, { code: "stage_policy_unavailable" }, "write", "policy")).toMatchObject({ type: "unknown", reason: "malformed" });
    expect(classifyCollaborationStageError(401, undefined, "write", "policy")).toEqual({ type: "denied" });
  });
});

describe("confirmation matching against the frozen request (contract 3.2 and per-action rules)", () => {
  const success = (code: string, confirmation: unknown): Extract<CollaborationStageWriteResult, { type: "success" }> =>
    ({ type: "success", code, collaborationRevision: "13", confirmation: CollaborationStageConfirmationSchema.parse(confirmation) });
  const schemas = CollaborationStageRequestSchemas;
  const requests: { [A in Exclude<CollaborationStageFrozenRequest["action"], "create_code_candidate" | "decide_stage" | "revoke_decision" | "advance_stage">]: Extract<CollaborationStageFrozenRequest, { action: A }> } = {
    configure_roles: { action: "configure_roles", issueId: stageIds.issue, body: schemas.configure_roles.parse({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "0", roles }) },
    create_proposal: { action: "create_proposal", issueId: stageIds.issue, body: schemas.create_proposal.parse({ ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture, source: { kind: "manual" } }) },
    revise_proposal: { action: "revise_proposal", issueId: stageIds.issue, proposalId: stageIds.proposal, body: schemas.revise_proposal.parse({ ...stageObservationsFixture, expected_policy_revision: "1",
      expected_candidate: candidate1, content: stageProposalContentFixture, source: { kind: "manual" } }) },
    decide_proposal: { action: "decide_proposal", issueId: stageIds.issue, proposalId: stageIds.proposal, body: schemas.decide_proposal.parse({ ...stageObservationsFixture, expected_policy_revision: "1",
      expected_role_revision: "1", candidate: candidate2, decision: "accept", reason: "" }) },
    confirm_scope: { action: "confirm_scope", issueId: stageIds.issue, body: schemas.confirm_scope.parse({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1",
      expected_scope_revision: "0", children: [], reason: "" }) },
    revise_policy: { action: "revise_policy", workspaceId: stageIds.workspace, projectId: stageIds.project, body: schemas.revise_policy.parse({ expected_policy_revision: "0", enabled: false, reason: "Pause" }) },
  };
  const codes = { configure_roles: "stage_roles_configured", create_proposal: "stage_proposal_created", revise_proposal: "stage_proposal_revised",
    decide_proposal: "stage_proposal_accepted", confirm_scope: "stage_scope_confirmed", revise_policy: "stage_policy_revised" } as const;

  it.each(Object.keys(requests) as (keyof typeof requests)[])("retires %s on its exact confirmation", (action) => {
    expect(matchesCollaborationStageConfirmation(requests[action], success(codes[action], stageConfirmations[action]))).toBe(true);
    expect(matchesCollaborationStageConfirmation(collaborationStageConfirmationIdentity(requests[action]), success(codes[action], stageConfirmations[action]))).toBe(true);
  });
  it("never retires a request with another action's or another owner's confirmation", () => {
    for (const action of Object.keys(requests) as (keyof typeof requests)[]) {
      for (const other of Object.keys(requests) as (keyof typeof requests)[]) {
        if (other === action) continue;
        expect(matchesCollaborationStageConfirmation(requests[action], success(codes[other], stageConfirmations[other])), `${action} <- ${other}`).toBe(false);
      }
    }
    const elsewhere = { ...stageConfirmations.configure_roles, issue_id: stageIds.child1 };
    expect(matchesCollaborationStageConfirmation(requests.configure_roles, success("stage_roles_configured", elsewhere))).toBe(false);
    expect(matchesCollaborationStageConfirmation(requests.revise_policy, success("stage_policy_revised", { ...stageConfirmations.revise_policy, project_id: stageIds.child1 }))).toBe(false);
  });
  it.each<[string, keyof typeof requests, string, Record<string, unknown>]>([
    ["an unchanged result under a different revision", "configure_roles", "stage_roles_unchanged", { role_revision: "1" }],
    ["a created proposal that is not version 1", "create_proposal", "stage_proposal_created", { version: "2" }],
    ["a revision under another proposal", "revise_proposal", "stage_proposal_revised", { proposal_id: stageIds.child1 }],
    ["a revision of another parent", "revise_proposal", "stage_proposal_revised", { parent_candidate_id: stageIds.candidate2 }],
    ["a revision that skips a version", "revise_proposal", "stage_proposal_revised", { version: "3" }],
    ["a decision rebound to a newer version", "decide_proposal", "stage_proposal_accepted", { version: "3" }],
    ["a decision for another candidate", "decide_proposal", "stage_proposal_accepted", { candidate_id: stageIds.candidate1 }],
    ["a decision for other content", "decide_proposal", "stage_proposal_accepted", { content_digest: stageDigests.content1 }],
    ["a rejection answering an accept", "decide_proposal", "stage_proposal_rejected", { decision: "rejected" }],
    ["an accept code with a rejected decision", "decide_proposal", "stage_proposal_accepted", { decision: "rejected" }],
    ["a decision under another role revision", "decide_proposal", "stage_proposal_accepted", { role_revision: "2" }],
    ["a decision under another policy revision", "decide_proposal", "stage_proposal_accepted", { policy_revision: "2" }],
    ["a decision under another proposal", "decide_proposal", "stage_proposal_accepted", { proposal_id: stageIds.child2 }],
    ["a scope in another lifecycle", "confirm_scope", "stage_scope_confirmed", { lifecycle_epoch: "2" }],
    ["a scope revision that is not the next one", "confirm_scope", "stage_scope_confirmed", { scope_revision: "2" }],
    ["a policy with the opposite switch", "revise_policy", "stage_policy_revised", { enabled: true }],
  ])("keeps the request for %s", (_name, action, code, change) => {
    expect(matchesCollaborationStageConfirmation(requests[action], success(code, { ...stageConfirmations[action], ...change }))).toBe(false);
    expect(matchesCollaborationStageConfirmation(collaborationStageConfirmationIdentity(requests[action]), success(code, { ...stageConfirmations[action], ...change }))).toBe(false);
  });
  it("accepts an unchanged role configuration only at the observed revision, and a rejection only for a reject", () => {
    const unchanged = { ...requests.configure_roles, body: { ...requests.configure_roles.body, expected_role_revision: "1" } };
    expect(matchesCollaborationStageConfirmation(unchanged, success("stage_roles_unchanged", stageConfirmations.configure_roles))).toBe(true);
    // `configured` creates a revision: it can never answer with the observed one or an older one.
    expect(matchesCollaborationStageConfirmation(unchanged, success("stage_roles_configured", stageConfirmations.configure_roles))).toBe(false);
    const later = { ...requests.configure_roles, body: { ...requests.configure_roles.body, expected_role_revision: "3" } };
    expect(matchesCollaborationStageConfirmation(later, success("stage_roles_configured", { ...stageConfirmations.configure_roles, role_revision: "2" }))).toBe(false);
    expect(matchesCollaborationStageConfirmation(later, success("stage_roles_configured", { ...stageConfirmations.configure_roles, role_revision: "4" }))).toBe(true);
    const reject = { ...requests.decide_proposal, body: { ...requests.decide_proposal.body, decision: "reject" as const, reason: "Out of scope" } };
    expect(matchesCollaborationStageConfirmation(reject, success("stage_proposal_rejected", { ...stageConfirmations.decide_proposal, decision: "rejected" }))).toBe(true);
    expect(matchesCollaborationStageConfirmation(reject, success("stage_proposal_accepted", stageConfirmations.decide_proposal))).toBe(false);
  });
  it("never retires a request with a code that does not belong to the variant", () => {
    const forged = { type: "success", code: "stage_proposal_created", collaborationRevision: "13", confirmation: CollaborationStageConfirmationSchema.parse(stageConfirmations.configure_roles) } as const;
    expect(matchesCollaborationStageConfirmation(requests.configure_roles, forged)).toBe(false);
  });
});
