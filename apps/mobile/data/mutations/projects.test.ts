import { QueryClient, type UseMutationOptions } from "@tanstack/react-query";
import type { Project, UpdateProjectRequest } from "@multica/core/types";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { projectKeys } from "../queries/projects";
import { allowProjectAccess, markProjectAccessDenied } from "../realtime/project-access";
import { useDeleteProject, useUpdateProject } from "./projects";

const state = vi.hoisted(() => ({ qc: null as QueryClient | null, options: null as unknown }));
vi.mock("@tanstack/react-query", async (original) => ({
  ...await original<typeof import("@tanstack/react-query")>(),
  useQueryClient: () => state.qc,
  useMutation: (options: unknown) => { state.options = options; return {}; },
}));
vi.mock("@/data/api", () => ({ api: { updateProject: vi.fn(), deleteProject: vi.fn() } }));
vi.mock("@/data/workspace-store", () => ({ useWorkspaceStore: (select: (value: { currentWorkspaceId: string }) => unknown) => select({ currentWorkspaceId: "mutation-workspace" }) }));
const project: Project = { id: "project", workspace_id: "mutation-workspace", title: "project", description: "goal",
  icon: null, status: "planned", priority: "none", lead_type: null, lead_id: null, start_date: null, due_date: null,
  created_at: "", updated_at: "", issue_count: 3, done_count: 2, resource_count: 0,
  revision: 2, description_revision: 1, completed_issue_count: 1, cancelled_issue_count: 1, open_issue_count: 1, statistics_complete: true };
type Context = { prevDetail?: Project; prevList?: Project[]; detailKey: readonly unknown[]; listKey: readonly unknown[]; wsId: string; optimistic: boolean };
type Options = UseMutationOptions<Project, Error, UpdateProjectRequest, Context>;
const mutationContext = { client: null, meta: undefined, mutationKey: undefined } as never;
beforeEach(() => {
  state.qc = new QueryClient();
  allowProjectAccess(project.workspace_id);
  state.qc.setQueryData(projectKeys.detail(project.workspace_id, project.id), project);
  state.qc.setQueryData(projectKeys.list(project.workspace_id), [project]);
  vi.clearAllMocks();
});
describe("mobile project mutation compatibility", () => {
  it.each(["project_permission_denied", "project_evidence_forbidden", "project_updates_disabled"])("keeps readable data and caller input after an operation 403 %s", async (code) => {
    useUpdateProject(project.id);
    const options = state.options as Options;
    const input = { description: "my unsaved text", expected_description_revision: 1 };
    const context = await options.onMutate!(input, mutationContext);
    await options.onError?.(Object.assign(new Error("denied"), { status: 403, body: { code } }), input, context, mutationContext);
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toEqual(project);
    expect(input).toEqual({ description: "my unsaved text", expected_description_revision: 1 });
  });
  it("retains old cache until versioned description/status writes succeed", async () => {
    useUpdateProject(project.id);
    const options = state.options as Options;
    await options.onMutate?.({ description: "draft", expected_description_revision: 1 }, mutationContext);
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toEqual(project);
    await options.onMutate?.({ status: "completed" }, mutationContext);
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toEqual(project);
  });
  it("retains optional P1 fields when an authoritative property response omits them", async () => {
    useUpdateProject(project.id);
    const options = state.options as Options;
    const { completed_issue_count: _completed, cancelled_issue_count: _cancelled, open_issue_count: _open, ...partial } = project;
    const context = await options.onMutate!({ title: "changed" }, mutationContext);
    await options.onSuccess?.({ ...partial, revision: 3, title: "changed" }, { title: "changed" }, context, mutationContext);
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toMatchObject({ title: "changed", completed_issue_count: 1, cancelled_issue_count: 1 });
  });
  it("does not roll back sensitive data after a 403", async () => {
    useUpdateProject(project.id);
    const options = state.options as Options;
    const patch = { title: "optimistic" };
    const context = await options.onMutate!(patch, mutationContext);
    markProjectAccessDenied(project.workspace_id);
    await options.onError?.(Object.assign(new Error("denied"), { status: 403 }), patch, context, mutationContext);
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toBeUndefined();
    await options.onSuccess?.({ ...project, revision: 3 }, patch, context, mutationContext);
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toBeUndefined();
  });
  it("does not erase detail or list when deletion fails", async () => {
    vi.mocked(api.deleteProject).mockRejectedValueOnce(new Error("offline"));
    useDeleteProject(project.id);
    const options = state.options as UseMutationOptions<string, Error, void>;
    expect(options.onMutate).toBeUndefined();
    expect(options.onSettled).toBeUndefined();
    await expect(options.mutationFn?.(undefined, mutationContext)).rejects.toThrow("offline");
    expect(state.qc?.getQueryData(projectKeys.detail(project.workspace_id, project.id))).toEqual(project);
  });
});
