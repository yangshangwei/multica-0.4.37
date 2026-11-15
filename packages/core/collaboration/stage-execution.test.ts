// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { executeCollaborationStageIntent, prepareCollaborationStageAction, recoverCollaborationStageIntent } from "./stage-actions";
import { deriveCollaborationStageCodeState } from "./stage-code-presentation";
import { collaborationStageFrozenRequest, freezeCollaborationStageIntent } from "./stage-intent";
import { CollaborationStageRequestSchemas, parseCollaborationStageCodeCandidate, parseCollaborationStageView } from "./stage-schema";
import { parseCollaborationTestReport } from "./stage-test-report-schema";
import { stageReportDetailFixture } from "./stage-test-report-fixtures";
import { stageCodeCandidateRefFixture, stageCodeEvidenceFixture, stageCodeViewFixture, stageExecutionBuildFixture, stageExecutionIds, stageExecutionViewFixture,
  stageIds, stageIssueContextFixture, stageObservationsFixture, stageRollbackObservationFixture, stageTestReportSummaryFixture, stageViewManagedFixture } from "./stage-test-fixtures";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000206" } }) } }));
afterEach(() => vi.restoreAllMocks());
const now = Date.parse("2026-11-14T03:05:00Z");
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
const stageView = (stage: "release" | "verification") => parseCollaborationStageView({ ...stageViewManagedFixture,
  stage: { current: stage, stage_revision: "4" }, code_candidates: { items: [{ ...stageCodeCandidateRefFixture, state: "current" }], truncated: false } })!;
const record = () => parseCollaborationStageCodeCandidate(structuredClone(stageExecutionViewFixture))!;
const state = (value = record(), stage: "release" | "verification" = "release") => deriveCollaborationStageCodeState(issue, stageView(stage), value, stageIds.release, now);
const body = { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_stage_revision: "4", candidate: stageCodeCandidateRefFixture,
  step: "release", decision: "approve", reason: "Reviewed exact target", expires_at: "2026-11-14T03:15:00Z", depends_on: [stageExecutionIds.qa],
  evidence: { ...stageCodeEvidenceFixture, ci_digest: null, build: stageExecutionBuildFixture } };
const report = () => parseCollaborationTestReport({ ...stageReportDetailFixture, report_id: stageTestReportSummaryFixture.report_id,
  subject: { ...stageReportDetailFixture.subject, artifact_digest: stageExecutionBuildFixture.artifact_digest }, reported_result: "passed", applicability: "usable", case_count: 3, failed_case_count: 0,
  cases: ["callback-delay", "callback-duplicate", "refund"].map((id) => ({ id, outcome: "passed", log_index: 1 })) })!;
function prepare(step: "release" | "rollback" = "release", value = record(), evidenceReport = report()) {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  vi.spyOn(Date, "now").mockReturnValue(now);
  return { api, scope: collaborationScope(stageIds.release), input: { action: "decide_stage" as const, record: value, candidate: stageCodeCandidateRefFixture,
    step, decision: "approve" as const, reason: "Reviewed exact evidence", expires_at: body.expires_at, evidenceReport,
    depends_on: [step === "release" ? stageExecutionIds.qa : stageExecutionIds.release],
    evidence: step === "release" ? body.evidence : { ...stageCodeEvidenceFixture, ci_digest: null, deployment_id: stageExecutionIds.fact,
      rollback_target_digest: "b".repeat(64), rollback_observation: { observation_id: stageRollbackObservationFixture.observation_id, observation_digest: stageRollbackObservationFixture.observation_digest } } } };
}

describe("2c R4 execution projections", () => {
  it("retains native operation strings and exact target, intent and metric identities", () => {
    expect(record()).toMatchObject({ execution_targets: stageExecutionViewFixture.execution_targets, execution_operations: stageExecutionViewFixture.execution_operations,
      execution_facts: stageExecutionViewFixture.execution_facts, rollback_observations: stageExecutionViewFixture.rollback_observations });
  });
  it("keeps old omitted execution support unknown and blocks execution-dependent actions", () => {
    const old = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [{ ...stageExecutionViewFixture.decisions[0], evidence: { ...body.evidence, build: { artifact_digest: "c".repeat(64), environment: "staging" } } }],
      execution_facts: stageExecutionViewFixture.execution_facts.map(({ intent_id: _intent, target_digest: _target, ...fact }) => fact) })!;
    expect(old).not.toBeNull();
    expect(old).not.toHaveProperty("execution_operations");
    expect(state(old).approve).toEqual([]);
    expect(state(old).advances).toEqual([]);
    expect(state(old).revocable).not.toContain(stageExecutionIds.release);
  });
  it.each([
    ["partial array group", (v: typeof stageExecutionViewFixture) => ({ ...v, rollback_observations: undefined })],
    ["null group", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_targets: null })],
    ["missing fact intent", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_facts: [{ ...v.execution_facts[0], intent_id: undefined }] })],
    ["missing fact target", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_facts: [{ ...v.execution_facts[0], target_digest: undefined }] })],
    ["pending with a fact", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_operations: [{ ...v.execution_operations[0], state: "pending" }] })],
    ["not-started with a fact", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_operations: [{ ...v.execution_operations[0], state: "not_started" }] })],
    ["not-started with a native operation", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_facts: [], execution_operations: [{ ...v.execution_operations[0], state: "not_started", fact_id: null }] })],
    ["conflicting fact target", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_facts: [{ ...v.execution_facts[0], target_digest: "e".repeat(64) }] })],
    ["oversized native ID", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_operations: [{ ...v.execution_operations[0], external_operation_id: "界".repeat(86) }] })],
    ["NUL native ID", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_facts: [{ ...v.execution_facts[0], operation_id: "x\0y" }] })],
    ["oversized environment", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_targets: [{ ...v.execution_targets[0], environment: "界".repeat(43) }] })],
    ["reversed metric window", (v: typeof stageExecutionViewFixture) => ({ ...v, rollback_observations: [{ ...v.rollback_observations[0], window_start: "2026-11-14T03:05:00Z" }] })],
    ["oversized metric summary", (v: typeof stageExecutionViewFixture) => ({ ...v, rollback_observations: [{ ...v.rollback_observations[0], summary: "界".repeat(667) }] })],
    ["duplicate operations", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_operations: [...v.execution_operations, ...v.execution_operations] })],
    ["duplicate metrics", (v: typeof stageExecutionViewFixture) => ({ ...v, rollback_observations: [...v.rollback_observations, ...v.rollback_observations] })],
    ["oversized group", (v: typeof stageExecutionViewFixture) => ({ ...v, execution_targets: Array(51).fill(v.execution_targets[0]) })],
  ])("refuses %s", (_name, change) => expect(parseCollaborationStageCodeCandidate(change(structuredClone(stageExecutionViewFixture)))).toBeNull());
  it("keeps source-denied new groups empty and anonymous", () => {
    const denied = { ...stageExecutionViewFixture, candidate: { ...stageExecutionViewFixture.candidate, baseline: null }, blocked_evidence: 1,
      test_reports: [], decisions: [], execution_facts: [], execution_targets: [], execution_operations: [], rollback_observations: [],
      observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
      signable: stageExecutionViewFixture.signable.map((s) => ({ ...s, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "release" }] })) };
    expect(parseCollaborationStageCodeCandidate(denied)).not.toBeNull();
    for (const field of ["execution_targets", "execution_operations", "rollback_observations"] as const) {
      expect(parseCollaborationStageCodeCandidate({ ...denied, [field]: stageExecutionViewFixture[field] })).toBeNull();
    }
  });
});

describe("2c approval and execution are separate", () => {
  it.each(["pending", "unknown", "not_started", "failed"] as const)("%s consumes approval without authorizing verification", (status) => {
    const v = record();
    v.execution_operations = [{ ...v.execution_operations![0]!, state: status, fact_id: status === "failed" ? stageExecutionIds.fact : null, external_operation_id: status === "failed" ? "deploy/native-42" : null }];
    v.execution_facts = status === "failed" ? [{ ...v.execution_facts[0]!, result: "failed" }] : [];
    expect(state(v).revocable).not.toContain(stageExecutionIds.release);
    expect(state(v).advances).toEqual([]);
  });
  it("advances only the selected approval's unique matching successful intent and fact", () => {
    expect(state().advances).toEqual([{ stage: "verification", decisionIds: [stageExecutionIds.release] }]);
    for (const change of [{ target_digest: "e".repeat(64) }, { intent_id: stageIds.request }, { approval_decision_id: stageExecutionIds.qa }]) {
      const v = record(); v.execution_facts = [{ ...v.execution_facts[0]!, ...change }];
      expect(state(v).advances).toEqual([]);
    }
    const v = record(); v.execution_operations = [];
    expect(state(v).advances).toEqual([]);
    const sourceOnly = record(); sourceOnly.candidate.baseline!.artifact = null;
    expect(state(sourceOnly).advances).toEqual([]);
  });
  it("offers rollback only in verification with current anomalous evidence and the original release", () => {
    expect(state(record(), "release").approve).not.toContain("rollback");
    expect(state(record(), "release").returns).not.toContain("rollback");
    expect(state(record(), "verification").approve).toEqual(["rollback"]);
    for (const change of [{ condition: "healthy" as const }, { condition: "unknown" as const }, { state: "ambiguous" as const }, { state: "superseded" as const },
      { valid_until: "2026-11-14T03:05:00Z" }, { deployment_id: stageIds.request }, { target_digest: "e".repeat(64) }]) {
      const v = record(); v.rollback_observations = [{ ...v.rollback_observations![0]!, ...change }];
      expect(state(v, "verification").approve).toEqual([]);
      expect(state(v, "verification").returns).toEqual(["rollback"]);
    }
  });
  it("keeps missing-adapter empty arrays unavailable for approval while reason-only returns stay available", () => {
    const v = record(); v.execution_targets = []; v.execution_operations = []; v.execution_facts = []; v.rollback_observations = [];
    expect(state(v).approve).toEqual([]);
    expect(state(v, "verification").approve).toEqual([]);
    expect(state(v, "verification").returns).toEqual(["rollback"]);
  });
  it("does not infer an old approval is unconsumed from a full bounded operation page", () => {
    const v = record();
    v.execution_facts = [];
    v.execution_operations = Array.from({ length: 50 }, (_, i) => ({ ...v.execution_operations![0]!,
      intent_id: `00000000-0000-4000-8000-${(0x1000 + i).toString(16).padStart(12, "0")}`,
      approval_decision_id: `00000000-0000-4000-8000-${(0x2000 + i).toString(16).padStart(12, "0")}`,
      state: "unknown" as const, fact_id: null, external_operation_id: null }));
    expect(state(v).revocable).not.toContain(stageExecutionIds.release);
  });
  it("freezes the registered release target and checks the selected QA report's build", () => {
    const { scope, input } = prepare();
    expect(JSON.parse(prepareCollaborationStageAction(scope, issue, stageView("release"), input).bodyJSON).evidence).toEqual(body.evidence);
    for (const evidenceReport of [null, { ...report(), subject: { ...report().subject, artifact_digest: null } }, { ...report(), report_id: stageIds.request }]) {
      expect(() => prepareCollaborationStageAction(scope, issue, stageView("release"), { ...input, evidenceReport })).toThrow();
    }
    expect(() => prepareCollaborationStageAction(scope, issue, stageView("release"), { ...input, evidence: { ...input.evidence, build: { ...stageExecutionBuildFixture, target_digest: "e".repeat(64) } } })).toThrow();
  });
  it("freezes the exact metric and immediate predecessor, rejecting arbitrary historical artifacts", () => {
    const { scope, input } = prepare("rollback");
    expect(JSON.parse(prepareCollaborationStageAction(scope, issue, stageView("verification"), input).bodyJSON).evidence).toMatchObject({
      deployment_id: stageExecutionIds.fact, rollback_target_digest: "b".repeat(64), rollback_observation: { observation_id: stageExecutionIds.observation, observation_digest: "f".repeat(64) } });
    for (const evidence of [{ ...input.evidence, rollback_target_digest: "a".repeat(64) }, { ...input.evidence, rollback_observation: undefined },
      { ...input.evidence, rollback_observation: { observation_id: stageExecutionIds.observation, observation_digest: "e".repeat(64) } }]) {
      expect(() => prepareCollaborationStageAction(scope, issue, stageView("verification"), { ...input, evidence })).toThrow();
    }
  });
});

describe("2c request compatibility", () => {
  it("requires target and exactly one dependency for release, metric for rollback", () => {
    expect(CollaborationStageRequestSchemas.decide_stage.safeParse(body).success).toBe(true);
    for (const change of [{ evidence: { ...body.evidence, build: { artifact_digest: "c".repeat(64), environment: "staging" } } },
      { depends_on: [stageExecutionIds.qa, stageIds.decision] }, { evidence: { ...body.evidence, rollback_observation: { observation_id: stageExecutionIds.observation, observation_digest: "f".repeat(64) } } }]) {
      expect(CollaborationStageRequestSchemas.decide_stage.safeParse({ ...body, ...change }).success).toBe(false);
    }
  });
  it.each(["code_review", "qa"] as const)("preserves old %s body bytes and original-key decoding", (step) => {
    const evidence = { ...stageCodeEvidenceFixture, ...(step === "qa" ? { test_report_id: stageTestReportSummaryFixture.report_id } : {}) };
    const old = { ...body, step, evidence, depends_on: step === "qa" ? [stageIds.decision] : [] };
    const frozen = freezeCollaborationStageIntent({ scope: { server: "https://server.example", userId: stageIds.reviewer }, target: { kind: "issue", issueId: issue.id }, action: "decide_stage", requestId: stageIds.request, body: old, createdAt: 1 });
    const parsed = collaborationStageFrozenRequest(frozen);
    expect(parsed).toMatchObject({ action: "decide_stage", body: { evidence } });
    expect(JSON.parse(frozen.bodyJSON).evidence).toEqual(evidence);
    expect(Object.keys(JSON.parse(frozen.bodyJSON).evidence)).toEqual(["ci_digest", "test_report_id", "build", "deployment_id", "rollback_target_digest"]);
    expect(frozen.bodyJSON).toBe(JSON.stringify({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_stage_revision: "4",
      step, decision: "approve", candidate: stageCodeCandidateRefFixture, evidence, depends_on: old.depends_on, reason: body.reason, expires_at: body.expires_at }));
  });
  it("preserves the original release target and key after a lost response and GET404", async () => {
    const { api, scope, input } = prepare();
    const intent = prepareCollaborationStageAction(scope, issue, stageView("release"), input);
    const post = vi.spyOn(api, "sendCollaborationStageRequest").mockRejectedValue(new TypeError("Lost response"));
    const get = vi.spyOn(api, "getCollaborationStageOperation").mockResolvedValue({ type: "not_found" });
    const saved = new Map<string, string>();
    const storage = { getItem: (key: string) => saved.get(key) ?? null, setItem: (key: string, value: string) => { saved.set(key, value); }, removeItem: (key: string) => { saved.delete(key); }, keys: () => [...saved.keys()] };
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    try {
      await expect(executeCollaborationStageIntent(client, scope, intent, storage)).resolves.toMatchObject({ kind: "unknown" });
      await expect(recoverCollaborationStageIntent(client, scope, intent, storage)).resolves.toMatchObject({ kind: "unknown", reason: "not_found" });
      expect(post).toHaveBeenCalledTimes(1); expect(get).toHaveBeenCalledWith(issue.id, intent.requestId);
      expect(JSON.parse([...saved.values()][0]!)).toMatchObject({ requestId: intent.requestId, bodyJSON: intent.bodyJSON });
      expect(JSON.parse(intent.bodyJSON).evidence.build).toEqual(stageExecutionBuildFixture);
    } finally { client.clear(); }
  });
});

// Independent review: terminal facts must preserve the contract's one-intent/approval identity.
it.each([
  ["same intent with another approval", { approval_decision_id: stageExecutionIds.qa, artifact_digest: "e".repeat(64) }],
  ["same approval with another intent", { intent_id: stageIds.request, artifact_digest: "e".repeat(64) }],
])("rejects a shadowed terminal fact: %s", (_name, contradiction) => {
  const value = structuredClone(stageExecutionViewFixture);
  value.execution_facts.push({ ...value.execution_facts[0]!, fact_id: stageIds.request, ...contradiction });
  expect(parseCollaborationStageCodeCandidate(value)).toBeNull();
});
it("rejects a fact that disagrees with the visible operation for its approval", () => {
  const value = structuredClone(stageExecutionViewFixture);
  value.execution_facts = [{ ...value.execution_facts[0]!, fact_id: stageIds.candidate2, intent_id: stageIds.request }];
  expect(parseCollaborationStageCodeCandidate(value)).toBeNull();
});
