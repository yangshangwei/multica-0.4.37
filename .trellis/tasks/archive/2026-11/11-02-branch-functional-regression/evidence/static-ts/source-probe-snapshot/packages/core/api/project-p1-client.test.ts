// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { p1Overview, p1Preview, p1WriteResult, P1_PROJECT_ID, P1_WORKSPACE_ID, P1_UPDATE_ID } from "../projects/test-fixtures/p1";
const base = "https://api.example.test";
afterEach(() => vi.unstubAllGlobals());
function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }); }
function respond(body: unknown) { const fetcher = vi.fn<typeof fetch>().mockResolvedValue(response(body)); vi.stubGlobal("fetch", fetcher); return fetcher; }
describe("P1 protected API boundaries", () => {
  it("sends the clicked overview version using the risk wire parameter", async () => {
    const fetcher = respond({ workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, signal: "blocked", items: [], total: 0, snapshot_version: "B", refreshed: true, overview: p1Overview, next_cursor: null });
    await new ApiClient(base).getProjectRiskIssues(P1_WORKSPACE_ID, P1_PROJECT_ID, { signal: "blocked", version: "A" });
    const url = new URL(String(fetcher.mock.calls[0]?.[0]));
    expect(url.searchParams.get("snapshot_version")).toBe("A");
    expect(url.searchParams.has("version")).toBe(false);
  });
  it("opens execution evidence only through the revision-scoped authorized reader", async () => {
    const taskId = "66666666-6666-4666-8666-666666666666";
    const payload = { workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, update_id: P1_UPDATE_ID, revision: 2,
      task: { id: taskId, status: "completed" }, messages: [{ task_id: taskId, issue_id: "", seq: 1, type: "text", content: "Authorized result" }] };
    const fetcher = respond(payload);
    const result = await new ApiClient(base).getProjectExecutionEvidence(P1_WORKSPACE_ID, P1_PROJECT_ID, P1_UPDATE_ID, 2, taskId);
    expect(result.messages[0]?.content).toBe("Authorized result");
    expect(String(fetcher.mock.calls[0]?.[0])).toContain(`/updates/${P1_UPDATE_ID}/revisions/2/executions/${taskId}`);
    respond({ ...payload, task: { id: P1_UPDATE_ID, status: "completed" } });
    await expect(new ApiClient(base).getProjectExecutionEvidence(P1_WORKSPACE_ID, P1_PROJECT_ID, P1_UPDATE_ID, 2, taskId)).rejects.toThrow();
  });
  it("captures workspace and forwards cancellation for overview", async () => {
    const fetcher = respond(p1Overview); const controller = new AbortController(); const signal = controller.signal;
    expect(await new ApiClient(base).getProjectOverview(P1_WORKSPACE_ID, P1_PROJECT_ID, { signal })).toEqual(p1Overview);
    expect(fetcher).toHaveBeenCalledWith(`${base}/api/projects/${P1_PROJECT_ID}/overview`, expect.objectContaining({ signal: expect.any(AbortSignal), headers: expect.objectContaining({ "X-Workspace-ID": P1_WORKSPACE_ID }) }));
    controller.abort();
    expect((fetcher.mock.calls[0]?.[1]?.signal as AbortSignal).aborted).toBe(true);
  });
  it.each([
    { ...p1Overview, workspace_id: P1_PROJECT_ID },
    { ...p1Overview, statistics: { ...p1Overview.statistics, project_id: P1_WORKSPACE_ID } },
    { ...p1Overview, statistics: { ...p1Overview.statistics, counts: { ...p1Overview.statistics.counts, total: -1 } } },
    { ...p1Overview, statistics: { ...p1Overview.statistics, complete: false, health: "clear" } },
    { ...p1Overview, statistics: { ...p1Overview.statistics, statistics: p1Overview.statistics } },
    { workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID },
  ])("rejects malformed or mismatched overview %#", async (body) => {
    respond(body); await expect(new ApiClient(base).getProjectOverview(P1_WORKSPACE_ID, P1_PROJECT_ID)).rejects.toThrow();
  });
  it("only treats dedicated capability 404 with a still-readable workspace as unsupported", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValueOnce(response({ error: "not found" }, 404)).mockResolvedValueOnce(response({ id: P1_WORKSPACE_ID, name: "Workspace", slug: "workspace" }));
    vi.stubGlobal("fetch", fetcher);
    expect(await new ApiClient(base).getProjectCapabilities(P1_WORKSPACE_ID)).toBeNull();
    expect(fetcher.mock.calls[0]?.[0]).toBe(`${base}/api/workspaces/${P1_WORKSPACE_ID}/project-capabilities`);
  });
  it.each([400, 401, 403, 503])("does not hide capability HTTP %i as unsupported", async (status) => {
    vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(response({ error: "failure" }, status)));
    await expect(new ApiClient(base).getProjectCapabilities(P1_WORKSPACE_ID)).rejects.toThrow();
  });
  it("does not accept malformed capability success", async () => {
    respond({ workspace_id: P1_WORKSPACE_ID, overview: true });
    await expect(new ApiClient(base).getProjectCapabilities(P1_WORKSPACE_ID)).rejects.toThrow();
  });
  it("does not present a malformed preview or write result as a successful publication", async () => {
    respond({ ...p1Preview, recipients: null });
    await expect(new ApiClient(base).previewProjectUpdate(P1_WORKSPACE_ID, P1_PROJECT_ID, p1Preview.draft as never)).rejects.toThrow();
    respond({ ...p1WriteResult, result_revision: 0 });
    await expect(new ApiClient(base).createProjectUpdate(P1_WORKSPACE_ID, P1_PROJECT_ID, {} as never)).rejects.toThrow();
  });
});
