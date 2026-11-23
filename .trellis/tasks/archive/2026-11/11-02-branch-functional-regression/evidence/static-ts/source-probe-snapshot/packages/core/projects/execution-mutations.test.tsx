// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import type { Project } from "../types";
import { workspaceKeys } from "../workspace/queries";
import { projectKeys } from "./queries";
import { useConfigureProjectSquad, useConfigureProjectSquads } from "./mutations";

const PROJECT: Project = {
  id: "p1", workspace_id: "ws-1", title: "Launch", description: null, icon: null,
  status: "planned", priority: "none", lead_type: null, lead_id: null,
  start_date: null, due_date: null, created_at: "", updated_at: "",
  issue_count: 0, done_count: 0, resource_count: 0,
};

afterEach(() => vi.restoreAllMocks());

describe("project execution mutation", () => {
  it("replaces candidates and updates only the captured workspace after switching", async () => {
    let release!: (project: Project) => void;
    const configureProjectSquads = vi.fn().mockImplementation(() => new Promise<Project>((resolve) => { release = resolve; }));
    setApiInstance({ configureProjectSquads } as unknown as ApiClient);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(projectKeys.detail("ws-1", "p1"), PROJECT);
    qc.setQueryData(projectKeys.list("ws-1"), { projects: [PROJECT], total: 1 });
    qc.setQueryData(projectKeys.list("ws-other"), { projects: [], total: 0 });
    const related = [workspaceKeys.agents("ws-1"), workspaceKeys.squads("ws-1"), workspaceKeys.skills("ws-1")];
    for (const key of related) qc.setQueryData(key, []);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result, rerender } = renderHook(({ wsId }) => useConfigureProjectSquads(wsId), {
      wrapper, initialProps: { wsId: "ws-1" },
    });
    const squads = [{ squad_id: "s1" }, { template_key: "review" }];
    let saving!: Promise<Project>;
    act(() => { saving = result.current.mutateAsync({ id: "p1", squads }); });
    await waitFor(() => expect(configureProjectSquads).toHaveBeenCalledWith("p1", squads, { workspaceId: "ws-1" }));
    rerender({ wsId: "ws-other" });
    const updated: Project = { ...PROJECT, execution_squads: [
      { state: "configured", squad_id: "s1" },
      { state: "failed", template_key: "review", error_code: "preparation_failed" },
    ] };
    await act(async () => { release(updated); await saving; });
    expect(qc.getQueryData(projectKeys.detail("ws-1", "p1"))).toEqual(updated);
    expect(qc.getQueryData(projectKeys.list("ws-1"))).toEqual({ projects: [updated], total: 1 });
    expect(qc.getQueryData(projectKeys.detail("ws-other", "p1"))).toBeUndefined();
    expect(qc.getQueryState(projectKeys.list("ws-other"))?.isInvalidated).toBe(false);
    for (const key of related) expect(qc.getQueryState(key)?.isInvalidated).toBe(true);
    qc.clear();
  });

  it("keeps an in-flight configuration result in its original workspace after switching", async () => {
    let release!: (project: Project) => void;
    const configureProjectSquad = vi.fn().mockImplementation(() => new Promise<Project>((resolve) => { release = resolve; }));
    setApiInstance({ configureProjectSquad } as unknown as ApiClient);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(projectKeys.detail("ws-1", "p1"), PROJECT);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result, rerender } = renderHook(({ wsId }) => useConfigureProjectSquad(wsId), {
      wrapper, initialProps: { wsId: "ws-1" },
    });
    let saving!: Promise<Project>;
    act(() => { saving = result.current.mutateAsync({ id: "p1", template_key: "feature-delivery" }); });
    await waitFor(() => expect(configureProjectSquad).toHaveBeenCalled());
    rerender({ wsId: "ws-other" });
    const updated: Project = { ...PROJECT, execution_squad: { state: "needs_runtime", template_key: "feature-delivery" } };
    await act(async () => { release(updated); await saving; });
    expect(qc.getQueryData(projectKeys.detail("ws-1", "p1"))).toEqual(updated);
    expect(qc.getQueryData(projectKeys.detail("ws-other", "p1"))).toBeUndefined();
    qc.clear();
  });

  it("keeps the saved project on setup failure and refreshes related resource queries", async () => {
    const updated: Project = { ...PROJECT, execution_squad: {
      state: "failed", template_key: "feature-delivery", error_code: "agent_name_conflict",
    } };
    const configureProjectSquad = vi.fn().mockResolvedValue(updated);
    setApiInstance({ configureProjectSquad } as unknown as ApiClient);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(projectKeys.detail("ws-1", "p1"), PROJECT);
    qc.setQueryData(projectKeys.list("ws-1"), { projects: [PROJECT], total: 1 });
    qc.setQueryData(projectKeys.list("ws-other"), { projects: [], total: 0 });
    const affectedKeys = [workspaceKeys.agents("ws-1"), workspaceKeys.squads("ws-1"), workspaceKeys.skills("ws-1")];
    for (const key of affectedKeys) qc.setQueryData(key, []);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result } = renderHook(() => useConfigureProjectSquad("ws-1"), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ id: "p1", template_key: "feature-delivery" });
    });
    expect(configureProjectSquad).toHaveBeenCalledWith("p1", { template_key: "feature-delivery" }, { workspaceId: "ws-1" });
    expect(qc.getQueryData(projectKeys.detail("ws-1", "p1"))).toEqual(updated);
    expect(qc.getQueryData(projectKeys.list("ws-1"))).toEqual({ projects: [updated], total: 1 });
    expect(qc.getQueryState(projectKeys.list("ws-other"))?.isInvalidated).toBe(false);
    for (const key of affectedKeys) expect(qc.getQueryState(key)?.isInvalidated).toBe(true);
    qc.clear();
  });
});
