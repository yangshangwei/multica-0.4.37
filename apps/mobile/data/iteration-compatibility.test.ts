// @vitest-environment node
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { buildIssueTextUpdate } from "./issue-edit";
import { InboxListSchema } from "./schemas";
import { deduplicateInboxItems } from "../lib/inbox-display";
import inboxFixture from "./project-update-inbox.fixture.json";

const workspace = vi.hoisted(() => ({ currentWorkspaceId: "11111111-1111-4111-8111-111111111111", currentWorkspaceSlug: "i1-mobile" }));
vi.mock("./workspace-store", () => ({ getCurrentSlug: () => workspace.currentWorkspaceSlug, useWorkspaceStore: { getState: () => workspace } }));
const iterationId = "22222222-2222-4222-8222-222222222222";
const issue = {
  id: "33333333-3333-4333-8333-333333333333", workspace_id: workspace.currentWorkspaceId,
  number: 1, identifier: "I1-1", title: "Mobile preserves server planning", description: "",
  status: "todo", priority: "none", assignee_type: null, assignee_id: null, creator_type: "member",
  creator_id: "44444444-4444-4444-8444-444444444444", parent_issue_id: null, project_id: null,
  position: 0, start_date: null, due_date: null, metadata: {}, properties: {},
  created_at: "2026-10-06T12:00:00Z", updated_at: "2026-10-06T12:00:00Z",
};
let api: typeof import("./api").api;
const fetchMock = vi.fn<typeof fetch>();
beforeAll(async () => {
  vi.stubEnv("EXPO_PUBLIC_API_URL", "https://mobile.test");
  api = (await import("./api")).api;
});
beforeEach(() => { fetchMock.mockReset(); vi.stubGlobal("fetch", fetchMock); });
afterEach(() => { vi.unstubAllGlobals(); });

describe("I1 installed mobile compatibility", () => {
  it("reads current planning but omits it from an ordinary text update", async () => {
    const current = { ...issue, current_iteration_id: iterationId, iteration_rollover_count: 3, revision: 7 };
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(current)));
    expect(await api.getIssue(issue.id)).toMatchObject(current);
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ ...current, title: "Edited from mobile" })));
    expect(await api.updateIssue(issue.id, buildIssueTextUpdate("Edited from mobile", ""))).toMatchObject({ current_iteration_id: iterationId, iteration_rollover_count: 3 });
    const request = fetchMock.mock.calls[1]![1]!;
    expect(request.method).toBe("PUT");
    expect(JSON.parse(String(request.body))).toEqual({ title: "Edited from mobile", description: "" });
  });
  it("preserves unknown planning on an old server instead of inventing zero rollover", async () => {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(issue)));
    const result = await api.getIssue(issue.id);
    expect(result.id).toBe(issue.id);
    expect(result.current_iteration_id).toBeUndefined();
    expect(result.iteration_rollover_count).toBeUndefined();
  });
  it("keeps an I1 notification alongside existing notifications without collapsing the list", () => {
    const notice = { ...inboxFixture[0]!, id: "55555555-5555-4555-8555-555555555555", type: "iteration", issue_id: null, details: { iteration_id: iterationId, kind: "end" } };
    const items = InboxListSchema.parse([notice, inboxFixture[1]!]);
    expect(deduplicateInboxItems(items)).toHaveLength(2);
    expect(items[0]).toMatchObject({ type: "iteration", details: { iteration_id: iterationId, kind: "end" } });
  });
});
