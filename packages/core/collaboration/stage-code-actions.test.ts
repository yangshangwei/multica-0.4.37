// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { prepareCollaborationStageAction, executeCollaborationStageIntent, recoverCollaborationStageIntent } from "./stage-actions";
import { readCollaborationStageIntents } from "./stage-intent";
import { parseCollaborationStageCodeCandidate, parseCollaborationStageView } from "./stage-schema";
import { stageCodeCandidateRefFixture, stageCodeEvidenceFixture, stageCodeViewFixture, stageConfirmations, stageIds, stageIssueContextFixture, stagePullRequestFixture, stageViewManagedFixture } from "./stage-test-fixtures";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const saved = vi.hoisted(() => new Map<string, string>());
vi.mock("../platform/storage", () => ({ defaultStorage: { keys: () => [...saved.keys()], getItem: (k: string) => saved.get(k) ?? null,
  setItem: (k: string, v: string) => saved.set(k, v), removeItem: (k: string) => saved.delete(k) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); saved.clear(); vi.restoreAllMocks(); });
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
const view = parseCollaborationStageView({ ...stageViewManagedFixture, code_candidates: { items: [{ candidate_id: stageIds.candidate1, version: "1", content_digest: stageCodeViewFixture.candidate.content_digest, state: "current" }], truncated: false }, stage: { current: "code_review", stage_revision: "2" },
  roles: { ...stageViewManagedFixture.roles, code_reviewer: stageViewManagedFixture.roles.product_approver } })!;
const record = parseCollaborationStageCodeCandidate(stageCodeViewFixture)!;
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  vi.spyOn(Date, "now").mockReturnValue(Date.parse("2026-11-14T03:05:00Z"));
  return { api, client, scope: collaborationScope(stageIds.product) };
}
const decision = { action: "decide_stage" as const, record, step: "code_review" as const, decision: "approve" as const, candidate: stageCodeCandidateRefFixture,
  evidence: stageCodeEvidenceFixture, depends_on: [], reason: "Reviewed exact head", expires_at: "2026-11-14T03:20:00Z" };
it("prepares A6 with configured role observations and no caller-supplied implementer", () => {
  const { scope } = setup();
  const intent = prepareCollaborationStageAction(scope, issue, view, { action: "create_code_candidate", child_issue_id: stageIds.child1,
    supersedes: null, pull_request: stagePullRequestFixture, artifact: null });
  expect(JSON.parse(intent.bodyJSON)).toMatchObject({ child_issue_id: stageIds.child1, pull_request: stagePullRequestFixture, artifact: null, expected_role_revision: "1" });
  expect(JSON.parse(intent.bodyJSON)).not.toHaveProperty("implementer_user_id");
});
it("keeps a new approval unresolved through GET404 and never rebinds its candidate or expiry", async () => {
  const { api, client, scope } = setup();
  const intent = prepareCollaborationStageAction(scope, issue, view, decision);
  const post = vi.spyOn(api, "sendCollaborationStageRequest").mockRejectedValue(new TypeError("Lost response"));
  await expect(executeCollaborationStageIntent(client, scope, intent)).resolves.toMatchObject({ kind: "unknown" });
  const get = vi.spyOn(api, "getCollaborationStageOperation").mockResolvedValueOnce({ type: "not_found" }).mockResolvedValueOnce({ type: "success", code: "stage_decision_recorded",
    collaborationRevision: "9007199254740994", confirmation: { ...stageConfirmations.decide_stage, step: "code_review", expires_at: decision.expires_at } });
  await expect(recoverCollaborationStageIntent(client, scope, intent)).resolves.toMatchObject({ kind: "unknown", reason: "not_found" });
  expect(readCollaborationStageIntents(scope, { kind: "issue", issueId: issue.id })).toEqual([intent]);
  await expect(recoverCollaborationStageIntent(client, scope, intent)).resolves.toMatchObject({ kind: "confirmed" });
  expect(get.mock.calls).toEqual([[issue.id, intent.requestId], [issue.id, intent.requestId]]);
  expect(post).toHaveBeenCalledTimes(1);
});
it("refuses a changed candidate, wrong actor, evidence digest or over-limit expiry before a key is used", () => {
  const { scope } = setup();
  for (const input of [{ ...decision, candidate: { ...decision.candidate, version: "2" } },
    { ...decision, evidence: { ...decision.evidence, ci_digest: "c".repeat(64) } },
    { ...decision, expires_at: "2026-11-14T03:26:00Z" },
    { ...decision, record: { ...record, blocked_evidence: 1 } }]) {
    expect(() => prepareCollaborationStageAction(scope, issue, view, input)).toThrow();
  }
  expect(() => prepareCollaborationStageAction(scope, issue, { ...view, roles: stageViewManagedFixture.roles }, decision)).toThrow();
  expect(saved.size).toBe(0);
});

it("revises the newest stale candidate without approving its obsolete evidence", () => {
  const { scope } = setup();
  const staleView = { ...view, code_candidates: { items: [{ ...stageCodeCandidateRefFixture, state: "stale" as const }], truncated: false } };
  const intent = prepareCollaborationStageAction(scope, issue, staleView, { action: "create_code_candidate", child_issue_id: stageIds.child1,
    supersedes: stageCodeCandidateRefFixture, pull_request: stagePullRequestFixture, artifact: null });
  expect(JSON.parse(intent.bodyJSON).supersedes).toEqual(stageCodeCandidateRefFixture);
});
it("C03 prepares only an evidence-free return when the source baseline is withheld", () => {
  const { scope } = setup();
  const withheld = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, baseline: null }, blocked_evidence: 1,
    observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
    signable: stageCodeViewFixture.signable.map((s) => ({ ...s, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "code_review" }] })) })!;
  const input = { ...decision, record: withheld, decision: "return" as const, expires_at: null,
    evidence: { ci_digest: null, test_report_id: null, build: null, deployment_id: null, rollback_target_digest: null } };
  expect(JSON.parse(prepareCollaborationStageAction(scope, issue, view, input).bodyJSON)).toMatchObject({ decision: "return", evidence: input.evidence, depends_on: [] });
  expect(() => prepareCollaborationStageAction(scope, issue, view, { ...input, evidence: { ...input.evidence, ci_digest: stageCodeEvidenceFixture.ci_digest } })).toThrow();
  expect(() => prepareCollaborationStageAction(scope, issue, view, { ...input, decision: "approve", expires_at: decision.expires_at })).toThrow();
});
