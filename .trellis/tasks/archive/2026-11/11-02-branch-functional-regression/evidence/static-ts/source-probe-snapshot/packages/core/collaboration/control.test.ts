// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { defaultStorage } from "../platform/storage";
import { collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";
import { collaborationIssueOptions } from "./queries";
import { parseCollaborationIssue } from "./schema";
import { parseCollaborationActionDraft } from "./action";
import { readCollaborationDrafts, saveCollaborationDraft } from "./draft";
import { parseCollaborationControlDraft, prepareCollaborationControl, recoverCollaborationControl, sendCollaborationControl } from "./control";
import { collaborationControlReceiptFixture as receipt, collaborationControlRequestId as requestId, collaborationExecutionIssueFixture, collaborationHostExecutionFixture } from "./test-fixtures";

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
  expect(issue.capabilities).toMatchObject({ control_ai: true, submit_command: false });
  const stop = parseCollaborationControlDraft(prepareCollaborationControl(scope, issue, requestId, "stop", { scope: "target", targetId: issue.execution!.targets[0]!.id }));
  expect(stop.scope).toBe("target");
  expect(stop).not.toHaveProperty("expected_execution_epoch");
  expect(stop).not.toHaveProperty("expected_control_version");
  expect(stop.targets[0]).not.toHaveProperty("expected_hold_version");
  expect(() => parseCollaborationActionDraft(draft)).toThrow();
});

it("captures v6 comprehension without upgrading a retained v5 control request", async () => {
  const { client, scope, issue, draft } = setup();
  const current = parseCollaborationIssue({ ...issue, execution: collaborationHostExecutionFixture })!;
  const fresh = prepareCollaborationControl(scope, current, requestId, "resume", { scope: "issue" });
  expect(parseCollaborationControlDraft(fresh)).toHaveProperty("execution_view_version", 6);
  expect(parseCollaborationControlDraft(draft)).not.toHaveProperty("execution_view_version");
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("offline")).mockResolvedValueOnce(json(receipt));
  vi.stubGlobal("fetch", fetch);
  await expect(sendCollaborationControl(client, scope, fresh)).rejects.toThrow();
  const retained = readCollaborationDrafts(fresh.scope)[0]!;
  await expect(recoverCollaborationControl(client, scope, retained)).resolves.toEqual(receipt);
  expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toHaveProperty("execution_view_version", 6);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
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

it("settles only after the mounted issue re-read so the next control observes the advanced version", async () => {
  const { client, scope, issue, draft } = setup();
  const advanced = structuredClone(issue);
  advanced.session = { ...advanced.session, queue_state: "active", control_version: "9007199254740996" };
  advanced.execution!.control = { ...advanced.execution!.control, queue_state: "active", control_version: "9007199254740996" };
  let reread!: (value: Response) => void;
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(json(issue))
    .mockResolvedValueOnce(json(receipt))
    .mockReturnValueOnce(new Promise<Response>((resolve) => { reread = resolve; }));
  vi.stubGlobal("fetch", fetchMock);
  const options = { ...collaborationIssueOptions(client, scope, issue.id), refetchInterval: false as const };
  const unsubscribe = new QueryObserver(client, options).subscribe(() => {});
  try {
    await expect.poll(() => client.getQueryData(options.queryKey)).toEqual(issue);
    let settled = false;
    const pending = sendCollaborationControl(client, scope, draft).then((value) => { settled = true; return value; });
    await expect.poll(() => fetchMock.mock.calls.length).toBe(3);
    expect(settled).toBe(false);
    reread(json(advanced));
    await expect(pending).resolves.toEqual(receipt);
    const next = prepareCollaborationControl(scope, client.getQueryData(options.queryKey)!, requestId, "stop", { scope: "issue" });
    expect(parseCollaborationControlDraft(next)).toMatchObject({ expected_control_version: "9007199254740996" });
  } finally { unsubscribe(); }
});

it("keeps waiting for the fresh read when realtime invalidations restart it more than once", async () => {
  const { client, scope, issue, draft } = setup();
  const advanced = structuredClone(issue);
  advanced.session = { ...advanced.session, queue_state: "active", control_version: "9007199254740996" };
  advanced.execution!.control = { ...advanced.execution!.control, queue_state: "active", control_version: "9007199254740996" };
  const rereads: ((value: Response) => void)[] = [];
  const fetchMock = vi.fn().mockResolvedValueOnce(json(issue)).mockResolvedValueOnce(json(receipt))
    .mockImplementation(() => new Promise<Response>((resolve) => { rereads.push(resolve); }));
  vi.stubGlobal("fetch", fetchMock);
  const options = { ...collaborationIssueOptions(client, scope, issue.id), refetchInterval: false as const };
  const unsubscribe = new QueryObserver(client, options).subscribe(() => {});
  try {
    await expect.poll(() => client.getQueryData(options.queryKey)).toEqual(issue);
    let settled = false;
    const pending = sendCollaborationControl(client, scope, draft).then((value) => { settled = true; return value; });
    await expect.poll(() => rereads.length).toBe(1);
    // Two collaboration:changed ticks each restart the in-flight re-read.
    void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issue.id) });
    await expect.poll(() => rereads.length).toBe(2);
    void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issue.id) });
    await expect.poll(() => rereads.length).toBe(3);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(settled).toBe(false);
    rereads[2]!(json(advanced));
    await expect(pending).resolves.toEqual(receipt);
    expect(client.getQueryData(options.queryKey)).toEqual(advanced);
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
    const pending = expect(sendCollaborationControl(client, scope, draft)).rejects.toMatchObject({ name: "AbortError" });
    await expect.poll(() => fetchMock.mock.calls.length).toBe(3);
    clearCollaborationAccess(client, scope, issue.id);
    await pending;
    expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  } finally { unsubscribe(); }
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

it("clears the protected issue when control recovery receives a fresh denial", async () => {
  const { client, scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "forbidden" }, 403));
  vi.stubGlobal("fetch", fetch);
  await expect(recoverCollaborationControl(client, scope, draft)).rejects.toMatchObject({ status: 403 });
  expect(client.getQueryData(queryKey)).toBeUndefined();
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["GET"]);
});
