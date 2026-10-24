// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { setCurrentWorkspace } from "../platform/workspace-storage";

const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const metadata = { issue_id: id(1), identifier: "SRC-1", source_workspace_id: id(2), source_name: "Source", status: "done", status_category: "done",
  collaboration_revision: "9007199254740993", creator: { user_id: id(3), name: "Creator", avatar_url: "", disabled: true },
  handler: { user_id: id(4), name: "Handler", avatar_url: "", disabled: true }, queue_state: "closed", unsettled_runs: 1 };
const body = { action: "reopen" as const, expectedCollaborationRevision: "9007199254740993", expectedStatus: "done", targetStatus: "todo",
  statusOptionsVersion: "a".repeat(64), reason: "Restore responsibility", successorUserId: id(5) };
const result = { operation: { id: id(8), organization_id: id(9), target_id: id(1), actor_id: id(6), target_kind: "collaboration_issue",
  kind: "collaboration.reopen", state: "applied", result_code: "reopened", confirmation: "not_required", reconciliation_state: "complete", version: 1 },
  result: { issue_id: id(1), request_id: id(7), action: "reopen", code: "reopened", collaboration_revision: "9007199254740995", creator_user_id: id(3),
    previous_handler_user_id: id(4), handler_user_id: id(5), previous_status: "done", status: "todo", queue_state: "paused", control_version: "4", unsettled_runs: 1 } };
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });

it("uses protected account routes and exact paging without inheriting a stale workspace", async () => {
  const fetcher = vi.fn<typeof fetch>()
    .mockResolvedValueOnce(response({ scope: id(9), items: [metadata], next_cursor: id(1) }))
    .mockResolvedValueOnce(response({ ...metadata, scope: id(9), actions: [], successor_required: true }))
    .mockResolvedValueOnce(response({ scope: id(9), issue_id: id(1), collaboration_revision: metadata.collaboration_revision, items: [], next_cursor: null }));
  vi.stubGlobal("fetch", fetcher);
  setCurrentWorkspace("unrelated", id(20));
  const api = new ApiClient("https://admin.example");
  api.setToken("synthetic-human-session");
  expect((await api.getAdminCollaborationIssues({ limit: 25, cursor: id(10) }))?.items[0]?.issueId).toBe(id(1));
  expect((await api.getAdminCollaborationIssue(id(1)))?.successorRequired).toBe(true);
  expect((await api.getAdminCollaborationCandidates(id(1), { cursor: id(5) }))?.items).toEqual([]);
  expect(fetcher.mock.calls.map(([url]) => url)).toEqual([
    `https://admin.example/api/admin/collaboration/issues?limit=25&cursor=${id(10)}`,
    `https://admin.example/api/admin/collaboration/issues/${id(1)}`,
    `https://admin.example/api/admin/collaboration/issues/${id(1)}/reopen-candidates?cursor=${id(5)}`,
  ]);
  for (const [, request] of fetcher.mock.calls) {
    const headers = new Headers(request?.headers);
    expect(headers.get("Authorization")).toBe("Bearer synthetic-human-session");
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
  }
});

it("sends the exact successor confirmation, decimal revision and original key", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValueOnce(response(result));
  vi.stubGlobal("fetch", fetcher);
  const received = await new ApiClient("https://admin.example").changeAdminCollaborationLifecycle(id(1), body, id(7));
  expect(received?.result).toMatchObject({ collaborationRevision: "9007199254740995", handlerUserId: id(5), queueState: "paused", unsettledRuns: 1 });
  const [url, request] = fetcher.mock.calls[0]!;
  expect(url).toBe(`https://admin.example/api/admin/collaboration/issues/${id(1)}/lifecycle`);
  expect(request?.method).toBe("POST");
  expect(new Headers(request?.headers).get("Idempotency-Key")).toBe(id(7));
  expect(JSON.parse(String(request?.body))).toEqual({ action: "reopen", expected_collaboration_revision: "9007199254740993",
    expected_status: "done", target_status: "todo", status_options_version: "a".repeat(64), reason: body.reason, successor_user_id: id(5) });
});

it("recovers only the specified issue and original request without resubmitting", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValueOnce(response(result));
  vi.stubGlobal("fetch", fetcher);
  expect((await new ApiClient("https://admin.example").getAdminCollaborationOperation(id(1), id(7)))?.result.requestId).toBe(id(7));
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(fetcher.mock.calls[0]?.[0]).toBe(`https://admin.example/api/admin/collaboration/issues/${id(1)}/operations/${id(7)}`);
  expect(fetcher.mock.calls[0]?.[1]?.body).toBeUndefined();
});

it.each([{ request_id: id(10) }, { handler_user_id: id(4) }, { previous_status: "cancelled" }, { status: "in_progress" }])("keeps mismatched write receipts uncertain: %j", async (patch) => {
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValueOnce(response({ ...result, result: { ...result.result, ...patch } })));
  expect(await new ApiClient("https://admin.example").changeAdminCollaborationLifecycle(id(1), body, id(7))).toBeNull();
});

it("does not accept another issue's metadata, candidates or retained result", async () => {
  const fetcher = vi.fn<typeof fetch>()
    .mockResolvedValueOnce(response({ ...metadata, scope: id(9), actions: [], successor_required: true }))
    .mockResolvedValueOnce(response({ scope: id(9), issue_id: id(1), collaboration_revision: "1", items: [], next_cursor: null }))
    .mockResolvedValueOnce(response(result));
  vi.stubGlobal("fetch", fetcher);
  const api = new ApiClient("https://admin.example");
  expect(await api.getAdminCollaborationIssue(id(10))).toBeNull();
  expect(await api.getAdminCollaborationCandidates(id(10))).toBeNull();
  expect(await api.getAdminCollaborationOperation(id(10), id(7))).toBeNull();
});

it("rejects malformed pagination, identities and irrelevant successor input before HTTP", async () => {
  const fetcher = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetcher);
  const api = new ApiClient("https://admin.example");
  await expect(api.getAdminCollaborationIssues({ limit: 101 })).rejects.toThrow();
  await expect(api.getAdminCollaborationCandidates(id(1), { cursor: "all" })).rejects.toThrow();
  await expect(api.getAdminCollaborationIssue("../other")).rejects.toThrow();
  await expect(api.changeAdminCollaborationLifecycle(id(1), { ...body, action: "complete" }, id(7))).rejects.toThrow();
  await expect(api.getAdminCollaborationOperation(id(1), "new-key")).rejects.toThrow();
  expect(fetcher).not.toHaveBeenCalled();
});

it("retains current administration denial for the shared access guard", async () => {
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValueOnce(response({ code: "admin_scope_forbidden", error: "Outside scope" }, 403)));
  await expect(new ApiClient("https://admin.example").getAdminCollaborationIssue(id(1))).rejects.toMatchObject({ status: 403 });
});
