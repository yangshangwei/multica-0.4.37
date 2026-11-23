// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { switchIssueId as issueId, switchTargetId as targetId, switchProposalId as proposalId, switchRequestId as key,
  switchOtherId as otherId, switchProposeInput, switchConsentInput, switchRevokeInput, switchCommitInput,
  switchProposal as proposal, switchReceipt as receipt } from "./switch-test-fixtures";

const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const baseUrl = "https://api.example.test";
const path = `${baseUrl}/api/collaboration/issues/${issueId}/targets/${targetId}/switch-proposals`;
afterEach(() => { vi.unstubAllGlobals(); setSchemaLogger(noopLogger); setCurrentWorkspace(null, null); });

it("reads complete Switch proposals with matching path identities and exact versions", async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(json([proposal])).mockResolvedValueOnce(json(proposal));
  vi.stubGlobal("fetch", fetchMock);
  setCurrentWorkspace("ambient-space", issueId);
  const api = new ApiClient(baseUrl);
  await expect(api.listCollaborationTargetSwitchProposals(issueId, targetId)).resolves.toEqual([proposal]);
  await expect(api.getCollaborationTargetSwitchProposal(issueId, targetId, proposalId)).resolves.toEqual(proposal);
  expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([`${path}?execution_view_version=6`, `${path}/${proposalId}?execution_view_version=6`]);
  for (const [, init] of fetchMock.mock.calls) {
    expect(init.method ?? "GET").toBe("GET");
    const headers = new Headers(init.headers);
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
  }
});

it("rejects malformed or duplicate proposals and another target or proposal", async () => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const api = new ApiClient(baseUrl);
  for (const value of [null, {}, [proposal, proposal], [{ ...proposal, target_id: otherId }], [{ ...proposal, proposed_config_version: 9007199254740994 }]]) {
    fetchMock.mockResolvedValueOnce(json(value));
    await expect(api.listCollaborationTargetSwitchProposals(issueId, targetId)).resolves.toBeNull();
  }
  for (const patch of [{ target_id: otherId }, { proposal_id: otherId }, { status: "future" }, { resources: null }]) {
    fetchMock.mockResolvedValueOnce(json({ ...proposal, ...patch }));
    await expect(api.getCollaborationTargetSwitchProposal(issueId, targetId, proposalId)).resolves.toBeNull();
  }
  fetchMock.mockResolvedValueOnce(json([]));
  await expect(api.listCollaborationTargetSwitchProposals(issueId, targetId)).resolves.toEqual([]);
});

it("sends all four immutable Switch actions using their own paths and exact authority", async () => {
  const { action: _action, target_id: _target, ...proposeBody } = switchProposeInput;
  const { action: _consent, target_id: _consentTarget, proposal_id: _proposal, ...consentBody } = switchConsentInput;
  const { action: _revoke, target_id: _revokeTarget, proposal_id: _revokeProposal, agent_id: _agent, ...revokeBody } = switchRevokeInput;
  const { action: _commit, target_id: _commitTarget, proposal_id: _commitProposal, ...commitBody } = switchCommitInput;
  const cases = [
    { input: switchProposeInput, suffix: "", body: proposeBody, code: "switch_proposed", status: 201 },
    { input: switchConsentInput, suffix: `/${proposalId}/consents`, body: consentBody, code: "switch_resource_consented", status: 201 },
    { input: switchRevokeInput, suffix: `/${proposalId}/consents/${switchRevokeInput.agent_id}/revoke`, body: revokeBody, code: "switch_consent_revoked", status: 200 },
    { input: switchCommitInput, suffix: `/${proposalId}/commit`, body: commitBody, code: "target_switched", status: 200 },
  ];
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  setCurrentWorkspace("ambient-space", issueId);
  const api = new ApiClient(baseUrl);
  api.setToken("test-credential");
  for (const [index, entry] of cases.entries()) {
    const expected = { ...receipt, action: entry.input.action, code: entry.code, status: entry.status };
    fetchMock.mockResolvedValueOnce(json(expected, entry.status));
    await expect(api.switchCollaborationTarget(issueId, key, entry.input)).resolves.toEqual(expected);
    const [url, init] = fetchMock.mock.calls[index]!;
    expect(url).toBe(`${path}${entry.suffix}`);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual(entry.body);
    const headers = new Headers(init.headers);
    expect(headers.get("Idempotency-Key")).toBe(key);
    expect(headers.get("Authorization")).toBe("Bearer test-credential");
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
  }
});

it("recovers original refusals by GET without posting or using current proposal readiness", async () => {
  const refused = { ...receipt, status: 409, code: "switch_consent_required" };
  const fetchMock = vi.fn().mockResolvedValueOnce(json(refused, 409)).mockResolvedValueOnce(json(refused));
  vi.stubGlobal("fetch", fetchMock);
  const api = new ApiClient(baseUrl);
  await expect(api.switchCollaborationTarget(issueId, key, switchCommitInput)).resolves.toEqual(refused);
  await expect(api.getCollaborationSwitchOperation(issueId, key, switchCommitInput)).resolves.toEqual(refused);
  const [url, init] = fetchMock.mock.calls[1]!;
  expect(url).toBe(`${baseUrl}/api/collaboration/issues/${issueId}/switch/operations/${key}`);
  expect(init.method ?? "GET").toBe("GET");
  expect(init.body).toBeUndefined();
});

it("rejects receipts with another issue, key, action, target, proposal or HTTP status", async () => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const api = new ApiClient(baseUrl);
  for (const patch of [{ issue_id: otherId }, { request_id: "other-key" }, { target_id: otherId }, { proposal_id: otherId },
    { action: "revoke_target_switch_consent", code: "switch_consent_revoked" }, { protocol_version: 1 }]) {
    fetchMock.mockResolvedValueOnce(json({ ...receipt, ...patch }));
    await expect(api.getCollaborationSwitchOperation(issueId, key, switchCommitInput)).resolves.toBeNull();
  }
  fetchMock.mockResolvedValueOnce(json(receipt, 201));
  await expect(api.switchCollaborationTarget(issueId, key, switchCommitInput)).resolves.toBeNull();
  fetchMock.mockResolvedValueOnce(json({ ...receipt, status: 409, code: "stale_hold" }, 500));
  await expect(api.switchCollaborationTarget(issueId, key, switchCommitInput)).resolves.toBeNull();
});

it("retains missing receipts while keeping private errors and malformed JSON out of results and logs", async () => {
  const error = vi.fn(), warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(json({ code: "operation_not_found", error: "PRIVATE_MISSING" }, 404))
    .mockResolvedValueOnce(json({ error: "PRIVATE_DENIAL", token: "PRIVATE_TOKEN" }, 403))
    .mockResolvedValueOnce(json({ error: "PRIVATE_READ", configuration: "PRIVATE_CONFIG" }, 403))
    .mockResolvedValueOnce(new Response("PRIVATE_INVALID_JSON", { status: 200 }))
    .mockResolvedValueOnce(json({ ...proposal, status: "PRIVATE_STATE" }));
  vi.stubGlobal("fetch", fetchMock);
  const api = new ApiClient(baseUrl, { logger: { ...noopLogger, error, warn } });
  await expect(api.getCollaborationSwitchOperation(issueId, key, switchCommitInput)).resolves.toBeNull();
  await expect(api.switchCollaborationTarget(issueId, key, switchCommitInput)).rejects.toMatchObject({ status: 403, body: undefined, message: "Collaboration Switch request failed" });
  await expect(api.getCollaborationTargetSwitchProposal(issueId, targetId, proposalId)).rejects.toMatchObject({ status: 403, body: undefined, message: "Collaboration Switch request failed" });
  await expect(api.getCollaborationSwitchOperation(issueId, key, switchCommitInput)).resolves.toBeNull();
  await expect(api.getCollaborationTargetSwitchProposal(issueId, targetId, proposalId)).resolves.toBeNull();
  expect(JSON.stringify([...error.mock.calls, ...warn.mock.calls])).not.toContain("PRIVATE_");
});

it("fences late Switch reads, writes and refusals after credential replacement during body decoding", async () => {
  for (const kind of ["list", "get", "write", "refusal"] as const) {
    let resolve!: (value: unknown) => void;
    let started!: () => void;
    const decoding = new Promise<void>((done) => { started = done; });
    const body = new Promise<unknown>((done) => { resolve = done; });
    const response = new Response("", { status: kind === "refusal" ? 409 : 200 });
    vi.spyOn(response, "json").mockImplementation(() => { started(); return body; });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
    const api = new ApiClient(baseUrl);
    api.setToken("old-credential");
    const request = kind === "list" ? api.listCollaborationTargetSwitchProposals(issueId, targetId)
      : kind === "get" ? api.getCollaborationTargetSwitchProposal(issueId, targetId, proposalId)
      : api.switchCollaborationTarget(issueId, key, switchCommitInput);
    const pending = expect(request).rejects.toMatchObject({ name: "AbortError" });
    await decoding;
    api.setToken("new-credential");
    resolve(kind === "list" ? [proposal] : kind === "get" ? proposal : kind === "refusal" ? { ...receipt, status: 409, code: "stale_hold" } : receipt);
    await pending;
  }
});

it("validates intent and identifiers before transport and respects abort during body decoding", async () => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const api = new ApiClient(baseUrl);
  await expect(api.switchCollaborationTarget(issueId, "bad\nkey", switchCommitInput)).rejects.toThrow("Invalid collaboration Switch intent");
  await expect(api.switchCollaborationTarget("not-an-issue", key, switchCommitInput)).rejects.toThrow();
  await expect(api.getCollaborationTargetSwitchProposal(issueId, targetId, "invalid")).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
  const controller = new AbortController();
  const response = new Response("");
  vi.spyOn(response, "json").mockImplementation(async () => { controller.abort(); return receipt; });
  fetchMock.mockResolvedValueOnce(response);
  await expect(api.switchCollaborationTarget(issueId, key, switchCommitInput, { signal: controller.signal })).rejects.toMatchObject({ name: "AbortError" });
});
