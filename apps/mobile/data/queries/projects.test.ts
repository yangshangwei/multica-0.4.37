import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { projectDetailOptions, projectKeys, projectListOptions } from "./projects";
import { allowProjectAccess, projectAccessEpoch } from "../realtime/project-access";
vi.mock("@/data/api", () => ({ api: { listProjects: vi.fn(), getProject: vi.fn() } }));
afterEach(() => vi.clearAllMocks());
describe("mobile project queries", () => {
  it("clears cached workspace data and blocks a late read after middleware 404 revocation", async () => {
    const wsId = "middleware-revoked";
    allowProjectAccess(wsId);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(projectKeys.resources(wsId, "project"), [{ label: "secret" }]);
    qc.setQueryData(projectKeys.detail("other", "other-project"), { title: "visible" });
    let resolve!: (project: Awaited<ReturnType<typeof api.getProject>>) => void;
    vi.mocked(api.getProject).mockReturnValueOnce(new Promise((next) => { resolve = next; }));
    const pending = qc.fetchQuery(projectDetailOptions(wsId, "project"));
    const rejected = expect(pending).rejects.toThrow();
    vi.mocked(api.listProjects).mockRejectedValueOnce(Object.assign(new Error("workspace not found"), {
      status: 404, body: { error: "workspace not found", code: "workspace_access_denied" },
    }));
    await expect(qc.fetchQuery(projectListOptions(wsId))).rejects.toThrow();
    expect(qc.getQueryData(projectKeys.resources(wsId, "project"))).toBeUndefined();
    resolve({ id: "project", workspace_id: wsId, description: "secret" } as never);
    await rejected;
    expect(qc.getQueryData(projectKeys.detail(wsId, "project"))).toBeUndefined();
    expect(qc.getQueryData(projectKeys.detail("other", "other-project"))).toEqual({ title: "visible" });
  });
  it.each(["project_permission_denied", "project_evidence_forbidden", "project_updates_disabled"])("preserves readable caches for operation/source 403 %s", async (code) => {
    const wsId = `operation-${code}`;
    allowProjectAccess(wsId);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const project = { description: "still readable" };
    qc.setQueryData(projectKeys.detail(wsId, "project"), project);
    qc.setQueryData(projectKeys.resources(wsId, "project"), [{ label: "still readable" }]);
    const epoch = projectAccessEpoch(wsId);
    const error = Object.assign(new Error("denied"), { status: 403, body: { code } });
    vi.mocked(api.listProjects).mockRejectedValueOnce(error);
    await expect(qc.fetchQuery(projectListOptions(wsId))).rejects.toBe(error);
    expect(qc.getQueryData(projectKeys.detail(wsId, "project"))).toEqual(project);
    expect(qc.getQueryData(projectKeys.resources(wsId, "project"))).toEqual([{ label: "still readable" }]);
    expect(projectAccessEpoch(wsId)).toBe(epoch);
  });
  it("uses mobile flat lists and captured workspace + signal", async () => {
    const project = { id: "project", workspace_id: "workspace", completed_issue_count: 0 };
    vi.mocked(api.listProjects).mockResolvedValue({ projects: [project], total: 1 } as never);
    vi.mocked(api.getProject).mockResolvedValue(project as never);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    expect(await qc.fetchQuery(projectListOptions("workspace"))).toEqual([project]);
    expect(await qc.fetchQuery(projectDetailOptions("workspace", "project"))).toEqual(project);
    expect(api.listProjects).toHaveBeenCalledWith({ workspaceId: "workspace", signal: expect.any(AbortSignal) });
    expect(api.getProject).toHaveBeenCalledWith("project", { workspaceId: "workspace", signal: expect.any(AbortSignal) });
  });
  it("403 clears protected workspace data before retry or navigation", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(projectKeys.detail("workspace", "project"), { description: "secret" });
    qc.setQueryData(projectKeys.resources("workspace", "project"), [{ label: "secret" }]);
    qc.setQueryData(projectKeys.detail("other", "other-project"), { title: "visible" });
    vi.mocked(api.listProjects).mockRejectedValue(Object.assign(new Error("denied"), { status: 403 }));
    await expect(qc.fetchQuery(projectListOptions("workspace"))).rejects.toThrow();
    expect(qc.getQueryData(projectKeys.detail("workspace", "project"))).toBeUndefined();
    expect(qc.getQueryData(projectKeys.resources("workspace", "project"))).toBeUndefined();
    expect(qc.getQueryData(projectKeys.detail("other", "other-project"))).toEqual({ title: "visible" });
  });
});
