import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { projectDetailOptions, projectKeys, projectListOptions } from "./projects";
vi.mock("@/data/api", () => ({ api: { listProjects: vi.fn(), getProject: vi.fn() } }));
afterEach(() => vi.clearAllMocks());
describe("mobile project queries", () => {
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
