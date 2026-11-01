// @vitest-environment node
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { focusManager, onlineManager, QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { bindKnowledgeProjectionLifecycle, expireKnowledgeProjections } from "./knowledge-cache";
import { collaborationKnowledgeCandidateOptions, collaborationKnowledgeCandidatesOptions, collaborationKnowledgeKeys, collaborationKnowledgeTargetOptions } from "./knowledge-queries";
import { knowledgeCandidatesFixture, knowledgeDetailFixture, knowledgeIds, knowledgeTargetFixture } from "./knowledge-test-fixtures";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const cleanups: (() => void)[] = [];
beforeEach(() => { vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "Date", "performance"] }); onlineManager.setOnline(true); focusManager.setFocused(true); });
afterEach(() => { cleanups.splice(0).reverse().forEach((dispose) => dispose()); vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const scope = collaborationScope(knowledgeIds.actor);
  cleanups.push(() => { expireKnowledgeProjections(client, scope, false); client.clear(); });
  const detail = { ...collaborationKnowledgeCandidateOptions(client, scope, knowledgeIds.candidate), gcTime: Infinity };
  return { client, scope, api, detail };
}
const json = (data: unknown) => new Response(JSON.stringify(data));

it("doesNotRestoreUnavailableKnowledgeFromLateResponse", async () => {
  const f = setup(); let finish!: (response: Response) => void; let signal: AbortSignal | undefined;
  vi.stubGlobal("fetch", vi.fn((_url, init) => { signal = init.signal; return new Promise<Response>((resolve) => { finish = resolve; }); }));
  const pending = f.client.fetchQuery(f.detail).catch((error: unknown) => error);
  expireKnowledgeProjections(f.client, f.scope, false);
  expect(signal?.aborted).toBe(true);
  finish(json(knowledgeDetailFixture)); await pending;
  expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  expect(JSON.stringify(f.client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain("PRIVATE");
});

it("erases all source/target/detail pages before refetch while preserving content-free recovery and other viewers", async () => {
  const f = setup(); vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(knowledgeDetailFixture)).mockResolvedValueOnce(json(knowledgeCandidatesFixture)).mockResolvedValueOnce(json(knowledgeTargetFixture)));
  await f.client.fetchQuery(f.detail);
  const source = { ...collaborationKnowledgeCandidatesOptions(f.client, f.scope, knowledgeIds.issue), gcTime: Infinity };
  const target = { ...collaborationKnowledgeTargetOptions(f.client, f.scope, knowledgeTargetFixture.target), gcTime: Infinity };
  await f.client.fetchQuery(source); await f.client.fetchQuery(target);
  const receipt = collaborationKnowledgeKeys.operation(f.scope, knowledgeIds.candidate, knowledgeIds.request);
  const other = collaborationKnowledgeKeys.candidate({ ...f.scope, userId: knowledgeIds.project }, knowledgeIds.candidate);
  f.client.setQueryData(receipt, { code: "knowledge_approved" }); f.client.setQueryData(other, { otherViewer: true });
  expireKnowledgeProjections(f.client, f.scope, false);
  for (const options of [f.detail, source, target]) expect(f.client.getQueryData(options.queryKey)).toBeUndefined();
  expect(f.client.getQueryData(receipt)).toEqual({ code: "knowledge_approved" }); expect(f.client.getQueryData(other)).toEqual({ otherViewer: true });
});

it("clears prior body at a failed refresh and never restores it on cancellation rollback", async () => {
  const f = setup(); const fetchMock = vi.fn().mockResolvedValueOnce(json(knowledgeDetailFixture)).mockRejectedValueOnce(new Error("offline")); vi.stubGlobal("fetch", fetchMock);
  await f.client.fetchQuery(f.detail); expect(f.client.getQueryData(f.detail.queryKey)?.value.candidate.availability).toBe("available");
  const pending = f.client.fetchQuery(f.detail).catch((error: unknown) => error);
  expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  expect(await pending).toBeInstanceOf(Error); expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  await f.client.cancelQueries({ queryKey: f.detail.queryKey });
  expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
});

it("expires at the earlier candidate expiry even while no hint or successful refresh arrives", async () => {
  const f = setup(); vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ ...knowledgeDetailFixture, next_expiry_at: "2026-11-01T00:00:07Z" })));
  await f.client.fetchQuery(f.detail);
  await vi.advanceTimersByTimeAsync(6999); expect(f.client.getQueryData(f.detail.queryKey)).toBeDefined();
  await vi.advanceTimersByTimeAsync(1); expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
});

it("cannot restore a private pre-refresh snapshot when the caller cancels with Query's default rollback", async () => {
  const f = setup(); let finish!: (value: typeof knowledgeDetailFixture) => void;
  vi.spyOn(f.api, "getCollaborationKnowledgeCandidate").mockResolvedValueOnce(knowledgeDetailFixture)
    .mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  await f.client.fetchQuery(f.detail);
  const pending = f.client.fetchQuery(f.detail).catch((error: unknown) => error);
  expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  await f.client.cancelQueries({ queryKey: f.detail.queryKey });
  expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  finish(knowledgeDetailFixture); await pending;
  expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
});

it("rejects requests older than 30 seconds and fences viewer replacement", async () => {
  const f = setup(); let finish!: (value: typeof knowledgeDetailFixture) => void;
  vi.spyOn(f.api, "getCollaborationKnowledgeCandidate").mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  const pending = f.client.fetchQuery(f.detail).catch((error: unknown) => error);
  await vi.advanceTimersByTimeAsync(30_001); finish(knowledgeDetailFixture);
  expect(await pending).toMatchObject({ name: "AbortError" }); expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  const replaced = f.client.fetchQuery(f.detail).catch((error: unknown) => error);
  setApiInstance(new ApiClient("https://different.example")); finish(knowledgeDetailFixture);
  expect(await replaced).toMatchObject({ name: "AbortError" }); expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
});

it("keeps paged reads on the first authorization deadline and refuses page requests without their boundary", async () => {
  const f = setup(); vi.spyOn(f.api, "getCollaborationKnowledgeCandidates").mockResolvedValue({ ...knowledgeCandidatesFixture, next_cursor: "next" });
  const root = { ...collaborationKnowledgeCandidatesOptions(f.client, f.scope, knowledgeIds.issue), gcTime: Infinity };
  const first = await f.client.fetchQuery(root); await vi.advanceTimersByTimeAsync(20_000);
  const next = { ...collaborationKnowledgeCandidatesOptions(f.client, f.scope, knowledgeIds.issue, { cursor: "next", boundary: first.boundary }), gcTime: Infinity };
  const page = await f.client.fetchQuery(next); expect(page.boundary).toBe(first.boundary);
  await vi.advanceTimersByTimeAsync(10_000);
  expect(f.client.getQueryData(root.queryKey)).toBeUndefined(); expect(f.client.getQueryData(next.queryKey)).toBeUndefined();
  expect(() => collaborationKnowledgeCandidatesOptions(f.client, f.scope, knowledgeIds.issue, { cursor: "next" })).toThrow(/boundary/);
});

it("uses one focus/offline responder and disposes it only after the last consumer", async () => {
  const f = setup(); vi.spyOn(f.api, "getCollaborationKnowledgeCandidate").mockResolvedValue(knowledgeDetailFixture);
  const first = bindKnowledgeProjectionLifecycle(f.client, f.scope), second = bindKnowledgeProjectionLifecycle(f.client, f.scope);
  cleanups.push(second); first();
  await f.client.fetchQuery(f.detail);
  focusManager.setFocused(false); expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
  focusManager.setFocused(true); await f.client.fetchQuery(f.detail);
  onlineManager.setOnline(false); expect(f.client.getQueryData(f.detail.queryKey)).toBeUndefined();
});
