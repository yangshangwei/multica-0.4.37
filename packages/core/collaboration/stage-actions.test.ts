// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { clearCollaborationAccess, collaborationIssueAccessDenied, collaborationKeys, collaborationScope, CollaborationUnavailableError } from "./access";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import {
  CollaborationStagePrepareError, dismissCollaborationStageIntent, executeCollaborationStageIntent, prepareCollaborationStageAction,
  prepareCollaborationStagePolicyRevision, recoverCollaborationStageIntent,
} from "./stage-actions";
import { CollaborationStageIntentStorageError, readCollaborationStageIntents, type CollaborationStageIntent } from "./stage-intent";
import { collaborationStageKeys } from "./stage-queries";
import { parseCollaborationStagePolicy, parseCollaborationStageView, type CollaborationStageSettledResult } from "./stage-schema";
import {
  stageConfirmations, stageDigests, stageIds, stageIssueContextFixture, stageObservationsFixture, stagePolicyFixture, stagePolicyVirtualFixture, stageProposalContentFixture,
  stageViewManagedFixture, stageViewNeedsConfirmationFixture, stageViewUnmanagedFixture, stagePullRequestFixture,
} from "./stage-test-fixtures";
import type { StorageAdapter } from "../types/storage";

const actor = stageIds.product;
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const saved = vi.hoisted(() => new Map<string, string>());
vi.mock("../platform/storage", () => ({ defaultStorage: {
  keys: () => [...saved.keys()], getItem: (key: string) => saved.get(key) ?? null,
  setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key),
} }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); saved.clear(); vi.restoreAllMocks(); });

const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
const managed = parseCollaborationStageView(stageViewManagedFixture)!;
const first = parseCollaborationStageView(stageViewNeedsConfirmationFixture)!;
const candidate = { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 };
const roles = {
  business_owner_user_id: stageIds.owner, implementer_user_id: stageIds.implementer, product_approver_user_id: stageIds.product,
  code_reviewer_user_id: stageIds.reviewer, qa_approver_user_id: stageIds.qa, release_approver_user_id: stageIds.release,
};
const issueTarget = { kind: "issue", issueId: stageIds.issue } as const;
const projectTarget = { kind: "project", workspaceId: stageIds.workspace, projectId: stageIds.project } as const;

function setup(api = new ApiClient("https://server.example")) {
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  clients.push(client);
  const scope = collaborationScope(actor);
  return { api, client, scope, intentScope: { server: scope.server, userId: scope.userId } };
}
type Fixture = ReturnType<typeof setup>;
const accept = (f: Fixture) => prepareCollaborationStageAction(f.scope, issue, managed, { action: "decide_proposal", proposalId: stageIds.proposal, candidate, decision: "accept", reason: "" });
const accepted: CollaborationStageSettledResult = { type: "success", code: "stage_proposal_accepted", collaborationRevision: "13", confirmation: { ...stageConfirmations.decide_proposal } };
const pending = (f: Fixture) => readCollaborationStageIntents(f.intentScope, issueTarget);

describe("preparing a frozen intent", () => {
  it("captures the four workflow observations from the detail and the stage revisions from the view", () => {
    const f = setup();
    const intent = accept(f);
    expect(Object.isFrozen(intent)).toBe(true);
    expect(intent).toMatchObject({ action: "decide_proposal", target: issueTarget, proposalId: stageIds.proposal, scope: f.intentScope });
    expect(JSON.parse(intent.bodyJSON)).toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", candidate, decision: "accept", reason: "" });
    expect(accept(f).requestId).not.toBe(intent.requestId);
    expect(saved.size).toBe(0);
  });
  it("builds each increment-1 body exactly as the contract lists it", () => {
    const f = setup();
    const body = (intent: CollaborationStageIntent) => JSON.parse(intent.bodyJSON);
    expect(body(prepareCollaborationStageAction(f.scope, issue, first, { action: "configure_roles", roles })))
      .toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "0", roles });
    expect(body(prepareCollaborationStageAction(f.scope, issue, first, { action: "create_proposal", content: stageProposalContentFixture, source: { kind: "run", run_id: stageIds.run } })))
      .toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture, source: { kind: "run", run_id: stageIds.run } });
    const revise = prepareCollaborationStageAction(f.scope, issue, managed, { action: "revise_proposal", proposalId: stageIds.proposal, expectedCandidate: candidate,
      content: stageProposalContentFixture, source: { kind: "manual" } });
    expect(revise.proposalId).toBe(stageIds.proposal);
    expect(body(revise)).toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_candidate: candidate, content: stageProposalContentFixture, source: { kind: "manual" } });
    const scopeIntent = prepareCollaborationStageAction(f.scope, issue, managed, { action: "confirm_scope", children: [{ issue_id: stageIds.child1, content_digest: stageDigests.child1, code: null }], reason: "" });
    expect(body(scopeIntent)).toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_scope_revision: "1",
      children: [{ issue_id: stageIds.child1, content_digest: stageDigests.child1, code: null }], reason: "" });
  });
  it.each([
    ["the capability hint is not exactly true", () => ({ ...managed, capabilities: { ...managed.capabilities, decide_proposal: false } }), issue, "unavailable"],
    ["the root is not managed", () => parseCollaborationStageView(stageViewUnmanagedFixture)!, issue, "unavailable"],
    ["a server enum is unknown", () => ({ ...managed, scope_state: "unknown" as const }), issue, "unavailable"],
    ["the viewer has no write context", () => managed, { ...issue, write_context: null }, "unavailable"],
    ["the view belongs to another issue", () => ({ ...managed, issue_id: stageIds.child1 }), issue, "unavailable"],
    ["the view observed another revision", () => ({ ...managed, collaboration_revision: "7" }), issue, "stale_view"],
    ["the view observed another lifecycle", () => ({ ...managed, lifecycle_epoch: "2" }), issue, "stale_view"],
  ] as const)("refuses when %s", (_name, view, detail, reason) => {
    const f = setup();
    let error: unknown;
    try { prepareCollaborationStageAction(f.scope, detail, view(), { action: "decide_proposal", proposalId: stageIds.proposal, candidate, decision: "accept", reason: "" }); }
    catch (caught) { error = caught; }
    expect(error).toBeInstanceOf(CollaborationStagePrepareError);
    expect((error as CollaborationStagePrepareError).reason).toBe(reason);
  });
  it("refuses local role conflicts, invalid input and oversized bodies without spending a key", () => {
    const f = setup();
    const reason = (run: () => unknown) => { try { run(); return "none"; } catch (error) { return error instanceof CollaborationStagePrepareError ? error.reason : "other"; } };
    expect(reason(() => prepareCollaborationStageAction(f.scope, issue, first, { action: "configure_roles", roles: { ...roles, code_reviewer_user_id: stageIds.implementer } }))).toBe("role_conflict");
    expect(reason(() => prepareCollaborationStageAction(f.scope, issue, first, { action: "configure_roles", roles: { ...roles, release_approver_user_id: stageIds.qa } }))).toBe("role_conflict");
    expect(reason(() => prepareCollaborationStageAction(f.scope, issue, managed, { action: "decide_proposal", proposalId: stageIds.proposal, candidate, decision: "reject", reason: "  " }))).toBe("invalid");
    expect(reason(() => prepareCollaborationStageAction(f.scope, issue, first, { action: "create_proposal", content: { summary: "", children: [] }, source: { kind: "manual" } }))).toBe("invalid");
    const large = { summary: "s", children: Array.from({ length: 7 }, (_unused, index) => ({ title: `Child ${index}`, description: "界".repeat(21_845), priority: "none" as const })) };
    expect(reason(() => prepareCollaborationStageAction(f.scope, issue, first, { action: "create_proposal", content: large, source: { kind: "manual" } }))).toBe("none");
    expect(saved.size).toBe(0);
  });
  it("prepares a project policy revision and does not offer enabling unless activation is exactly available", () => {
    const f = setup();
    const policy = parseCollaborationStagePolicy(stagePolicyFixture)!;
    const disable = prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, policy, { enabled: false, reason: " Pause the pilot " }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" });
    expect(disable).toMatchObject({ action: "revise_policy", target: projectTarget });
    expect(JSON.parse(disable.bodyJSON)).toEqual({ expected_policy_revision: "1", enabled: false, reason: "Pause the pilot" });
    expect(JSON.parse(prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, policy, { enabled: true, reason: "Go" }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" }).bodyJSON).enabled).toBe(true);
    const reason = (value: unknown) => { try { prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, value as typeof policy, { enabled: true, reason: "Go" }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" }); return "none"; }
      catch (error) { return error instanceof CollaborationStagePrepareError ? error.reason : "other"; } };
    expect(reason(parseCollaborationStagePolicy(stagePolicyVirtualFixture))).toBe("activation_unavailable");
    const { activation_available: _omitted, ...older } = stagePolicyFixture;
    expect(reason(parseCollaborationStagePolicy(older))).toBe("activation_unavailable");
    // Disabling is never gated by the activation hint.
    expect(JSON.parse(prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, parseCollaborationStagePolicy(stagePolicyVirtualFixture)!, { enabled: false, reason: "Stay off" }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" }).bodyJSON))
      .toEqual({ expected_policy_revision: "0", enabled: false, reason: "Stay off" });
  });
});

describe("executing an intent", () => {
  it("persists the intent before the POST and retires it on its exact confirmation", async () => {
    const f = setup(); const intent = accept(f);
    const send = vi.spyOn(f.api, "sendCollaborationStageRequest").mockImplementation(async () => { expect(pending(f)).toEqual([intent]); return accepted; });
    const invalidate = vi.spyOn(f.client, "invalidateQueries");
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toEqual({ kind: "confirmed", code: "stage_proposal_accepted", confirmation: accepted.type === "success" ? accepted.confirmation : null, cleanupFailed: false });
    expect(send).toHaveBeenCalledExactlyOnceWith({ action: "decide_proposal", issueId: stageIds.issue, proposalId: stageIds.proposal, requestId: intent.requestId, bodyJSON: intent.bodyJSON });
    expect(pending(f)).toEqual([]);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: collaborationKeys.scope(f.scope) });
  });
  it.each([
    ["a rebound version", { ...stageConfirmations.decide_proposal, version: "3" }, "stage_proposal_accepted"],
    ["another candidate", { ...stageConfirmations.decide_proposal, candidate_id: stageIds.candidate1 }, "stage_proposal_accepted"],
    ["the opposite decision", { ...stageConfirmations.decide_proposal, decision: "rejected" as const }, "stage_proposal_rejected"],
    ["another action's variant", { ...stageConfirmations.configure_roles }, "stage_roles_configured"],
  ])("keeps the intent when the confirmation names %s", async (_name, confirmation, code) => {
    const f = setup(); const intent = accept(f);
    vi.spyOn(f.api, "sendCollaborationStageRequest").mockResolvedValue({ type: "success", code, collaborationRevision: "13", confirmation });
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "unknown", reason: "malformed" });
    expect(pending(f)).toEqual([intent]);
  });
  it("settles a committed refusal with its original status and code, without treating it as access loss", async () => {
    const f = setup(); const intent = accept(f);
    vi.spyOn(f.api, "sendCollaborationStageRequest").mockResolvedValue({ type: "refusal", stored: true, status: 403, code: "stage_role_required", collaborationRevision: "12" });
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toEqual({ kind: "refused", status: 403, code: "stage_role_required", cleanupFailed: false });
    expect(pending(f)).toEqual([]);
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(false);
  });
  it("retires an unwritten refusal only for the first and only dispatch of a fresh intent", async () => {
    const f = setup(); const intent = accept(f);
    const send = vi.spyOn(f.api, "sendCollaborationStageRequest").mockResolvedValue({ type: "refusal", stored: false, status: 503, code: "collaboration_busy", collaborationRevision: null });
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toEqual({ kind: "not_written", status: 503, code: "collaboration_busy", cleanupFailed: false });
    expect(pending(f)).toEqual([]);
    // After an uncertain first attempt the same answer proves nothing about the original.
    const second = accept(f);
    send.mockResolvedValueOnce({ type: "unknown", reason: "unavailable", status: 503, code: "collaboration_unavailable" });
    await expect(executeCollaborationStageIntent(f.client, f.scope, second)).resolves.toMatchObject({ kind: "unknown", reason: "unavailable" });
    await expect(executeCollaborationStageIntent(f.client, f.scope, second)).resolves.toEqual({ kind: "unknown", reason: "unproven_refusal", status: 503, code: "collaboration_busy" });
    expect(pending(f)).toEqual([second]);
    expect(send).toHaveBeenCalledTimes(3);
  });
  it("does not trust an unwritten refusal for an intent restored from storage after a reload", async () => {
    const before = setup(); const intent = accept(before);
    vi.spyOn(before.api, "sendCollaborationStageRequest").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(executeCollaborationStageIntent(before.client, before.scope, intent)).resolves.toEqual({ kind: "unknown", reason: "transport", status: null, code: null });
    const after = setup();
    const [restored] = pending(after);
    expect(restored).toEqual(intent);
    vi.spyOn(after.api, "sendCollaborationStageRequest").mockResolvedValue({ type: "refusal", stored: false, status: 400, code: "invalid_stage_request", collaborationRevision: null });
    await expect(executeCollaborationStageIntent(after.client, after.scope, restored!)).resolves.toMatchObject({ kind: "unknown", reason: "unproven_refusal" });
    expect(pending(after)).toEqual([intent]);
  });
  it("replays the same key and body on an explicit resend and settles on the original success", async () => {
    const f = setup(); const intent = accept(f);
    const send = vi.spyOn(f.api, "sendCollaborationStageRequest").mockResolvedValueOnce({ type: "unknown", reason: "malformed", status: 200, code: null }).mockResolvedValueOnce(accepted);
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "unknown" });
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "confirmed" });
    expect(send.mock.calls[0]).toEqual(send.mock.calls[1]);
    expect(pending(f)).toEqual([]);
  });
  it("clears protected state on a plain access answer and retains GET-only recovery", async () => {
    const f = setup(); const intent = accept(f);
    vi.spyOn(f.api, "sendCollaborationStageRequest").mockRejectedValue(new ApiError("Collaboration stage request failed", 404, "Not Found"));
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).rejects.toMatchObject({ status: 404 });
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(true);
    expect(pending(f)).toMatchObject([{ recoveryOnly: true, requestId: intent.requestId }]);
  });
  it("sends nothing when the intent cannot be preserved first", async () => {
    const f = setup(); const intent = accept(f);
    const send = vi.spyOn(f.api, "sendCollaborationStageRequest");
    const lossy: StorageAdapter = { keys: () => [], getItem: () => null, setItem: () => undefined, removeItem: () => undefined };
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent, lossy)).rejects.toBeInstanceOf(CollaborationStageIntentStorageError);
    expect(send).not.toHaveBeenCalled();
  });
  it("refuses an intent of another account, a tampered body or a superseded session before sending", async () => {
    const f = setup(); const intent = accept(f);
    const send = vi.spyOn(f.api, "sendCollaborationStageRequest");
    await expect(executeCollaborationStageIntent(f.client, f.scope, { ...intent, scope: { ...intent.scope, userId: stageIds.owner } })).rejects.toBeInstanceOf(CollaborationUnavailableError);
    await expect(executeCollaborationStageIntent(f.client, f.scope, { ...intent, bodyJSON: intent.bodyJSON.replace('"accept"', '"reject"') })).rejects.toBeInstanceOf(CollaborationUnavailableError);
    await expect(executeCollaborationStageIntent(f.client, collaborationScope(stageIds.owner), intent)).rejects.toBeInstanceOf(CollaborationUnavailableError);
    expect(send).not.toHaveBeenCalled();
    expect(saved.size).toBe(0);
  });
  it("reports a confirmed result even when its local recovery record cannot be removed", async () => {
    const f = setup(); const intent = accept(f);
    vi.spyOn(f.api, "sendCollaborationStageRequest").mockResolvedValue(accepted);
    const sticky = new Map<string, string>();
    const storage: StorageAdapter = { keys: () => [...sticky.keys()], getItem: (key) => sticky.get(key) ?? null, setItem: (key, value) => { sticky.set(key, value); }, removeItem: () => undefined };
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent, storage)).resolves.toMatchObject({ kind: "confirmed", cleanupFailed: true });
  });
});

describe("project policy intents", () => {
  const policy = parseCollaborationStagePolicy(stagePolicyFixture)!;
  it("renders the temporary gating as not written, never as a retryable error", async () => {
    const f = setup(); const intent = prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, policy, { enabled: true, reason: "Go" }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" });
    const send = vi.spyOn(f.api, "reviseProjectStagePolicy").mockResolvedValue({ type: "refusal", stored: false, status: 503, code: "stage_policy_unavailable", collaborationRevision: null });
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toEqual({ kind: "not_written", status: 503, code: "stage_policy_unavailable", cleanupFailed: false });
    expect(send).toHaveBeenCalledExactlyOnceWith({ workspaceId: stageIds.workspace, projectId: stageIds.project, requestId: intent.requestId, bodyJSON: intent.bodyJSON });
    expect(readCollaborationStageIntents(f.intentScope, projectTarget)).toEqual([]);
  });
  it("retires only on the matching project, switch and a newer revision", async () => {
    const f = setup(); const intent = prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, policy, { enabled: false, reason: "Pause" }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" });
    const send = vi.spyOn(f.api, "reviseProjectStagePolicy");
    for (const confirmation of [{ ...stageConfirmations.revise_policy, policy_revision: "1" }, { ...stageConfirmations.revise_policy, policy_revision: "2", enabled: true },
      { ...stageConfirmations.revise_policy, policy_revision: "2", project_id: stageIds.child1 }]) {
      send.mockResolvedValueOnce({ type: "success", code: "stage_policy_revised", collaborationRevision: null, confirmation });
      await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "unknown", reason: "malformed" });
    }
    send.mockResolvedValueOnce({ type: "success", code: "stage_policy_revised", collaborationRevision: null, confirmation: { ...stageConfirmations.revise_policy, policy_revision: "2" } });
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "confirmed", confirmation: { policy_revision: "2" } });
    expect(readCollaborationStageIntents(f.intentScope, projectTarget)).toEqual([]);
  });
  it("recovers by GET: a missing original operation stays unknown and never PUTs", async () => {
    const f = setup(); const intent = prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, policy, { enabled: false, reason: "Pause" }, { workspaceId: stageIds.workspace, userId: stageIds.product, role: "owner" });
    const put = vi.spyOn(f.api, "reviseProjectStagePolicy").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "unknown", reason: "transport" });
    const get = vi.spyOn(f.api, "getProjectStagePolicyOperation").mockResolvedValueOnce({ type: "not_found" })
      .mockResolvedValueOnce({ type: "success", code: "stage_policy_revised", collaborationRevision: null, confirmation: { ...stageConfirmations.revise_policy, policy_revision: "2" } });
    await expect(recoverCollaborationStageIntent(f.client, f.scope, intent)).resolves.toEqual({ kind: "unknown", reason: "not_found", status: 404, code: null });
    expect(readCollaborationStageIntents(f.intentScope, projectTarget)).toEqual([intent]);
    await expect(recoverCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "confirmed" });
    expect(get).toHaveBeenCalledWith(stageIds.workspace, stageIds.project, intent.requestId);
    expect(put).toHaveBeenCalledTimes(1);
    expect(readCollaborationStageIntents(f.intentScope, projectTarget)).toEqual([]);
  });
});

describe("GET-only recovery of an issue operation (contract 3.5)", () => {
  it("keeps an A5 full code head and original key through reload, missing receipt and exact recovery", async () => {
    const before = setup();
    const code = { pull_request: { ...stagePullRequestFixture } };
    const intent = prepareCollaborationStageAction(before.scope, issue, managed, { action: "confirm_scope",
      children: [{ issue_id: stageIds.child1, content_digest: stageDigests.child1, code }], reason: "Carry this version" });
    const post = vi.spyOn(before.api, "sendCollaborationStageRequest").mockRejectedValue(new TypeError("Lost response"));
    code.pull_request.head_sha = "b".repeat(40);
    await expect(executeCollaborationStageIntent(before.client, before.scope, intent)).resolves.toMatchObject({ kind: "unknown" });
    const after = setup();
    const [restored] = pending(after);
    expect(restored).toEqual(intent);
    expect(JSON.parse("bodyJSON" in restored! ? restored.bodyJSON : "null").children[0].code.pull_request.head_sha).toBe(stagePullRequestFixture.head_sha);
    const newPost = vi.spyOn(after.api, "sendCollaborationStageRequest");
    const get = vi.spyOn(after.api, "getCollaborationStageOperation").mockResolvedValueOnce({ type: "not_found" })
      .mockResolvedValueOnce({ type: "success", code: "stage_scope_confirmed", collaborationRevision: "9007199254740994",
        confirmation: { ...stageConfirmations.confirm_scope, scope_revision: "2" } });
    await expect(recoverCollaborationStageIntent(after.client, after.scope, restored!)).resolves.toMatchObject({ kind: "unknown", reason: "not_found" });
    expect(pending(after)).toEqual([intent]);
    await expect(recoverCollaborationStageIntent(after.client, after.scope, restored!)).resolves.toMatchObject({ kind: "confirmed" });
    expect(get.mock.calls).toEqual([[stageIds.issue, intent.requestId], [stageIds.issue, intent.requestId]]);
    expect(post).toHaveBeenCalledTimes(1);
    expect(newPost).not.toHaveBeenCalled();
    expect(pending(after)).toEqual([]);
  });
  async function unknownFirst(f: Fixture) {
    const intent = accept(f);
    const send = vi.spyOn(f.api, "sendCollaborationStageRequest").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "unknown", reason: "transport" });
    return { intent, send };
  }
  it("retires the intent on the original success and never POSTs", async () => {
    const f = setup(); const { intent, send } = await unknownFirst(f);
    const get = vi.spyOn(f.api, "getCollaborationStageOperation").mockResolvedValue(accepted);
    await expect(recoverCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "confirmed", code: "stage_proposal_accepted" });
    expect(get).toHaveBeenCalledExactlyOnceWith(stageIds.issue, intent.requestId);
    expect(send).toHaveBeenCalledTimes(1);
    expect(pending(f)).toEqual([]);
  });
  it("shows a stored refusal with its original status and code and retires only the request", async () => {
    const f = setup(); const { intent, send } = await unknownFirst(f);
    vi.spyOn(f.api, "getCollaborationStageOperation").mockResolvedValue({ type: "refusal", stored: true, status: 409, code: "stage_candidate_changed", collaborationRevision: "12" });
    await expect(recoverCollaborationStageIntent(f.client, f.scope, intent)).resolves.toEqual({ kind: "refused", status: 409, code: "stage_candidate_changed", cleanupFailed: false });
    expect(send).toHaveBeenCalledTimes(1);
    expect(pending(f)).toEqual([]);
  });
  it.each<[string, CollaborationStageSettledResult, Record<string, unknown>]>([
    ["404 not_found", { type: "not_found" }, { kind: "unknown", reason: "not_found", status: 404, code: null }],
    ["409 operation_incomplete", { type: "incomplete" }, { kind: "unknown", reason: "incomplete", status: 409, code: "operation_incomplete" }],
    ["a malformed body", { type: "unknown", reason: "malformed", status: 200, code: null }, { kind: "unknown", reason: "malformed" }],
    ["a contradictory confirmation", { type: "success", code: "stage_proposal_accepted", collaborationRevision: "13", confirmation: { ...stageConfirmations.decide_proposal, version: "3" } }, { kind: "unknown", reason: "malformed" }],
    ["an unwritten refusal shape", { type: "refusal", stored: false, status: 503, code: "collaboration_busy", collaborationRevision: null }, { kind: "unknown", reason: "unproven_refusal" }],
  ])("keeps the intent pending on %s and never writes", async (_name, result, outcome) => {
    const f = setup(); const { intent, send } = await unknownFirst(f);
    vi.spyOn(f.api, "getCollaborationStageOperation").mockResolvedValue(result);
    await expect(recoverCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject(outcome);
    expect(send).toHaveBeenCalledTimes(1);
    expect(pending(f)).toEqual([intent]);
  });
  it("keeps the intent when the recovery read itself fails in transit", async () => {
    const f = setup(); const { intent, send } = await unknownFirst(f);
    vi.spyOn(f.api, "getCollaborationStageOperation").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(recoverCollaborationStageIntent(f.client, f.scope, intent)).resolves.toMatchObject({ kind: "unknown", reason: "transport" });
    expect(send).toHaveBeenCalledTimes(1);
    expect(pending(f)).toEqual([intent]);
  });
  it("recovers after a reload from the stored key and body alone", async () => {
    const before = setup(); const { intent } = await unknownFirst(before);
    const after = setup();
    const post = vi.spyOn(after.api, "sendCollaborationStageRequest");
    vi.spyOn(after.api, "getCollaborationStageOperation").mockResolvedValue(accepted);
    await expect(recoverCollaborationStageIntent(after.client, after.scope, pending(after)[0]!)).resolves.toMatchObject({ kind: "confirmed" });
    expect(post).not.toHaveBeenCalled();
    expect(pending(after)).toEqual([]);
    expect(intent.requestId).toBeTruthy();
  });
  it("lets the user discard only the local record; nothing is sent and no outcome is claimed", async () => {
    const f = setup(); const { intent, send } = await unknownFirst(f);
    const get = vi.spyOn(f.api, "getCollaborationStageOperation");
    dismissCollaborationStageIntent(f.scope, intent);
    expect(pending(f)).toEqual([]);
    expect(send).toHaveBeenCalledTimes(1);
    expect(get).not.toHaveBeenCalled();
    expect(() => dismissCollaborationStageIntent(collaborationScope(stageIds.owner), intent)).toThrow(CollaborationUnavailableError);
  });
});

it("invalidates the stage subtree with the collaboration scope after a settled write and after access loss", async () => {
  const f = setup(); const intent = accept(f);
  const key = collaborationStageKeys.view(f.scope, stageIds.issue);
  f.client.setQueryData(key, { state: "available", value: managed });
  vi.spyOn(f.api, "sendCollaborationStageRequest").mockResolvedValue(accepted);
  await executeCollaborationStageIntent(f.client, f.scope, intent);
  expect(f.client.getQueryState(key)?.isInvalidated).toBe(true);
  clearCollaborationAccess(f.client, f.scope, stageIds.issue);
  expect(f.client.getQueryData(key)).toBeUndefined();
});

it("C01 strips private intent on issue denial but recovers the original account receipt by GET only", async () => {
  const f = setup();
  const intent = prepareCollaborationStageAction(f.scope, issue, managed, { action: "decide_proposal", proposalId: stageIds.proposal, candidate,
    decision: "accept", reason: "PRIVATE approval reason" });
  const post = vi.spyOn(f.api, "sendCollaborationStageRequest").mockRejectedValue(new TypeError("Lost response"));
  await executeCollaborationStageIntent(f.client, f.scope, intent);
  clearCollaborationAccess(f.client, f.scope, issue.id);
  const [recovery] = pending(f);
  expect(recovery).toMatchObject({ recoveryOnly: true, requestId: intent.requestId, action: intent.action });
  expect(recovery).not.toHaveProperty("bodyJSON");
  expect([...saved.values()].join("")).not.toContain("PRIVATE approval reason");
  const get = vi.spyOn(f.api, "getCollaborationStageOperation").mockResolvedValueOnce({ type: "not_found" }).mockResolvedValueOnce(accepted);
  await expect(executeCollaborationStageIntent(f.client, f.scope, recovery!)).rejects.toBeDefined();
  await expect(executeCollaborationStageIntent(f.client, f.scope, intent)).rejects.toBeDefined();
  await expect(recoverCollaborationStageIntent(f.client, f.scope, recovery!)).resolves.toMatchObject({ kind: "unknown", reason: "not_found" });
  expect(pending(f)).toEqual([recovery]);
  await expect(recoverCollaborationStageIntent(f.client, f.scope, recovery!)).resolves.toMatchObject({ kind: "confirmed" });
  expect(post).toHaveBeenCalledTimes(1);
  expect(get.mock.calls).toEqual([[issue.id, intent.requestId], [issue.id, intent.requestId]]);
  expect(pending(f)).toEqual([]);
});
it.each([null, { role: "member" }, { role: null }, { role: "owner", workspaceId: stageIds.child1 }, { role: "owner", userId: stageIds.owner }] as const)("C05 rejects missing or mismatched policy administration authority %j", (change) => {
  const f = setup(), policy = parseCollaborationStagePolicy(stagePolicyFixture)!;
  const authority = change === null ? null : { workspaceId: stageIds.workspace, userId: stageIds.product, ...change };
  expect(() => prepareCollaborationStagePolicyRevision(f.scope, stageIds.workspace, policy, { enabled: false, reason: "Pause" }, authority)).toThrow(CollaborationStagePrepareError);
});
