// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { defaultStorage } from "../platform/storage";
import { collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";
import { collaborationIssueOptions } from "./queries";
import { parseCollaborationIssue } from "./schema";
import { parseCollaborationActionDraft } from "./action";
import { parseCollaborationControlDraft } from "./control";
import { readCollaborationDrafts, saveCollaborationDraft } from "./draft";
import { collaborationExecutionIssueFixture, collaborationHostExecutionFixture } from "./test-fixtures";
import { collaborationSwitchProposalOptions, collaborationSwitchProposalsOptions, parseCollaborationSwitchDraft,
  prepareCollaborationSwitch, recoverCollaborationSwitch, sendCollaborationSwitch } from "./switch";
import { switchIssueId as issueId, switchTargetId as targetId, switchProposalId as proposalId, switchRequestId as requestId,
  switchOtherId as otherId, switchResource as resource, switchProposal as proposal, switchReceipt as receipt } from "./switch-test-fixtures";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
const stored = vi.hoisted(() => new Map<string, string>());
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
vi.mock("../platform/storage", () => ({ defaultStorage: {
  getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => { stored.set(key, value); },
  removeItem: (key: string) => { stored.delete(key); },
  keys: () => [...stored.keys()],
} }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); stored.clear(); auth.user.id = issueId; vi.unstubAllGlobals(); });
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
function setup() {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue({ ...collaborationExecutionIssueFixture, write_context: { participant_epoch: "2", account_epoch: "9007199254740993" } })!;
  const draft = prepareCollaborationSwitch(scope, issue, requestId, { action: "commit_target_switch", targetId, proposalId }, proposal);
  return { api, client, scope, issue, draft };
}

it("captures authority and target observations from one snapshot in a distinct Switch draft", () => {
  const { scope, issue, draft } = setup();
  expect(parseCollaborationSwitchDraft(draft)).toMatchObject({ action: "commit_target_switch", target_id: targetId, proposal_id: proposalId,
    expected_lifecycle_epoch: "1", expected_participant_epoch: "2", expected_account_epoch: "9007199254740993",
    control: { target_id: targetId, expected_target_epoch: "9007199254740993", expected_config_version: "9007199254740993",
      expected_hold_version: "9007199254740993", expected_control_version: "9007199254740995", observed_group_generation: "1" } });
  expect(draft.payload).toMatchObject({ domain: "target_switch", requestId });
  expect(() => parseCollaborationActionDraft(draft)).toThrow();
  expect(() => parseCollaborationControlDraft(draft)).toThrow();
  expect(issue.capabilities).toMatchObject({ control_ai: true, submit_command: false });
  expect(() => prepareCollaborationSwitch(scope, { ...issue, execution: null }, requestId, { action: "commit_target_switch", targetId, proposalId })).toThrow();
  expect(() => prepareCollaborationSwitch(scope, issue, requestId, { action: "commit_target_switch", targetId: otherId, proposalId })).toThrow();
});

it("does not turn organization rights into another owner's consent", () => {
  const { scope, issue } = setup();
  const pending = { ...proposal, status: "pending" as const, resources: [{ ...proposal.resources[0]!, consented: false, owner_user_id: otherId }] };
  const selection = { action: "consent_target_switch" as const, targetId, proposalId, agentId: resource.agent_id,
    identity: { kind: "managed" as const, installation_id: issueId, binding_id: otherId, binding_epoch: "1" } };
  expect(() => prepareCollaborationSwitch(scope, issue, requestId, selection, pending)).toThrow();
  expect(() => prepareCollaborationSwitch(scope, { ...issue, capabilities: { ...issue.capabilities, invite: false } }, requestId,
    { action: "propose_target_switch", targetId, roster: [resource] })).toThrow();
  expect(() => prepareCollaborationSwitch(scope, issue, requestId, { action: "commit_target_switch", targetId, proposalId }, { ...proposal, status: "stale" })).toThrow();
  expect(() => prepareCollaborationSwitch(scope, issue, requestId, { action: "propose_target_switch", targetId, roster: [{ ...resource, agent_id: otherId }] })).toThrow();
});

it("requires explicit execution mode on a v6 Switch consent", () => {
  const { scope, issue } = setup();
  const hostIssue = parseCollaborationIssue({ ...issue, execution: collaborationHostExecutionFixture })!;
  const selection = { action: "consent_target_switch" as const, targetId, proposalId, agentId: resource.agent_id,
    identity: { kind: "managed" as const, installation_id: issueId, binding_id: otherId, binding_epoch: "1" } };
  expect(() => prepareCollaborationSwitch(scope, hostIssue, requestId, selection, proposal)).toThrow();
  expect(parseCollaborationSwitchDraft(prepareCollaborationSwitch(scope, issue, requestId, selection, proposal)).execution_view_version).toBeUndefined();
});

it("freezes proposal rosters and consent bindings without borrowing later live facts", () => {
  const { scope, issue } = setup();
  const roster = [{ ...resource }];
  const proposed = prepareCollaborationSwitch(scope, issue, requestId, { action: "propose_target_switch", targetId, roster });
  roster[0]!.runtime_id = otherId;
  roster.push({ ...resource, agent_id: otherId });
  expect(parseCollaborationSwitchDraft(proposed)).toMatchObject({ action: "propose_target_switch", roster: [resource] });
  const identity = { kind: "managed" as const, installation_id: issueId, binding_id: otherId, binding_epoch: "9007199254740999" };
  const consent = prepareCollaborationSwitch(scope, issue, requestId, { action: "consent_target_switch", targetId, proposalId, agentId: resource.agent_id, identity }, proposal);
  identity.binding_epoch = "1";
  expect(parseCollaborationSwitchDraft(consent)).toMatchObject({ identity: { binding_epoch: "9007199254740999" } });
  const revoked = prepareCollaborationSwitch(scope, issue, requestId, { action: "revoke_target_switch_consent", targetId, proposalId, agentId: resource.agent_id }, proposal);
  expect(parseCollaborationSwitchDraft(revoked)).toMatchObject({ action: "revoke_target_switch_consent", proposal_id: proposalId, agent_id: resource.agent_id });
});

it("persists before sending, keeps live state unchanged, then invalidates the issue and proposals", async () => {
  const { client, scope, issue, draft } = setup();
  const issueKey = collaborationKeys.issue(scope, issue.id);
  const proposalOptions = collaborationSwitchProposalsOptions(client, scope, issue.id, targetId);
  client.setQueryData(issueKey, issue);
  client.setQueryData(proposalOptions.queryKey, [proposal]);
  const fetchMock = vi.fn().mockImplementation(async () => {
    expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
    expect(client.getQueryData(issueKey)).toEqual(issue);
    return json(receipt);
  });
  vi.stubGlobal("fetch", fetchMock);
  await expect(sendCollaborationSwitch(client, scope, draft)).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(client.getQueryData(issueKey)).toEqual(issue);
  expect(client.getQueryData(proposalOptions.queryKey)).toEqual([proposal]);
  expect(client.getQueryState(issueKey)?.isInvalidated).toBe(true);
  expect(client.getQueryState(proposalOptions.queryKey)?.isInvalidated).toBe(true);
});

it("retains unresolved intents across reload and GET-only recovery until an original receipt arrives", async () => {
  const { client, scope, draft } = setup();
  const refused = { ...receipt, status: 409, code: "switch_consent_required" };
  const fetchMock = vi.fn().mockRejectedValueOnce(new TypeError("Network unavailable"))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json({ code: "operation_incomplete" }, 409))
    .mockResolvedValueOnce(json({ ...receipt, proposal_id: otherId }))
    .mockResolvedValueOnce(json(refused));
  vi.stubGlobal("fetch", fetchMock);
  await expect(sendCollaborationSwitch(client, scope, draft)).rejects.toThrow("Network unavailable");
  setApiInstance(new ApiClient("https://server.example"));
  const reloadedScope = collaborationScope(auth.user.id);
  const [retained] = readCollaborationDrafts(draft.scope);
  expect(retained).toEqual(draft);
  for (let attempt = 0; attempt < 3; attempt++) {
    await expect(recoverCollaborationSwitch(client, reloadedScope, retained!)).rejects.toThrow();
    expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
  }
  await expect(recoverCollaborationSwitch(client, reloadedScope, retained!)).resolves.toEqual(refused);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(fetchMock.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET", "GET", "GET", "GET"]);
  expect(fetchMock.mock.calls.slice(1).every(([url]) => url.endsWith(`/switch/operations/${requestId}`))).toBe(true);
});

it("settles only after the mounted issue re-read so the next action observes fresh versions", async () => {
  const { client, scope, issue, draft } = setup();
  let reread!: (value: Response) => void;
  const fetchMock = vi.fn().mockResolvedValueOnce(json(issue)).mockResolvedValueOnce(json(receipt))
    .mockReturnValueOnce(new Promise<Response>((resolve) => { reread = resolve; }));
  vi.stubGlobal("fetch", fetchMock);
  const options = { ...collaborationIssueOptions(client, scope, issue.id), refetchInterval: false as const };
  const unsubscribe = new QueryObserver(client, options).subscribe(() => {});
  try {
    await expect.poll(() => client.getQueryData(options.queryKey)).toEqual(issue);
    let settled = false;
    const pending = sendCollaborationSwitch(client, scope, draft).then((value) => { settled = true; return value; });
    await expect.poll(() => fetchMock.mock.calls.length).toBe(3);
    expect(settled).toBe(false);
    reread(json(issue));
    await expect(pending).resolves.toEqual(receipt);
  } finally { unsubscribe(); }
});

it("does not hand back a receipt when access is lost while the fresh read is pending", async () => {
  const { client, scope, issue, draft } = setup();
  const fetchMock = vi.fn().mockResolvedValueOnce(json(issue)).mockResolvedValueOnce(json(receipt))
    .mockReturnValueOnce(new Promise<Response>(() => {}));
  vi.stubGlobal("fetch", fetchMock);
  const options = { ...collaborationIssueOptions(client, scope, issue.id), refetchInterval: false as const };
  const unsubscribe = new QueryObserver(client, options).subscribe(() => {});
  try {
    await expect.poll(() => client.getQueryData(options.queryKey)).toEqual(issue);
    const pending = expect(sendCollaborationSwitch(client, scope, draft)).rejects.toMatchObject({ name: "AbortError" });
    await expect.poll(() => fetchMock.mock.calls.length).toBe(3);
    clearCollaborationAccess(client, scope, issue.id);
    await pending;
    expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  } finally { unsubscribe(); }
});

it("rejects rewritten requests, other domains and unsafe persistence before any network call", async () => {
  const { client, scope, draft } = setup();
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  saveCollaborationDraft(draft);
  await expect(sendCollaborationSwitch(client, scope, { ...draft, requestId: otherId })).rejects.toThrow();
  await expect(sendCollaborationSwitch(client, scope, { ...draft, payload: { domain: "execution_control", requestId, input: parseCollaborationSwitchDraft(draft) } })).rejects.toThrow();
  await expect(sendCollaborationSwitch(client, scope, draft, { storage: { ...defaultStorage, keys: undefined } })).rejects.toThrow();
  await expect(sendCollaborationSwitch(client, scope, draft, { storage: { ...defaultStorage, setItem: () => { throw new Error("Storage full"); } } })).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
  expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
});

it("isolates submitted data from caller edits while a response is pending", async () => {
  const { client, scope, draft } = setup();
  const originalScope = { ...draft.scope };
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = sendCollaborationSwitch(client, scope, draft);
  draft.requestId = otherId;
  draft.scope.issueId = otherId;
  draft.payload = { action: "propose_target_switch" };
  finish(json(receipt));
  await expect(pending).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(originalScope)).toEqual([]);
});

it("rejects stale account or server scope before sending and fences a late previous-account receipt", async () => {
  const { client, scope, draft } = setup();
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  await expect(sendCollaborationSwitch(client, scope, { ...draft, scope: { ...draft.scope, server: "https://other.example" } })).rejects.toThrow();
  await expect(sendCollaborationSwitch(client, scope, { ...draft, scope: { ...draft.scope, userId: otherId } })).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
  let finish!: (value: Response) => void;
  fetchMock.mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; }));
  const pending = expect(sendCollaborationSwitch(client, scope, draft)).rejects.toMatchObject({ name: "AbortError" });
  auth.user.id = otherId;
  finish(json(receipt));
  await pending;
  const nextScope = collaborationScope(auth.user.id);
  await expect(recoverCollaborationSwitch(client, nextScope, draft)).rejects.toThrow();
  expect(readCollaborationDrafts({ ...draft.scope, userId: otherId })).toEqual([]);
  expect(fetchMock).toHaveBeenCalledOnce();
});

it("does not let an old access denial erase a recovered snapshot or release a late receipt", async () => {
  const { client, scope, draft } = setup();
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = expect(sendCollaborationSwitch(client, scope, draft)).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, issueId);
  await protectCollaborationRead(client, scope, issueId, async () => true, true);
  const key = collaborationKeys.issue(scope, issueId);
  client.setQueryData(key, { title: "Recovered permission" });
  finish(json({ code: "forbidden", error: "PRIVATE_OLD_DENIAL" }, 403));
  await pending;
  expect(client.getQueryData(key)).toEqual({ title: "Recovered permission" });
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  await expect(protectCollaborationRead(client, scope, issueId, async () => true)).resolves.toBe(true);
});

it.each([false, true])("clears protected proposals and custom stored drafts after current permission denial (recovery=%s)", async (recovery) => {
  const { client, scope, draft } = setup();
  const privateStorage = new Map<string, string>();
  const storage = { keys: () => [...privateStorage.keys()], getItem: (key: string) => privateStorage.get(key) ?? null,
    setItem: (key: string, value: string) => { privateStorage.set(key, value); }, removeItem: (key: string) => { privateStorage.delete(key); } };
  const query = collaborationSwitchProposalOptions(client, scope, issueId, targetId, proposalId);
  client.setQueryData(query.queryKey, proposal);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "forbidden" }, 403)));
  await expect((recovery ? recoverCollaborationSwitch : sendCollaborationSwitch)(client, scope, draft, { storage })).rejects.toMatchObject({ status: 403 });
  expect(client.getQueryData(query.queryKey)).toBeUndefined();
  expect(privateStorage.size).toBe(0);
});

it("uses scoped proposal queries and prevents a delayed query from restoring revoked data", async () => {
  const { client, scope } = setup();
  const list = collaborationSwitchProposalsOptions(client, scope, issueId, targetId);
  const detail = collaborationSwitchProposalOptions(client, scope, issueId, targetId, proposalId);
  expect(list.queryKey).toEqual([...collaborationKeys.issue(scope, issueId), "switch-proposals", targetId]);
  expect(detail.queryKey).toEqual([...list.queryKey, proposalId]);
  const fetchMock = vi.fn().mockResolvedValueOnce(json([proposal])).mockResolvedValueOnce(json(proposal));
  vi.stubGlobal("fetch", fetchMock);
  await expect(client.fetchQuery(list)).resolves.toEqual([proposal]);
  await expect(client.fetchQuery(detail)).resolves.toEqual(proposal);
  let finish!: (value: Response) => void;
  fetchMock.mockReturnValueOnce(new Promise<Response>((resolve) => { finish = resolve; }));
  const pending = expect(client.fetchQuery(detail)).rejects.toThrow();
  clearCollaborationAccess(client, scope, issueId);
  finish(json(proposal));
  await pending;
  expect(client.getQueryData(list.queryKey)).toBeUndefined();
  expect(client.getQueryData(detail.queryKey)).toBeUndefined();
});

it("recovers only an explicitly redacted GET receipt after issue access loss", async () => {
  const { client, scope, draft } = setup();
  const redacted = { redacted: true, protocol_version: 2, issue_id: issueId, request_id: requestId, action: "commit_target_switch", status: 200, code: "target_switched" };
  clearCollaborationAccess(client, scope, issueId);
  const fetch = vi.fn().mockResolvedValueOnce(json(receipt)).mockResolvedValueOnce(json(redacted));
  vi.stubGlobal("fetch", fetch);
  await expect(sendCollaborationSwitch(client, scope, draft)).rejects.toMatchObject({ status: 403 });
  expect(fetch).not.toHaveBeenCalled();
  await expect(recoverCollaborationSwitch(client, scope, draft)).rejects.toThrow();
  await expect(recoverCollaborationSwitch(client, scope, draft)).resolves.toEqual(redacted);
  expect(fetch.mock.calls.every(([, init]) => (init.method ?? "GET") === "GET")).toBe(true);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  await expect(protectCollaborationRead(client, scope, issueId, async () => true)).rejects.toMatchObject({ status: 403 });
});
