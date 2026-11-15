// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "../api";
import {
  stageConfirmations, stageDigests, stageIds, stageObservationsFixture, stagePolicyFixture, stageProposalContentFixture, stageProposalDetailFixture,
  stageProposalPageFixture, stageViewManagedFixture,
} from "./stage-test-fixtures";

const base = "https://server.example";
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const empty = (status: number) => new Response(null, { status });
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });
function setup(...responses: Response[]) {
  const fetch = vi.fn();
  for (const response of responses) fetch.mockResolvedValueOnce(response);
  vi.stubGlobal("fetch", fetch);
  return { fetch, api: new ApiClient(base) };
}
const issuePath = `/api/collaboration/issues/${stageIds.issue}`;
const policyPath = `/api/projects/${stageIds.project}/stage-policy`;
const candidate = { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 };
const decideBody = JSON.stringify({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", candidate, decision: "accept", reason: "" });
const decideRequest = { action: "decide_proposal", issueId: stageIds.issue, proposalId: stageIds.proposal, requestId: stageIds.request, bodyJSON: decideBody } as const;
const policyBody = JSON.stringify({ expected_policy_revision: "0", enabled: false, reason: "Pause" });
const policyRequest = { workspaceId: stageIds.workspace, projectId: stageIds.project, requestId: stageIds.request2, bodyJSON: policyBody };

describe("R1 stage view", () => {
  it("reads the account-scoped route without ambient workspace headers", async () => {
    const f = setup(json(stageViewManagedFixture));
    await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toMatchObject({ state: "available", value: { issue_id: stageIds.issue, managed: true } });
    const [url, init] = f.fetch.mock.calls[0]!;
    expect(url).toBe(`${base}${issuePath}/stage`);
    expect(init.method ?? "GET").toBe("GET");
    expect(Object.keys(init.headers).map((name) => name.toLowerCase())).not.toContain("x-workspace-id");
  });
  it("is unavailable for a malformed body, another issue's view or a non-JSON answer", async () => {
    const f = setup(json({ ...stageViewManagedFixture, managed: "yes" }), json({ ...stageViewManagedFixture, issue_id: stageIds.child1 }), new Response("<html>", { status: 200 }));
    for (let index = 0; index < 3; index += 1) await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
  });
  it("reports an older server, an unready schema and an unfamiliar refusal without claiming access loss", async () => {
    const f = setup(empty(404), new Response("404 page not found", { status: 404 }), empty(405), json({ code: "stage_schema_unavailable", error: "x" }, 503),
      json({ code: "stage_view_moved", error: "x" }, 404));
    await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toEqual({ state: "unsupported" });
    await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toEqual({ state: "unsupported" });
    await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toEqual({ state: "unsupported" });
    await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toEqual({ state: "unavailable", reason: "schema" });
    await expect(f.api.getCollaborationStage(stageIds.issue)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
  });
  it("surfaces a structured object denial and transient failures as errors without the private body", async () => {
    const f = setup(json({ code: "not_found", error: "PRIVATE detail" }, 404), json({ code: "collaboration_busy", error: "x" }, 503));
    const denied = await f.api.getCollaborationStage(stageIds.issue).catch((error: unknown) => error);
    expect(denied).toBeInstanceOf(ApiError);
    expect((denied as ApiError).status).toBe(404);
    expect((denied as ApiError).body).toBeUndefined();
    expect((denied as ApiError).message).not.toContain("PRIVATE");
    await expect(f.api.getCollaborationStage(stageIds.issue)).rejects.toMatchObject({ status: 503 });
  });
  it("refuses a non-UUID issue before any request", async () => {
    const f = setup();
    await expect(f.api.getCollaborationStage("../other")).rejects.toThrow(TypeError);
    expect(f.fetch).not.toHaveBeenCalled();
  });
});

describe("R2 proposal list", () => {
  it("sends a bounded limit and the opaque cursor", async () => {
    const f = setup(json(stageProposalPageFixture), json({ ...stageProposalPageFixture, next_cursor: "next" }));
    await expect(f.api.getCollaborationStageProposals(stageIds.issue)).resolves.toMatchObject({ state: "available" });
    await expect(f.api.getCollaborationStageProposals(stageIds.issue, { limit: 5, cursor: "opaque cursor" })).resolves.toMatchObject({ state: "available", value: { next_cursor: "next" } });
    expect(f.fetch.mock.calls[0]![0]).toBe(`${base}${issuePath}/stage/proposals?limit=20`);
    expect(f.fetch.mock.calls[1]![0]).toBe(`${base}${issuePath}/stage/proposals?limit=5&cursor=opaque+cursor`);
  });
  it("is unavailable for a malformed page, another issue, an oversized page or a non-advancing cursor", async () => {
    const two = { ...stageProposalPageFixture, items: [stageProposalPageFixture.items[0], { ...stageProposalPageFixture.items[0], proposal_id: stageIds.child1 }] };
    const f = setup(json({ ...stageProposalPageFixture, items: "none" }), json({ ...stageProposalPageFixture, issue_id: stageIds.child1 }), json(two),
      json({ ...stageProposalPageFixture, next_cursor: "same" }));
    await expect(f.api.getCollaborationStageProposals(stageIds.issue)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
    await expect(f.api.getCollaborationStageProposals(stageIds.issue)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
    await expect(f.api.getCollaborationStageProposals(stageIds.issue, { limit: 1 })).resolves.toEqual({ state: "unavailable", reason: "malformed" });
    await expect(f.api.getCollaborationStageProposals(stageIds.issue, { cursor: "same" })).resolves.toEqual({ state: "unavailable", reason: "malformed" });
  });
  it("refuses an invalid limit or cursor locally and reports an absent route", async () => {
    const f = setup(empty(404));
    await expect(f.api.getCollaborationStageProposals(stageIds.issue, { limit: 51 })).rejects.toThrow(TypeError);
    await expect(f.api.getCollaborationStageProposals(stageIds.issue, { cursor: "" })).rejects.toThrow(TypeError);
    expect(f.fetch).not.toHaveBeenCalled();
    await expect(f.api.getCollaborationStageProposals(stageIds.issue)).resolves.toEqual({ state: "unsupported" });
  });
});

describe("R3 proposal detail", () => {
  it("binds the detail to the requested issue and proposal", async () => {
    const f = setup(json(stageProposalDetailFixture), json({ ...stageProposalDetailFixture, proposal_id: stageIds.child1 }), json({ ...stageProposalDetailFixture, versions: [] }));
    await expect(f.api.getCollaborationStageProposal(stageIds.issue, stageIds.proposal)).resolves.toMatchObject({ state: "available", value: { proposal_id: stageIds.proposal } });
    expect(f.fetch.mock.calls[0]![0]).toBe(`${base}${issuePath}/stage/proposals/${stageIds.proposal}`);
    await expect(f.api.getCollaborationStageProposal(stageIds.issue, stageIds.proposal)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
    await expect(f.api.getCollaborationStageProposal(stageIds.issue, stageIds.proposal)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
  });
  it("distinguishes a missing proposal from lost issue access", async () => {
    const f = setup(json({ code: "stage_proposal_not_found", error: "x" }, 404), json({ code: "not_found", error: "x" }, 404));
    await expect(f.api.getCollaborationStageProposal(stageIds.issue, stageIds.proposal)).resolves.toEqual({ state: "not_found" });
    await expect(f.api.getCollaborationStageProposal(stageIds.issue, stageIds.proposal)).rejects.toMatchObject({ status: 404 });
  });
});

describe("A1-A5 issue workflow writes", () => {
  it.each([
    ["configure_roles", undefined, "/stage/roles"],
    ["create_proposal", undefined, "/stage/proposals"],
    ["revise_proposal", stageIds.proposal, `/stage/proposals/${stageIds.proposal}/versions`],
    ["decide_proposal", stageIds.proposal, `/stage/proposals/${stageIds.proposal}/decisions`],
    ["confirm_scope", undefined, "/stage/scope-confirmations"],
  ] as const)("POSTs %s to its route with the frozen key and the exact body", async (action, proposalId, suffix) => {
    const f = setup(json({ code: "x", collaboration_revision: "13" }));
    const bodyJSON = `{"frozen":"${action}"}`;
    await f.api.sendCollaborationStageRequest({ action, issueId: stageIds.issue, proposalId, requestId: stageIds.request, bodyJSON });
    const [url, init] = f.fetch.mock.calls[0]!;
    expect(url).toBe(`${base}${issuePath}${suffix}`);
    expect(init.method).toBe("POST");
    expect(init.headers["Idempotency-Key"]).toBe(stageIds.request);
    expect(init.headers["Content-Type"]).toBe("application/json");
    expect(init.body).toBe(bodyJSON);
  });
  it("refuses a route identity that does not belong to the action before any request", async () => {
    const f = setup();
    await expect(f.api.sendCollaborationStageRequest({ ...decideRequest, proposalId: undefined })).rejects.toThrow(TypeError);
    await expect(f.api.sendCollaborationStageRequest({ action: "configure_roles", issueId: stageIds.issue, proposalId: stageIds.proposal, requestId: stageIds.request, bodyJSON: "{}" })).rejects.toThrow(TypeError);
    await expect(f.api.sendCollaborationStageRequest({ ...decideRequest, requestId: "" })).rejects.toThrow(TypeError);
    await expect(f.api.sendCollaborationStageRequest({ ...decideRequest, issueId: "x" })).rejects.toThrow(TypeError);
    expect(f.fetch).not.toHaveBeenCalled();
  });
  it("returns the success variant, a committed refusal and an unwritten refusal as results", async () => {
    const f = setup(json({ code: "stage_proposal_accepted", collaboration_revision: "13", stage_confirmation: stageConfirmations.decide_proposal }),
      json({ code: "stage_role_required", error: "stage_role_required", status: 403, collaboration_revision: "12" }, 403),
      json({ code: "stage_proposal_not_found", error: "x", status: 404, collaboration_revision: "12" }, 404),
      json({ code: "collaboration_busy", error: "collaboration_busy" }, 503), empty(404));
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).resolves.toMatchObject({ type: "success", code: "stage_proposal_accepted", confirmation: { decision_id: stageIds.decision } });
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).resolves.toEqual({ type: "refusal", stored: true, status: 403, code: "stage_role_required", collaborationRevision: "12" });
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).resolves.toMatchObject({ type: "refusal", stored: true, status: 404 });
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).resolves.toMatchObject({ type: "refusal", stored: false, code: "collaboration_busy" });
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).resolves.toMatchObject({ type: "refusal", stored: false, code: "stage_route_unavailable" });
  });
  it("keeps malformed, contradictory and unavailable answers unknown", async () => {
    const f = setup(json({ code: "stage_proposal_accepted", collaboration_revision: "13" }), new Response("{", { status: 200 }),
      json({ code: "stage_role_required", status: 409, collaboration_revision: "12" }, 409), json({ code: "collaboration_unavailable", error: "x" }, 503), empty(204));
    for (let index = 0; index < 5; index += 1) await expect(f.api.sendCollaborationStageRequest(decideRequest)).resolves.toMatchObject({ type: "unknown" });
  });
  it("throws a sanitized error for plain access answers so the ordinary cleanup still runs", async () => {
    const f = setup(json({ code: "forbidden", error: "PRIVATE" }, 403), json({ code: "not_found", error: "x" }, 404));
    const error = await f.api.sendCollaborationStageRequest(decideRequest).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(403);
    expect((error as ApiError).body).toBeUndefined();
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).rejects.toMatchObject({ status: 404 });
  });
  it("lets a transport failure reach the caller, who keeps the intent", async () => {
    const f = setup();
    f.fetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await expect(f.api.sendCollaborationStageRequest(decideRequest)).rejects.toThrow("Failed to fetch");
  });
});

describe("O1 original-key recovery", () => {
  it("only GETs the existing operation route", async () => {
    const f = setup(json({ code: "stage_proposal_accepted", collaboration_revision: "13", stage_confirmation: stageConfirmations.decide_proposal }));
    await expect(f.api.getCollaborationStageOperation(stageIds.issue, stageIds.request)).resolves.toMatchObject({ type: "success" });
    const [url, init] = f.fetch.mock.calls[0]!;
    expect(url).toBe(`${base}${issuePath}/operations/${stageIds.request}`);
    expect(init.method ?? "GET").toBe("GET");
    expect(init.body).toBeUndefined();
    expect(init.headers["Idempotency-Key"]).toBeUndefined();
  });
  it("maps a stored refusal, an incomplete operation, a missing receipt and a malformed body", async () => {
    const f = setup(json({ code: "stage_candidate_changed", collaboration_revision: "12", status: 409 }), json({ code: "operation_incomplete", error: "x" }, 409),
      json({ code: "not_found", error: "x" }, 404), json({ code: "completed", collaboration_revision: "12" }), json({ code: "stage_candidate_changed", collaboration_revision: "12", status: 403 }));
    await expect(f.api.getCollaborationStageOperation(stageIds.issue, stageIds.request)).resolves.toEqual({ type: "refusal", stored: true, status: 409, code: "stage_candidate_changed", collaborationRevision: "12" });
    await expect(f.api.getCollaborationStageOperation(stageIds.issue, stageIds.request)).resolves.toEqual({ type: "incomplete" });
    await expect(f.api.getCollaborationStageOperation(stageIds.issue, stageIds.request)).resolves.toEqual({ type: "not_found" });
    await expect(f.api.getCollaborationStageOperation(stageIds.issue, stageIds.request)).resolves.toMatchObject({ type: "unknown", reason: "malformed" });
    await expect(f.api.getCollaborationStageOperation(stageIds.issue, stageIds.request)).resolves.toMatchObject({ type: "unknown", reason: "malformed" });
    expect(f.fetch.mock.calls.every(([, init]) => (init.method ?? "GET") === "GET")).toBe(true);
  });
});

describe("P1-P3 project stage policy", () => {
  it("reads the workspace-scoped route for the captured source workspace", async () => {
    const f = setup(json(stagePolicyFixture));
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toMatchObject({ state: "available", value: { policy_revision: "1", activation_available: true } });
    const [url, init] = f.fetch.mock.calls[0]!;
    expect(url).toBe(`${base}${policyPath}`);
    expect(init.headers["X-Workspace-ID"]).toBe(stageIds.workspace);
    expect(init.headers["X-Workspace-Slug"]).toBe("");
  });
  it("is unavailable for a malformed policy or another project, and never throws an access-shaped error", async () => {
    const f = setup(json({ ...stagePolicyFixture, enabled: 1 }), json({ ...stagePolicyFixture, project_id: stageIds.child1 }),
      json({ code: "stage_policy_denied", error: "x" }, 403), json({ code: "stage_policy_project_not_found", error: "x" }, 404), json({ code: "stage_policy_busy", error: "x" }, 409),
      json({ code: "stage_schema_unavailable", error: "x" }, 503), empty(404), json({ error: "workspace not found" }, 404), json({ code: "workspace_access_denied", error: "x" }, 404));
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "denied" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "not_found" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "unavailable", reason: "busy" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "unavailable", reason: "schema" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "unsupported" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "unsupported" });
    await expect(f.api.getProjectStagePolicy(stageIds.workspace, stageIds.project)).resolves.toEqual({ state: "denied" });
  });
  it("PUTs a revision with the frozen key and body and returns the policy variant", async () => {
    const f = setup(json({ code: "stage_policy_revised", stage_confirmation: stageConfirmations.revise_policy }),
      json({ code: "stage_policy_unavailable", error: "stage_policy_unavailable" }, 503), json({ code: "stage_policy_revised", stage_confirmation: stageConfirmations.configure_roles }),
      json({ code: "stage_policy_revised" }));
    await expect(f.api.reviseProjectStagePolicy(policyRequest)).resolves.toMatchObject({ type: "success", confirmation: { action: "revise_policy", policy_revision: "1", enabled: false } });
    const [url, init] = f.fetch.mock.calls[0]!;
    expect(url).toBe(`${base}${policyPath}`);
    expect(init.method).toBe("PUT");
    expect(init.headers["Idempotency-Key"]).toBe(stageIds.request2);
    expect(init.headers["X-Workspace-ID"]).toBe(stageIds.workspace);
    expect(init.body).toBe(policyBody);
    await expect(f.api.reviseProjectStagePolicy(policyRequest)).resolves.toEqual({ type: "refusal", stored: false, status: 503, code: "stage_policy_unavailable", collaborationRevision: null });
    await expect(f.api.reviseProjectStagePolicy(policyRequest)).resolves.toMatchObject({ type: "unknown", reason: "malformed" });
    await expect(f.api.reviseProjectStagePolicy(policyRequest)).resolves.toMatchObject({ type: "unknown", reason: "malformed" });
  });
  it("recovers by GET only: the original success, or an unknown result that never becomes a PUT", async () => {
    const f = setup(json({ code: "stage_policy_revised", stage_confirmation: stageConfirmations.revise_policy }), json({ code: "stage_policy_operation_not_found", error: "x" }, 404),
      json({ ...stagePolicyFixture }));
    await expect(f.api.getProjectStagePolicyOperation(stageIds.workspace, stageIds.project, stageIds.request2)).resolves.toMatchObject({ type: "success" });
    expect(f.fetch.mock.calls[0]![0]).toBe(`${base}${policyPath}/operations/${stageIds.request2}`);
    await expect(f.api.getProjectStagePolicyOperation(stageIds.workspace, stageIds.project, stageIds.request2)).resolves.toEqual({ type: "not_found" });
    // The current policy "happening to be equal" is not a receipt.
    await expect(f.api.getProjectStagePolicyOperation(stageIds.workspace, stageIds.project, stageIds.request2)).resolves.toMatchObject({ type: "unknown", reason: "malformed" });
    expect(f.fetch.mock.calls.every(([, init]) => (init.method ?? "GET") === "GET")).toBe(true);
  });
});

it("builds increment-1 proposal bodies that the transport sends unchanged", async () => {
  const f = setup(json({ code: "stage_proposal_created", collaboration_revision: "13", stage_confirmation: stageConfirmations.create_proposal }));
  const bodyJSON = JSON.stringify({ ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture, source: { kind: "manual" } });
  await expect(f.api.sendCollaborationStageRequest({ action: "create_proposal", issueId: stageIds.issue, requestId: stageIds.request, bodyJSON })).resolves.toMatchObject({ type: "success" });
  expect(JSON.parse(f.fetch.mock.calls[0]![1].body)).toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture, source: { kind: "manual" } });
});
