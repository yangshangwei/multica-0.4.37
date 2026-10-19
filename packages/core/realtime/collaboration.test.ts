// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, collaborationScope, protectCollaborationRead } from "../collaboration/access";
import { collaborationIssueOptions } from "../collaboration/queries";
import { collaborationIssueFixture } from "../collaboration/test-fixtures";
import { executeCollaborationComment, prepareCollaborationComment } from "../collaboration/comment";
import { parseCollaborationTriggerPreview } from "../collaboration/content-schema";
import { readCollaborationDrafts, saveCollaborationDraft } from "../collaboration/draft";
import { defaultStorage } from "../platform/storage";
import type { WSEventType } from "../types/events";
import { bindCollaborationRealtime } from "./collaboration";
import { issueKeys } from "../issues/queries";

const actor = "00000000-0000-4000-8000-000000000001";
const issueId = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
const stored = vi.hoisted(() => new Map<string, string>());
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
vi.mock("../platform/storage", () => ({ defaultStorage: {
  keys: () => [...stored.keys()], getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => stored.set(key, value), removeItem: (key: string) => stored.delete(key),
} }));

const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).reverse().forEach((dispose) => dispose()); stored.clear(); vi.unstubAllGlobals(); });
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const change = (event_seq: string) => ({ issue_id: issueId, event_seq, collaboration_revision: "2", authority_epoch: "2" });
const receipt = { status: 201, code: "comment_created", protocol_version: 2, issue_id: issueId, comment_id: requestId, comment_revision: "1", event_id: requestId, event_seq: "3", targets: [] };

function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  cleanups.push(() => client.clear());
  const scope = collaborationScope(actor);
  const key = collaborationKeys.issue(scope, issueId);
  const issue = { ...collaborationIssueFixture, id: issueId, realtime: { event_seq: "1", authority_epoch: "1" } };
  client.setQueryData(key, issue);
  const handlers = new Map<WSEventType, (payload: unknown) => void>();
  let authenticated = () => {};
  const subscribeScope = vi.fn(() => () => {});
  const revalidateScope = vi.fn();
  cleanups.push(bindCollaborationRealtime(client, scope, issueId, {
    subscribe: (event, callback) => { handlers.set(event, callback); return () => { handlers.delete(event); }; },
    onAuthenticated: (callback) => { authenticated = callback; return () => {}; }, subscribeScope, revalidateScope,
  }));
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: issueId, lifecycle_epoch: "1", write_context: { participant_epoch: "1", account_epoch: "1" }, targets: [], recipients: [] })!;
  const draft = prepareCollaborationComment(scope, issueId, requestId, "Private draft", [], preview);
  return { client, scope, key, issue, draft, subscribeScope, revalidateScope, emit: (event: WSEventType, payload: unknown) => handlers.get(event)?.(payload), reconnect: () => authenticated() };
}

it("subscribes once and preserves an original comment POST through ordinary updates", async () => {
  const f = setup();
  const sourceKey = issueKeys.detail(f.issue.source_workspace.id, issueId);
  f.client.setQueryData(sourceKey, { id: issueId, title: "Source snapshot" });
  let finish!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = executeCollaborationComment(f.client, f.scope, f.draft);
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([f.draft]);
  f.emit("collaboration:changed", change("2"));
  expect(collaborationAccessEpoch(f.client, f.scope, issueId)).toBe(0);
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([f.draft]);
  finish(json(receipt));
  await expect(pending).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([]);
  expect(f.subscribeScope).toHaveBeenCalledExactlyOnceWith("issue", issueId);
  expect(f.client.getQueryState(sourceKey)?.isInvalidated).toBe(true);
  expect(f.client.getQueryData(sourceKey)).toEqual({ id: issueId, title: "Source snapshot" });
});

it("refreshes source windows from issue events without copying a collaboration DTO", () => {
  const f = setup();
  const workspaceId = f.issue.source_workspace.id;
  const keys = [issueKeys.detail(workspaceId, issueId), [...issueKeys.tableAll(workspaceId), "rows"],
    issueKeys.children(workspaceId, "parent"), issueKeys.childrenByParents(workspaceId, ["parent"]),
    issueKeys.projectGantt(workspaceId, "project"), ["projects", workspaceId], ["iterations", workspaceId]];
  for (const key of keys) f.client.setQueryData(key, { source: true });
  const other = issueKeys.detail("other-workspace", "other-issue");
  f.client.setQueryData(other, { source: "other" });
  f.emit("collaboration:changed", change("2"));
  for (const key of keys) {
    expect(f.client.getQueryState(key)?.isInvalidated).toBe(true);
    expect(f.client.getQueryData(key)).toEqual({ source: true });
  }
  expect(f.client.getQueryState(other)?.isInvalidated).toBe(false);
});

it("accepts only empty account invalidations and fences obsolete sessions", () => {
  const f = setup();
  const sourceKey = issueKeys.detail("source-workspace", issueId);
  f.client.setQueryData(sourceKey, { id: issueId });
  f.emit("collaboration:workbench_changed", { issue_id: issueId });
  expect(f.client.getQueryState(sourceKey)?.isInvalidated).toBe(false);
  f.emit("collaboration:workbench_changed", {});
  expect(f.client.getQueryState(sourceKey)?.isInvalidated).toBe(true);
  f.client.setQueryData(sourceKey, { id: issueId });
  setApiInstance(new ApiClient("https://another-server.example"));
  f.emit("collaboration:workbench_changed", {});
  expect(f.client.getQueryState(sourceKey)?.isInvalidated).toBe(false);
});

it("fences a pending POST and old GET before clearing revoked content and drafts", async () => {
  const f = setup();
  let finish!: (response: Response) => void;
  let oldGet!: (value: string) => void;
  vi.stubGlobal("fetch", vi.fn((_url, init) => init.method === "POST" ? new Promise<Response>((resolve) => { finish = resolve; }) : Promise.resolve(json({ code: "not_found" }, 404))));
  const pending = executeCollaborationComment(f.client, f.scope, f.draft);
  const oldRead = protectCollaborationRead(f.client, f.scope, issueId, () => new Promise<string>((resolve) => { oldGet = resolve; }));
  const cancelledEpochs: number[] = [];
  const cancel = f.client.cancelQueries.bind(f.client);
  vi.spyOn(f.client, "cancelQueries").mockImplementation((...args) => { cancelledEpochs.push(collaborationAccessEpoch(f.client, f.scope, issueId)); return cancel(...args); });
  f.emit("collaboration:access_changed", change("2"));
  expect(cancelledEpochs[0]).toBe(1);
  expect(f.client.getQueryData(f.key)).toBeUndefined();
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([]);
  finish(json(receipt)); oldGet("late private content");
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  await expect(oldRead).rejects.toMatchObject({ name: "AbortError" });
  expect(f.client.getQueryData(f.key)).toBeUndefined();
});

it("rebuilds source-member-only access after participant removal", async () => {
  const f = setup();
  const sourceOnly = { ...f.issue, write_context: null, realtime: { event_seq: "2", authority_epoch: "2" }, capabilities: {
    ...f.issue.capabilities, control_ai: false, submit_command: false, invite: false, transfer: false, remove_participants: false,
    edit: false, change_status: false, complete: false, cancel: false, reopen: false,
  } };
  vi.stubGlobal("fetch", vi.fn(async () => json(sourceOnly)));
  f.emit("collaboration:access_changed", change("2"));
  expect(f.client.getQueryData(f.key)).toBeUndefined();
  await vi.waitFor(() => expect(f.client.getQueryData(f.key)).toMatchObject({ write_context: null, capabilities: { read: true, control_ai: false } }));
  expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
});

it("preserves independent participation and pending intent when source membership is lost", async () => {
  const f = setup();
  saveCollaborationDraft(f.draft, defaultStorage);
  const current = { ...f.issue, capabilities: { ...f.issue.capabilities, edit_source_structure: false, reorder: false } };
  vi.stubGlobal("fetch", vi.fn(async () => json(current)));
  f.client.setQueryData([...f.key, "source-options"], { private: "source directory" });
  f.emit("collaboration:source_access_changed", { workspace_id: f.issue.source_workspace.id });
  expect(f.client.getQueryData([...f.key, "source-options"])).toBeUndefined();
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([f.draft]);
  await vi.waitFor(() => expect(f.client.getQueryData(f.key)).toMatchObject({ capabilities: { read: true, edit_source_structure: false } }));
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([f.draft]);
  expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
});

it("revalidates on reconnect with GETs only and clears a missed revocation", async () => {
  const f = setup();
  const fetch = vi.fn(async () => json({ code: "not_found" }, 404));
  vi.stubGlobal("fetch", fetch);
  const observer = new QueryObserver(f.client, { ...collaborationIssueOptions(f.client, f.scope, issueId), staleTime: Infinity });
  cleanups.push(observer.subscribe(() => {}));
  f.reconnect();
  await vi.waitFor(() => expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true));
  expect(f.client.getQueryData(f.key)).toBeUndefined();
  expect(fetch.mock.calls.length).toBeGreaterThan(0);
  for (const [, init] of fetch.mock.calls as unknown as [string, RequestInit][]) expect(init.method ?? "GET").toBe("GET");
});

it("ignores stale and malformed events without clearing a newer snapshot or its pending key", () => {
  const f = setup();
  f.client.setQueryData(f.key, { ...f.issue, realtime: { event_seq: "9007199254740994", authority_epoch: "2" } });
  saveCollaborationDraft(f.draft);
  f.emit("collaboration:access_changed", change("9007199254740993"));
  f.emit("collaboration:access_changed", { ...change("3"), event_seq: 3 });
  f.emit("subscribe_error", { scope: "issue", id: issueId, error: "lookup_failed" });
  f.emit("subscribe_error", { scope: "issue", id: issueId, error: "unknown_scope" });
  expect(collaborationAccessEpoch(f.client, f.scope, issueId)).toBe(0);
  expect(readCollaborationDrafts(f.draft.scope)).toEqual([f.draft]);
});

it("does not apply old-session callbacks after replacing the API instance", () => {
  const f = setup();
  setApiInstance(new ApiClient("https://replacement.example"));
  f.emit("collaboration:access_changed", change("2"));
  expect(f.client.getQueryData(f.key)).toEqual(f.issue);
  expect(collaborationAccessEpoch(f.client, f.scope, issueId)).toBe(0);
});

it("rechecks a denied issue subscription when account membership changes again", async () => {
  const f = setup();
  vi.stubGlobal("fetch", vi.fn(async () => json({ code: "not_found" }, 404)));
  f.emit("subscribe_error", { scope: "issue", id: issueId, error: "forbidden" });
  f.emit("collaboration:workbench_changed", {});
  expect(f.revalidateScope).toHaveBeenCalledExactlyOnceWith("issue", issueId);
  f.emit("subscribe_ack", { scope: "issue", id: issueId });
  f.emit("collaboration:workbench_changed", {});
  expect(f.revalidateScope).toHaveBeenCalledTimes(1);
  await vi.waitFor(() => expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true));
});
