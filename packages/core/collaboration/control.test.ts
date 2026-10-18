// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { defaultStorage } from "../platform/storage";
import { collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";
import { parseCollaborationIssue } from "./schema";
import { parseCollaborationActionDraft } from "./action";
import { readCollaborationDrafts, saveCollaborationDraft } from "./draft";
import { parseCollaborationControlDraft, prepareCollaborationControl, recoverCollaborationControl, sendCollaborationControl } from "./control";
import { collaborationControlReceiptFixture as receipt, collaborationControlRequestId as requestId, collaborationExecutionIssueFixture } from "./test-fixtures";

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
afterEach(() => { for (const client of clients.splice(0)) client.clear(); stored.clear(); vi.unstubAllGlobals(); });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
function setup() {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue({ ...collaborationExecutionIssueFixture, write_context: { participant_epoch: "1", account_epoch: "9007199254740999" } })!;
  const draft = prepareCollaborationControl(scope, issue, requestId, "resume", { scope: "issue" });
  return { api, client, scope, issue, draft };
}

it("captures one authorized snapshot and excludes hold/gate expectations from Stop", () => {
  const { scope, issue, draft } = setup();
  const input = parseCollaborationControlDraft(draft);
  expect(input).toMatchObject({ expected_lifecycle_epoch: issue.lifecycle_epoch, expected_account_epoch: "9007199254740999", expected_execution_epoch: "9007199254740997" });
  expect(input.targets[0]).toMatchObject({ expected_hold_version: "9007199254740993" });
  expect(issue.capabilities.control_ai).toBe(false);
  const stop = parseCollaborationControlDraft(prepareCollaborationControl(scope, issue, requestId, "stop", { scope: "target", targetId: issue.execution!.targets[0]!.id }));
  expect(stop.scope).toBe("target");
  expect(stop).not.toHaveProperty("expected_execution_epoch");
  expect(stop).not.toHaveProperty("expected_control_version");
  expect(stop.targets[0]).not.toHaveProperty("expected_hold_version");
  expect(() => parseCollaborationActionDraft(draft)).toThrow();
});

it("persists the immutable original intent before transport and updates state only by refetch", async () => {
  const { client, scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  const fetchMock = vi.fn().mockImplementation(async (_url: string, init: RequestInit) => {
    expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
    expect(JSON.parse(String(init.body))).toEqual(parseCollaborationControlDraft(draft));
    expect(client.getQueryData(queryKey)).toEqual(issue);
    return json(receipt);
  });
  vi.stubGlobal("fetch", fetchMock);
  await expect(sendCollaborationControl(client, scope, draft)).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(client.getQueryData(queryKey)).toEqual(issue);
  expect(client.getQueryState(queryKey)?.isInvalidated).toBe(true);
});

it("retains unknown requests and recovers their original refusal using GET alone", async () => {
  const { client, scope, draft } = setup();
  const refused = { ...receipt, status: 409, code: "stale_hold", targets: [], channels: [], target_holds: [] };
  const fetchMock = vi.fn().mockRejectedValueOnce(new TypeError("Network unavailable"))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json(refused));
  vi.stubGlobal("fetch", fetchMock);
  await expect(sendCollaborationControl(client, scope, draft)).rejects.toThrow("Network unavailable");
  const [retained] = readCollaborationDrafts(draft.scope);
  expect(retained).toEqual(draft);
  await expect(recoverCollaborationControl(client, scope, retained!)).rejects.toThrow("Collaboration response is unavailable");
  expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
  await expect(recoverCollaborationControl(client, scope, retained!)).resolves.toEqual(refused);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(fetchMock.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET", "GET"]);
  expect(fetchMock.mock.calls.slice(1).every(([url]) => url.endsWith(`/operations/${requestId}`))).toBe(true);
});

it("refuses rewritten keys, human-operation drafts and unavailable storage before transport", async () => {
  const { client, scope, draft } = setup();
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  saveCollaborationDraft(draft);
  await expect(sendCollaborationControl(client, scope, { ...draft, requestId: "00000000-0000-4000-8000-000000000099" })).rejects.toThrow();
  await expect(recoverCollaborationControl(client, scope, { ...draft, payload: { action: "transfer" } })).rejects.toThrow();
  await expect(sendCollaborationControl(client, scope, draft, { storage: { ...defaultStorage, keys: undefined } })).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
});

it("keeps an older access denial from erasing a recovered snapshot or emitting a late receipt", async () => {
  const { client, scope, issue, draft } = setup();
  let finish!: (value: Response) => void;
  const response = new Promise<Response>((resolve) => { finish = resolve; });
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(response));
  const pending = expect(sendCollaborationControl(client, scope, draft)).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, issue.id);
  const recovered = await protectCollaborationRead(client, scope, issue.id, async () => ({ title: "Recovered permission" }), true);
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, recovered);
  finish(json({ code: "forbidden", error: "PRIVATE_OLD_DENIAL" }, 403));
  await pending;
  expect(client.getQueryData(queryKey)).toEqual(recovered);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  await expect(protectCollaborationRead(client, scope, issue.id, async () => "still authorized")).resolves.toBe("still authorized");
});

it("isolates the submitted intent from caller edits while its response is pending", async () => {
  const { client, scope, draft } = setup();
  const originalScope = { ...draft.scope };
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = sendCollaborationControl(client, scope, draft);
  draft.requestId = "00000000-0000-4000-8000-000000000099";
  draft.scope.issueId = draft.requestId;
  draft.payload = { action: "stop" };
  finish(json(receipt));
  await expect(pending).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(originalScope)).toEqual([]);
});
