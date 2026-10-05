import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { Project } from "@multica/core/types";
import projectInboxFixture from "./project-update-inbox.fixture.json";
import { QueryClient } from "@tanstack/react-query";
import { allowProjectAccess, isProjectAccessDenied, observeProjectAccess, onProjectAccessDenied, projectAccessEpoch } from "./realtime/project-access";

const workspace = vi.hoisted(() => ({ currentWorkspaceId: "11111111-1111-4111-8111-111111111111", currentWorkspaceSlug: "first" }));
vi.mock("./workspace-store", () => ({
  getCurrentSlug: () => workspace.currentWorkspaceSlug,
  useWorkspaceStore: { getState: () => workspace },
}));
const projectId = "22222222-2222-4222-8222-222222222222";
const oldProject: Project = {
  id: projectId, workspace_id: workspace.currentWorkspaceId, title: "Project", description: "Goal",
  icon: null, status: "planned", priority: "none", lead_type: null, lead_id: null,
  start_date: null, due_date: null, created_at: "2026-10-05T00:00:00Z", updated_at: "2026-10-05T00:00:00Z",
  issue_count: 5, done_count: 3, resource_count: 0,
};
const modernProject = { ...oldProject, revision: 2, description_revision: 1,
  statistics_complete: true, completed_issue_count: 2, cancelled_issue_count: 1, open_issue_count: 2 };
let api: typeof import("./api").api;
const fetchMock = vi.fn<typeof fetch>();
const respond = (body: unknown, status = 200) => fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(body), { status }));
beforeAll(async () => {
  vi.stubEnv("EXPO_PUBLIC_API_URL", "https://mobile.test");
  api = (await import("./api")).api;
});
beforeEach(() => {
  workspace.currentWorkspaceId = oldProject.workspace_id;
  workspace.currentWorkspaceSlug = "first";
  allowProjectAccess(oldProject.workspace_id);
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  vi.spyOn(console, "log").mockImplementation(() => {});
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });

describe("mobile project response contract", () => {
  it.each(["project_permission_denied", "project_evidence_forbidden", "project_updates_disabled"])("preserves readable workspace after operation-level 403 %s", async (code) => {
    const qc = new QueryClient();
    const key = ["projects", oldProject.workspace_id, "detail", projectId];
    qc.setQueryData(key, modernProject);
    const stop = observeProjectAccess(qc);
    const navigate = vi.fn();
    const stopNavigate = onProjectAccessDenied(navigate);
    const epoch = projectAccessEpoch(oldProject.workspace_id);
    try {
      respond({ code, message: "Operation denied" }, 403);
      await expect(api.deleteProject(projectId)).rejects.toMatchObject({ status: 403, body: { code } });
      expect(qc.getQueryData(key)).toEqual(modernProject);
      expect(projectAccessEpoch(oldProject.workspace_id)).toBe(epoch);
      expect(isProjectAccessDenied(oldProject.workspace_id)).toBe(false);
      expect(navigate).not.toHaveBeenCalled();
    } finally { stop(); stopNavigate(); }
  });
  it.each(["forbidden", undefined])("still revokes protected data for a scope 403 (%s)", async (code) => {
    const qc = new QueryClient();
    const key = ["projects", oldProject.workspace_id, "detail", projectId];
    qc.setQueryData(key, modernProject);
    const stop = observeProjectAccess(qc);
    try {
      respond(code ? { code } : {}, 403);
      await expect(api.getProject(projectId)).rejects.toMatchObject({ status: 403 });
      expect(qc.getQueryData(key)).toBeUndefined();
      expect(isProjectAccessDenied(oldProject.workspace_id)).toBe(true);
    } finally { stop(); }
  });
  it("does not cancel a readable in-flight project after an operation denial", async () => {
    let resolve!: (value: Response) => void;
    fetchMock.mockReturnValueOnce(new Promise((next) => { resolve = next; }));
    const read = api.getProject(projectId);
    respond({ code: "project_permission_denied" }, 403);
    await expect(api.deleteProject(projectId)).rejects.toMatchObject({ status: 403 });
    resolve(new Response(JSON.stringify(modernProject)));
    await expect(read).resolves.toMatchObject({ id: projectId, description: "Goal" });
  });
  it("keeps existing and P1 notifications through the real mobile inbox API parser", async () => {
    const broken = projectInboxFixture.map((item) => item.type === "project_update"
      ? { ...item, details: { ...item.details, revision: 1 } } : item);
    respond(broken);
    await expect(api.listInbox()).resolves.toEqual([]);
    respond(projectInboxFixture);
    const result = await api.listInbox();
    expect(result.map((item) => item.id)).toEqual(projectInboxFixture.map((item) => item.id));
    expect(result[0]?.details?.revision).toBe("1");
  });
  it("keeps legacy closure and distinguishes missing split counts from zero", async () => {
    respond(oldProject);
    expect(await api.getProject(projectId)).toMatchObject({ done_count: 3 });
    respond({ ...modernProject, completed_issue_count: 0, cancelled_issue_count: 3 });
    expect(await api.getProject(projectId)).toMatchObject({ done_count: 3, completed_issue_count: 0 });
    respond(oldProject);
    expect((await api.getProject(projectId)).completed_issue_count).toBeUndefined();
  });
  it.each([
    { ...modernProject, completed_issue_count: -1 },
    { ...modernProject, revision: Number.MAX_SAFE_INTEGER + 1 },
    { ...modernProject, id: "another-project" },
    { ...modernProject, workspace_id: "another-workspace" },
    { ...modernProject, id: "" },
  ])("rejects malformed/foreign project identity instead of empty success", async (body) => {
    respond(body);
    await expect(api.getProject(projectId)).rejects.toThrow();
  });
  it("validates create/update and preserves server P1 fields on a property write", async () => {
    respond({ ...modernProject, title: "Updated" });
    expect(await api.updateProject(projectId, { title: "Updated" })).toMatchObject({
      title: "Updated", description_revision: 1, completed_issue_count: 2,
    });
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ title: "Updated" });
    respond({ ...modernProject, id: "wrong" });
    await expect(api.updateProject(projectId, { title: "bad" })).rejects.toThrow();
    respond({ title: "missing identity" });
    await expect(api.createProject({ title: "new" })).rejects.toThrow();
  });
  it("rejects malformed list rather than presenting an empty workspace", async () => {
    respond({ projects: [{ ...modernProject, workspace_id: "foreign" }], total: 1 });
    await expect(api.listProjects()).rejects.toThrow();
    respond({ message: "wrong shape" });
    await expect(api.listProjects()).rejects.toThrow();
  });
  it("pins workspace identity in the request header", async () => {
    respond(modernProject);
    await api.getProject(projectId);
    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({ "X-Workspace-ID": oldProject.workspace_id, "X-Workspace-Slug": "" });
  });
  it("explains a 428 versioned description limitation without swallowing the error", async () => {
    respond({ code: "project_description_revision_required" }, 428);
    await expect(api.updateProject(projectId, { description: "unsaved" })).rejects.toMatchObject({ status: 428, message: expect.stringContaining("Web or Desktop") });
  });
  it("preserves explicit description revision and conflict details", async () => {
    respond({ code: "project_description_conflict", current: modernProject }, 409);
    await expect(api.updateProject(projectId, { description: "unsaved", expected_description_revision: 1 })).rejects.toMatchObject({ status: 409, body: { current: modernProject } });
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ description: "unsaved", expected_description_revision: 1 });
  });
  it("forwards cancellation and retains a 30 second request ceiling", async () => {
    vi.useFakeTimers();
    fetchMock.mockImplementation((_url, init) => new Promise((_resolve, reject) => {
      init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
    }));
    const controller = new AbortController();
    const cancelled = api.getProject(projectId, { signal: controller.signal });
    const rejected = expect(cancelled).rejects.toThrow("aborted");
    controller.abort();
    await rejected;
    const timed = expect(api.getProject(projectId)).rejects.toMatchObject({ status: 0 });
    await vi.advanceTimersByTimeAsync(30_000);
    await timed;
  });
  it("invokes the existing 401 sign-out callback", async () => {
    const onUnauthorized = vi.fn();
    api.setOptions({ onUnauthorized });
    respond({}, 401);
    await expect(api.getProject(projectId)).rejects.toMatchObject({ status: 401 });
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });
  it("only treats the dedicated capability 404 plus readable workspace as unsupported", async () => {
    respond({}, 404);
    respond({ id: oldProject.workspace_id });
    await expect(api.getProjectCapabilities()).resolves.toEqual({ supported: false });
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      `https://mobile.test/api/workspaces/${oldProject.workspace_id}/project-capabilities`,
      `https://mobile.test/api/workspaces/${oldProject.workspace_id}`,
    ]);
  });
  it.each([400, 401, 403, 500])("does not call capability HTTP %s unsupported", async (status) => {
    respond({}, status);
    await expect(api.getProjectCapabilities()).rejects.toMatchObject({ status });
    expect(fetchMock).toHaveBeenCalledOnce();
  });
  it("rejects deleted workspace, malformed capability and network errors", async () => {
    respond({}, 404); respond({}, 404);
    await expect(api.getProjectCapabilities()).rejects.toMatchObject({ status: 404 });
    respond({ workspace_id: oldProject.workspace_id, overview: true });
    await expect(api.getProjectCapabilities()).rejects.toThrow("Invalid project capability");
    fetchMock.mockRejectedValueOnce(new Error("offline"));
    await expect(api.getProjectCapabilities()).rejects.toThrow("offline");
  });
  it("parses supported capabilities without inventing mobile editing support", async () => {
    const capabilities = { workspace_id: oldProject.workspace_id, schema_version: 1,
      overview: true, updates: true, description_cas: true, planning_timezone: true };
    respond(capabilities);
    await expect(api.getProjectCapabilities()).resolves.toEqual({ supported: true, capabilities });
  });
  it("rejects a late successful HTTP response after access revocation", async () => {
    let resolve!: (value: Response) => void;
    fetchMock.mockReturnValueOnce(new Promise((next) => { resolve = next; }));
    const pending = api.getProject(projectId);
    const { markProjectAccessDenied } = await import("./realtime/project-access");
    markProjectAccessDenied(oldProject.workspace_id);
    resolve(new Response(JSON.stringify(modernProject)));
    await expect(pending).rejects.toMatchObject({ status: 403 });
  });
});
