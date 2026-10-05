/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import {
  getIssueSurfaceViewStore,
  pruneIssueSurfaceViewStates,
} from "../issues/stores/surface-view-store";
import { useDeleteProject, useUpdateProject } from "./mutations";
import { projectKeys } from "./queries";
import { p1Project } from "./test-fixtures/p1";

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe("useDeleteProject", () => {
  let qc: QueryClient;
  let deleteProject: ReturnType<typeof vi.fn<() => Promise<void>>>;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    deleteProject = vi.fn().mockResolvedValue(undefined);
    setApiInstance({ deleteProject } as unknown as ApiClient);
    setCurrentWorkspace("acme", "ws-1");
  });

  afterEach(() => {
    qc.clear();
    pruneIssueSurfaceViewStates([]);
    setCurrentWorkspace(null, null);
    vi.restoreAllMocks();
  });

  it("preserves detail, list, and view state until the server confirms deletion", async () => {
    let rejectDelete!: (error: Error) => void;
    deleteProject.mockImplementation(() => new Promise<void>((_resolve, reject) => { rejectDelete = reject; }));
    const project = { ...p1Project, id: "p1", workspace_id: "ws-1" };
    qc.setQueryData(projectKeys.list("ws-1"), { projects: [project], total: 1 });
    qc.setQueryData(projectKeys.detail("ws-1", "p1"), project);
    const store = getIssueSurfaceViewStore("project:p1");
    store.getState().setViewMode("list");
    const { result } = renderHook(() => useDeleteProject(), { wrapper: createWrapper(qc) });
    let pending!: Promise<unknown>;
    await act(async () => { pending = result.current.mutateAsync("p1").catch((error) => error); });
    const detailWhilePending = qc.getQueryData(projectKeys.detail("ws-1", "p1"));
    const listWhilePending = qc.getQueryData(projectKeys.list("ws-1"));
    await act(async () => { rejectDelete(new Error("forbidden")); await pending; });
    expect(detailWhilePending).toEqual(project);
    expect(listWhilePending).toEqual({ projects: [project], total: 1 });
    expect(qc.getQueryData(projectKeys.detail("ws-1", "p1"))).toEqual(project);
    expect(store.getState().viewMode).toBe("list");
  });

  it("clears the deleted project's issue surface view state", async () => {
    const store = getIssueSurfaceViewStore("project:p1");
    store.getState().setViewMode("list");
    expect(store.getState().viewMode).toBe("list");

    const { result } = renderHook(() => useDeleteProject(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync("p1");
    });

    expect(deleteProject).toHaveBeenCalledWith("p1", { workspaceId: "ws-1" });
    expect(store.getState().viewMode).toBe("board");
  });
});

// Description drafts stay outside authoritative caches until the CAS write succeeds.
describe("P1 versioned project updates", () => {
  it("does not optimistically expose a description that the server may reject", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const project = { ...p1Project, id: "p1", workspace_id: "ws-1" };
    let rejectWrite!: (error: Error) => void;
    const updateProject = vi.fn(() => new Promise<never>((_resolve, reject) => { rejectWrite = reject; }));
    setApiInstance({ updateProject } as unknown as ApiClient);
    qc.setQueryData(projectKeys.detail("ws-1", "p1"), project);
    qc.setQueryData(projectKeys.list("ws-1"), { projects: [project], total: 1 });
    const { result, unmount } = renderHook(() => useUpdateProject(), { wrapper: createWrapper(qc) });
    let pending!: Promise<unknown>;
    await act(async () => { pending = result.current.mutateAsync({ id: "p1", description: "Local draft" }).catch((error) => error); });
    const detailWhilePending = qc.getQueryData(projectKeys.detail("ws-1", "p1"));
    await act(async () => { rejectWrite(new Error("project_description_conflict")); await pending; });
    expect(detailWhilePending).toEqual(project);
    expect(qc.getQueryData(projectKeys.detail("ws-1", "p1"))).toEqual(project);
    unmount();
    qc.clear();
  });
});

describe("P1 revoked access barrier", () => {
  beforeEach(async () => { const { useProjectAccessStore } = await import("./access"); useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} }); });
  it.each(["success", "failure"])("does not restore protected caches after a late update %s", async (outcome) => {
    const qc = new QueryClient();
    const project = { ...p1Project, id: "revoked-project", workspace_id: "revoked-workspace", description: "private server text" };
    // The mocked hook is workspace ws-1; the API request captures that scope.
    project.workspace_id = "ws-1";
    qc.setQueryData(projectKeys.detail("ws-1", project.id), project);
    qc.setQueryData(projectKeys.list("ws-1"), { projects: [project], total: 1 });
    let resolve!: (value: unknown) => void; let reject!: (error: Error) => void;
    setApiInstance({ updateProject: vi.fn(() => new Promise((yes, no) => { resolve = yes; reject = no; })) } as unknown as ApiClient);
    const { result, unmount } = renderHook(() => useUpdateProject(), { wrapper: createWrapper(qc) });
    let pending!: Promise<unknown>;
    await act(async () => { pending = result.current.mutateAsync({ id: project.id, title: "private attempted title" }).catch((error) => error); });
    const { clearProtectedProjectContent, useProjectAccessStore } = await import("./access");
    act(() => clearProtectedProjectContent(qc, "ws-1"));
    await act(async () => { if (outcome === "success") resolve(project); else reject(new Error("failure")); await pending; });
    expect(qc.getQueryData(projectKeys.detail("ws-1", project.id))).toBeUndefined();
    expect(qc.getQueryData(projectKeys.list("ws-1"))).toBeUndefined();
    expect(JSON.stringify(qc.getMutationCache().getAll().map((mutation) => mutation.state))).not.toContain("private");
    unmount(); qc.clear(); useProjectAccessStore.setState({ denied: {} });
  });
});

it("RR01 an old-session optimistic mutation cannot roll back the new account's project", async () => {
  const { clearClientSessionData } = await import("../platform/session-cleanup");
  const { useProjectAccessStore } = await import("./access");
  useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  const qc = new QueryClient(); const oldProject = { ...p1Project, id: "p1", workspace_id: "ws-1", description: "old account text" };
  qc.setQueryData(projectKeys.detail("ws-1", "p1"), oldProject);
  let finish!: (value: unknown) => void;
  setApiInstance({ updateProject: vi.fn(() => new Promise((resolve) => { finish = resolve; })) } as unknown as ApiClient);
  const { result, unmount } = renderHook(() => useUpdateProject(), { wrapper: createWrapper(qc) });
  let pending!: Promise<unknown>;
  await act(async () => { pending = result.current.mutateAsync({ id: "p1", title: "old account edit" }).catch((error) => error); });
  act(() => clearClientSessionData(qc));
  const newProject = { ...oldProject, description: "new account authorized text" };
  qc.setQueryData(projectKeys.detail("ws-1", "p1"), newProject);
  await act(async () => { finish(oldProject); await pending; });
  expect(qc.getQueryData(projectKeys.detail("ws-1", "p1"))).toEqual(newProject);
  unmount(); qc.clear();
});
