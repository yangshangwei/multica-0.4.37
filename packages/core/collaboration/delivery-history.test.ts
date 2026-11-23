// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, CollaborationUnavailableError } from "./access";
import { collaborationDeliveryHistoryOptions } from "./delivery-queries";
import type { CollaborationDeliveryHistory } from "./delivery-schema";
import { collaborationIssueFixture } from "./test-fixtures";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const issueId = auth.user.id;
const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const submission = { id: id(2), issue_id: issueId, event_seq: "3", lifecycle_epoch: "1", author_user_id: auth.user.id, author: null,
  created_at: "2026-11-03T00:00:00Z", result: "Result", basis: "Checks", attachment_ids: [], source_comment_id: null, evidence_state: "available" as const, decision: null };
const page: CollaborationDeliveryHistory = { issue_id: issueId, items: [submission], next_cursor: "second" };
const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200 });
afterEach(() => vi.unstubAllGlobals());

it("reads bounded delivery history with account-only headers and caller cancellation", async () => {
  const api = new ApiClient("https://example.test");
  const fetch = vi.fn().mockResolvedValue(json(page));
  vi.stubGlobal("fetch", fetch);
  const controller = new AbortController();
  const signal = controller.signal;
  expect(await api.getCollaborationDeliveryHistory(issueId, { limit: 1 }, { signal })).toEqual(page);
  expect(fetch.mock.calls[0]?.[0]).toBe(`https://example.test/api/collaboration/issues/${issueId}/acceptance-submissions?limit=1`);
  expect(new Headers(fetch.mock.calls[0]?.[1]?.headers).has("X-Workspace-ID")).toBe(false);
  controller.abort();
  expect(fetch.mock.calls[0]?.[1]?.signal.aborted).toBe(true);
});

it("keeps an older server's missing history route unavailable without erasing authorized issue data", async () => {
  setApiInstance(new ApiClient("https://example.test"));
  const client = new QueryClient();
  const scope = collaborationScope(auth.user.id);
  const key = collaborationKeys.issue(scope, issueId);
  client.setQueryData(key, collaborationIssueFixture);
  const fetch = vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
    .mockResolvedValueOnce(json(collaborationIssueFixture));
  vi.stubGlobal("fetch", fetch);
  try {
    await expect(client.fetchInfiniteQuery(collaborationDeliveryHistoryOptions(client, scope, issueId))).rejects.toBeInstanceOf(CollaborationUnavailableError);
    expect(collaborationIssueAccessDenied(client, scope, issueId)).toBe(false);
    expect(client.getQueryData(key)).toEqual(collaborationIssueFixture);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(fetch.mock.calls[1]?.[0]).toContain(`/issues/${issueId}?`);
  } finally { client.clear(); }
});

it("still erases protected issue data when history returns a structured object denial", async () => {
  setApiInstance(new ApiClient("https://example.test"));
  const client = new QueryClient();
  const scope = collaborationScope(auth.user.id);
  const key = collaborationKeys.issue(scope, issueId);
  client.setQueryData(key, collaborationIssueFixture);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "not_found" }), { status: 404 })));
  try {
    await expect(client.fetchInfiniteQuery(collaborationDeliveryHistoryOptions(client, scope, issueId))).rejects.toThrow();
    expect(collaborationIssueAccessDenied(client, scope, issueId)).toBe(true);
    expect(client.getQueryData(key)).toBeUndefined();
  } finally { client.clear(); }
});

it.each([
  { ...page, issue_id: id(9), items: [{ ...submission, issue_id: id(9) }] },
  { ...page, items: [{ ...submission, basis: null }] },
  { ...page, items: [submission, { ...submission, id: id(3), event_seq: "2" }] },
  { ...page, next_cursor: "original" },
])("rejects cross-issue, malformed, oversized or nonprogressing response pages", async (body) => {
  const api = new ApiClient("https://example.test");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json(body)));
  expect(await api.getCollaborationDeliveryHistory(issueId, { limit: 1, cursor: "original" })).toBeNull();
});

it("does not cache a forged cursor cycle or conflicting immutable result across pages", () => {
  setApiInstance(new ApiClient("https://example.test"));
  const options = collaborationDeliveryHistoryOptions(new QueryClient(), collaborationScope(auth.user.id), issueId);
  const share = options.structuralSharing;
  if (typeof share !== "function") throw new Error("Expected page-chain validation");
  expect(() => share(undefined, { pages: [page, { ...page, next_cursor: "second" }], pageParams: [undefined, "second"] })).toThrow();
  expect(() => share(undefined, { pages: [page, { ...page, next_cursor: null, items: [{ ...submission, result: "Changed result" }] }], pageParams: [undefined, "second"] })).toThrow();
  const second = { ...page, next_cursor: null, items: [{ ...submission, id: id(3), event_seq: "2" }] };
  expect(share(undefined, { pages: [page, second], pageParams: [undefined, "second"] })).toMatchObject({ pages: [page, second] });
  expect(share({ pages: [page] }, undefined)).toBeUndefined();
});
