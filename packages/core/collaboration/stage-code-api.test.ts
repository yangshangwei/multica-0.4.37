// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { stageCodeViewFixture, stageIds } from "./stage-test-fixtures";
const base = "https://server.example";
const path = `/api/collaboration/issues/${stageIds.issue}/stage`;
afterEach(() => { vi.unstubAllGlobals(); });
const setup = (...responses: Response[]) => {
  const fetch = vi.fn();
  for (const response of responses) fetch.mockResolvedValueOnce(response);
  vi.stubGlobal("fetch", fetch);
  return { fetch, api: new ApiClient(base) };
};
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
it("reads R4 with exact candidate/issue identity and no workspace header", async () => {
  const { api, fetch } = setup(json(stageCodeViewFixture));
  const controller = new AbortController();
  await expect(api.getCollaborationStageCodeCandidate(stageIds.issue, stageIds.candidate1, { signal: controller.signal })).resolves.toMatchObject({ state: "available" });
  expect(fetch.mock.calls[0]?.[0]).toBe(`${base}${path}/code-candidates/${stageIds.candidate1}`);
  expect(fetch.mock.calls[0]?.[1].headers).not.toHaveProperty("X-Workspace-ID");
});
it("distinguishes unsupported/missing/malformed R4 without inferring evidence", async () => {
  const { api } = setup(new Response(null, { status: 404 }), json({ code: "stage_candidate_not_found" }, 404), json({ ...stageCodeViewFixture, issue_id: stageIds.child1 }),
    json({ ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, candidate_id: stageIds.candidate2 } }));
  await expect(api.getCollaborationStageCodeCandidate(stageIds.issue, stageIds.candidate1)).resolves.toEqual({ state: "unsupported" });
  await expect(api.getCollaborationStageCodeCandidate(stageIds.issue, stageIds.candidate1)).resolves.toEqual({ state: "not_found" });
  for (let i = 0; i < 2; i++) await expect(api.getCollaborationStageCodeCandidate(stageIds.issue, stageIds.candidate1)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
});
it.each([
  ["create_code_candidate", undefined, "/code-candidates"], ["decide_stage", undefined, "/decisions"],
  ["revoke_decision", stageIds.decision, `/decisions/${stageIds.decision}/revocations`], ["advance_stage", undefined, "/advance"],
] as const)("sends %s to its frozen route with the original bytes/key", async (action, decisionId, suffix) => {
  const { api, fetch } = setup(new Response(null, { status: 404 }));
  const bodyJSON = '{"frozen":"body"}';
  await expect(api.sendCollaborationStageRequest({ action, issueId: stageIds.issue, decisionId, requestId: stageIds.request, bodyJSON })).resolves.toMatchObject({ type: "refusal", code: "stage_route_unavailable" });
  expect(fetch.mock.calls[0]?.[0]).toBe(`${base}${path}${suffix}`);
  expect(fetch.mock.calls[0]?.[1]).toMatchObject({ method: "POST", body: bodyJSON, headers: { "Idempotency-Key": stageIds.request } });
});
it("refuses malformed or misplaced revocation identities before transport", async () => {
  const { api, fetch } = setup();
  const request = { action: "revoke_decision" as const, issueId: stageIds.issue, requestId: stageIds.request, bodyJSON: "{}" };
  await expect(api.sendCollaborationStageRequest(request)).rejects.toThrow(TypeError);
  await expect(api.sendCollaborationStageRequest({ ...request, decisionId: "../other" })).rejects.toThrow(TypeError);
  await expect(api.sendCollaborationStageRequest({ ...request, action: "advance_stage", decisionId: stageIds.decision })).rejects.toThrow(TypeError);
  expect(fetch).not.toHaveBeenCalled();
});
