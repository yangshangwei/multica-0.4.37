// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess } from "./access";
import { collaborationIssueOptions } from "./queries";
import { parseCollaborationIssue } from "./schema";
import { collaborationExecutionIssueFixture, collaborationControlRequestId as requestId } from "./test-fixtures";
import { readCollaborationDrafts, saveCollaborationDraft } from "./draft";
import { executeCollaborationCommandCancel, parseCollaborationCommandCancelDraft, prepareCollaborationCommandCancel } from "./command";

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
afterEach(() => {
  for (const client of clients.splice(0)) client.clear();
  stored.clear();
  setSchemaLogger(noopLogger);
  vi.unstubAllGlobals();
});
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });

function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue({ ...collaborationExecutionIssueFixture,
    write_context: { participant_epoch: "2", account_epoch: "9007199254740999" } })!;
  const command = issue.execution!.commands.find((item) => item.state === "waiting")!;
  const draft = prepareCollaborationCommandCancel(scope, issue, requestId, command.id);
  const receipt = {
    protocol_version: 2, issue_id: issue.id, request_id: requestId, action: "cancel_command", status: 200, code: "command_cancelled",
    scope: "target", target_id: command.target_id, command_id: command.id, command_sequence: command.sequence,
    channel_id: command.channel_id, batch_id: command.batch_id, group_id: null, queue_state: "paused", control_version: "9007199254740995",
    group_settled: false, run_id: null, run_settled: false, grant_id: null, channels: [], target_holds: [],
  };
  return { client, scope, issue, command, draft, receipt };
}

it("captures the waiting input and exact authority without requiring execution readiness", () => {
  const { scope, issue, command, draft } = setup();
  expect(issue.execution?.availability).toBe("unavailable");
  expect(parseCollaborationCommandCancelDraft(draft)).toEqual({
    domain: "cancel_command", commandId: command.id,
    input: { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: "2", expected_account_epoch: "9007199254740999",
      expected_collaboration_revision: issue.collaboration_revision, target_id: command.target_id },
  });
  const running = issue.execution!.commands.find((item) => item.state === "in_flight")!;
  expect(() => prepareCollaborationCommandCancel(scope, issue, requestId, running.id)).toThrow();
});

it("persists before POST and retires only its matching receipt without projecting old controls", async () => {
  const { client, scope, issue, command, draft, receipt } = setup();
  const key = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(key, issue);
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    expect(url).toContain(`/commands/${command.id}/cancel`);
    expect(init.method).toBe("POST");
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(requestId);
    expect(JSON.parse(String(init.body))).toEqual(parseCollaborationCommandCancelDraft(draft).input);
    expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
    expect(client.getQueryData(key)).toEqual(issue);
    return json(receipt);
  });
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCommandCancel(client, scope, draft)).resolves.toEqual(receipt);
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(client.getQueryData(key)).toEqual(issue);
  expect(client.getQueryState(key)?.isInvalidated).toBe(true);
});

it("settles only after the mounted issue re-read so the next action observes fresh versions", async () => {
  const { client, scope, issue, draft, receipt } = setup();
  let reread!: (value: Response) => void;
  const fetch = vi.fn().mockResolvedValueOnce(json(issue)).mockResolvedValueOnce(json(receipt))
    .mockReturnValueOnce(new Promise<Response>((resolve) => { reread = resolve; }));
  vi.stubGlobal("fetch", fetch);
  const options = { ...collaborationIssueOptions(client, scope, issue.id), refetchInterval: false as const };
  const unsubscribe = new QueryObserver(client, options).subscribe(() => {});
  try {
    await expect.poll(() => client.getQueryData(options.queryKey)).toEqual(issue);
    let settled = false;
    const pending = executeCollaborationCommandCancel(client, scope, draft).then((value) => { settled = true; return value; });
    await expect.poll(() => fetch.mock.calls.length).toBe(3);
    expect(settled).toBe(false);
    reread(json(issue));
    await expect(pending).resolves.toEqual(receipt);
  } finally { unsubscribe(); }
});

it("does not hand back a receipt when access is lost while the fresh read is pending", async () => {
  const { client, scope, issue, draft, receipt } = setup();
  const fetch = vi.fn().mockResolvedValueOnce(json(issue)).mockResolvedValueOnce(json(receipt))
    .mockReturnValueOnce(new Promise<Response>(() => {}));
  vi.stubGlobal("fetch", fetch);
  const options = { ...collaborationIssueOptions(client, scope, issue.id), refetchInterval: false as const };
  const unsubscribe = new QueryObserver(client, options).subscribe(() => {});
  try {
    await expect.poll(() => client.getQueryData(options.queryKey)).toEqual(issue);
    const pending = expect(executeCollaborationCommandCancel(client, scope, draft)).rejects.toMatchObject({ name: "AbortError" });
    await expect.poll(() => fetch.mock.calls.length).toBe(3);
    clearCollaborationAccess(client, scope, issue.id);
    await pending;
    expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  } finally { unsubscribe(); }
});

it("keeps the original key across unknown writes and missing GET receipts without reposting", async () => {
  const { client, scope, draft, receipt } = setup();
  const refusal = { ...receipt, status: 409, code: "command_not_waiting" };
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("offline"))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json(refusal));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCommandCancel(client, scope, draft)).rejects.toThrow("offline");
  expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
  await expect(executeCollaborationCommandCancel(client, scope, draft, true)).rejects.toThrow("Collaboration response is unavailable");
  expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
  await expect(executeCollaborationCommandCancel(client, scope, draft, true)).resolves.toEqual(refusal);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET", "GET"]);
  expect(fetch.mock.calls.slice(1).every(([url]) => url.endsWith(`/commands/operations/${requestId}`))).toBe(true);
});

it.each([
  { protocol_version: 99 },
  { issue_id: "00000000-0000-4000-8000-000000000099" },
  { request_id: "00000000-0000-4000-8000-000000000099" },
  { command_id: "00000000-0000-4000-8000-000000000099" },
  { target_id: "00000000-0000-4000-8000-000000000099" },
  { command_sequence: 2 },
  { status: 409, code: "command_not_waiting" },
])("retains the request when the mutation receipt disagrees with its contract: %j", async (patch) => {
  const { client, scope, draft, receipt } = setup();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ ...receipt, ...patch })));
  await expect(executeCollaborationCommandCancel(client, scope, draft)).rejects.toThrow("Collaboration response is unavailable");
  expect(readCollaborationDrafts(draft.scope)).toEqual([draft]);
});

it("does not log malformed private receipt content", async () => {
  const { client, scope, draft, receipt } = setup();
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ ...receipt, code: "PRIVATE_CANCELLATION_PAYLOAD" })));
  await expect(executeCollaborationCommandCancel(client, scope, draft)).rejects.toThrow("Collaboration response is unavailable");
  expect(warn).toHaveBeenCalled();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_CANCELLATION_PAYLOAD");
});

it.each([false, true])("clears a cached issue when cancellation is authoritatively denied (recovery=%s)", async (recovery) => {
  const { client, scope, issue, draft } = setup();
  const key = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(key, issue);
  saveCollaborationDraft(draft);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ code: "forbidden" }, 403)));
  await expect(executeCollaborationCommandCancel(client, scope, draft, recovery)).rejects.toMatchObject({ status: 403 });
  expect(client.getQueryData(key)).toBeUndefined();
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(true);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
});

it("recovers a content-free receipt after known access loss without enabling writes", async () => {
  const { client, scope, issue, draft } = setup();
  clearCollaborationAccess(client, scope, issue.id);
  const receipt = { protocol_version: 2, issue_id: issue.id, request_id: requestId, action: "cancel_command",
    status: 200, code: "command_cancelled", redacted: true };
  const fetch = vi.fn().mockResolvedValueOnce(json(receipt));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCommandCancel(client, scope, draft, true)).resolves.toEqual(receipt);
  expect(fetch.mock.calls[0]?.[1]?.method ?? "GET").toBe("GET");
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(true);
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
});

it("rejects a late recovery receipt after permission changes", async () => {
  const { client, scope, issue, draft, receipt } = setup();
  let finish!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = expect(executeCollaborationCommandCancel(client, scope, draft, true)).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, issue.id);
  finish(json(receipt));
  await pending;
  expect(readCollaborationDrafts(draft.scope)).toEqual([]);
});
