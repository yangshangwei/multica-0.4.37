// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { ApiClient } from "./client";

const BASE = "https://api.example.test";
const PROJECT = {
  id: "project-1", workspace_id: "ws-1", title: "Launch",
  description: null, icon: null, status: "planned", priority: "none",
  lead_type: null, lead_id: null,
  created_at: "2026-09-12T00:00:00Z", updated_at: "2026-09-12T00:00:00Z",
};

function respond(body: unknown, status = 200) {
  const request = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", request);
  return request;
}

afterEach(() => {
  setCurrentWorkspace(null, null);
  vi.unstubAllGlobals();
});

describe("project execution API boundary", () => {
  it("reads older project responses without losing the project", async () => {
    respond(PROJECT);
    const result = await new ApiClient(BASE).getProject(PROJECT.id);
    expect(result).toMatchObject({ id: PROJECT.id, start_date: null, resource_count: 0 });
    expect(result.execution_squad).toBeNull();
    expect(result.execution_squads).toEqual([]);
  });

  it("normalizes a legacy default into the candidate list", async () => {
    const execution_squad = { state: "configured", squad_id: "squad-1" };
    respond({ ...PROJECT, execution_squad });
    const result = await new ApiClient(BASE).getProject(PROJECT.id);
    expect(result.execution_squads).toEqual([execution_squad]);
  });

  it("uses the first candidate as the default and preserves the order", async () => {
    const execution_squads = [
      { state: "configured", squad_id: "squad-1" },
      { state: "needs_runtime", template_key: "review" },
    ];
    respond({ ...PROJECT, execution_squad: { state: "configured", squad_id: "old-default" }, execution_squads });
    const result = await new ApiClient(BASE).getProject(PROJECT.id);
    expect(result.execution_squads).toEqual(execution_squads);
    expect(result.execution_squad).toEqual(execution_squads[0]);
  });

  it("keeps malformed candidates unavailable without dropping the valid candidates", async () => {
    const valid = { state: "configured", squad_id: "squad-1" };
    respond({ ...PROJECT, execution_squads: ["bad", valid, { state: "future_state" }] });
    const result = await new ApiClient(BASE).getProject(PROJECT.id);
    expect(result.execution_squads).toEqual([
      { state: "failed", error_code: "invalid_configuration" }, valid,
      { state: "failed", error_code: "invalid_configuration" },
    ]);
    expect(result.execution_squad?.state).toBe("failed");
  });

  it("does not restore a legacy default when the candidate list is explicitly empty", async () => {
    respond({ ...PROJECT, execution_squad: { state: "configured", squad_id: "old-default" }, execution_squads: [] });
    const result = await new ApiClient(BASE).getProject(PROJECT.id);
    expect(result.execution_squads).toEqual([]);
    expect(result.execution_squad).toBeNull();
  });

  it("fails closed for an unreadable candidate list", async () => {
    respond({ ...PROJECT, execution_squads: "bad" });
    const result = await new ApiClient(BASE).getProject(PROJECT.id);
    expect(result.execution_squads).toEqual([{ state: "failed", error_code: "invalid_configuration" }]);
  });

  it("replaces all candidate squads within the captured workspace", async () => {
    const execution_squads = [{ state: "configured", squad_id: "squad-1" }, { state: "needs_runtime", template_key: "review" }];
    const request = respond({ ...PROJECT, execution_squads });
    const squads = [{ squad_id: "squad-1" }, { template_key: "review" }];
    const result = await new ApiClient(BASE).configureProjectSquads(PROJECT.id, squads, { workspaceId: "ws-1" });
    expect(result.execution_squads).toEqual(execution_squads);
    expect(request).toHaveBeenCalledWith(`${BASE}/api/projects/project-1/execution-squads`, expect.objectContaining({
      method: "PUT", body: JSON.stringify({ squads }),
      headers: expect.objectContaining({ "X-Workspace-ID": "ws-1", "X-Workspace-Slug": "" }),
    }));
  });

  it("rejects a plural configuration response for another project", async () => {
    respond({ ...PROJECT, id: "another-project", execution_squads: [] });
    await expect(new ApiClient(BASE).configureProjectSquads(PROJECT.id, [], { workspaceId: "ws-1" })).rejects.toThrow();
  });

  it.each(["bad", { state: "configured", squad_id: 42 }, { state: "future_state" }])(
    "keeps a project visible but blocks malformed execution configuration %#",
    async (execution_squad) => {
      respond({ projects: [{ ...PROJECT, execution_squad }], total: 1 });
      const result = await new ApiClient(BASE).listProjects();
      expect(result.projects).toHaveLength(1);
      expect(result.projects[0]?.execution_squad).toMatchObject({
        state: "failed", error_code: "invalid_configuration",
      });
    },
  );

  it("pins configuration to its captured workspace and preserves a failed setup response", async () => {
    setCurrentWorkspace("another-workspace", "ws-other");
    const execution_squad = { state: "failed", template_key: "feature-delivery", error_code: "agent_name_conflict" };
    const request = respond({ ...PROJECT, execution_squad });
    const selection = { template_key: "feature-delivery", language: "zh" as const };
    const result = await new ApiClient(BASE).configureProjectSquad(PROJECT.id, selection, { workspaceId: "ws-1" });
    expect(result.execution_squad).toEqual(execution_squad);
    expect(request).toHaveBeenCalledWith(`${BASE}/api/projects/project-1/execution-squad`, expect.objectContaining({
      method: "PUT", body: JSON.stringify(selection),
      headers: expect.objectContaining({ "X-Workspace-ID": "ws-1", "X-Workspace-Slug": "" }),
    }));
  });

  it("rejects a configuration response belonging to a different project", async () => {
    respond({ ...PROJECT, id: "another-project" });
    await expect(new ApiClient(BASE).configureProjectSquad(PROJECT.id, {}, { workspaceId: "ws-1" })).rejects.toThrow();
  });

  it("rejects a project response from a different captured workspace", async () => {
    respond({ ...PROJECT, workspace_id: "ws-other" });
    await expect(new ApiClient(BASE).getProject(PROJECT.id, { workspaceId: "ws-1" })).rejects.toThrow();
  });

  it.each(["uppercase", "compact"])("accepts a canonical UUID response to a %s project reference", async (format) => {
    const id = "abcdefab-1111-4111-8111-111111111111";
    const workspaceId = "abcdefab-2222-4222-8222-222222222222";
    const reference = format === "uppercase" ? id.toUpperCase() : id.replaceAll("-", "");
    respond({ ...PROJECT, id, workspace_id: workspaceId });
    await expect(new ApiClient(BASE).getProject(reference, { workspaceId: workspaceId.toUpperCase() })).resolves.toMatchObject({ id });
  });

  it.each([null, {}, { ...PROJECT, id: "" }, { ...PROJECT, id: 42 }])(
    "does not navigate to an unusable creation response %#", async (body) => {
      respond(body, 201);
      await expect(new ApiClient(BASE).createProject({ title: "Launch" })).rejects.toThrow();
    },
  );

  it("sends the selection on creation and preserves the resource echo", async () => {
    const resources = [{ id: "resource-1", resource_type: "github_repo" }];
    const execution_squad = { state: "needs_runtime", template_key: "feature-delivery" };
    const request = respond({ ...PROJECT, resources, execution_squad }, 201);
    const data = { title: "Launch", execution_squad: { template_key: "feature-delivery" } };
    const result = await new ApiClient(BASE).createProject(data, { workspaceId: "ws-1" });
    expect(result).toMatchObject({ resources, execution_squad });
    expect(request).toHaveBeenCalledWith(`${BASE}/api/projects`, expect.objectContaining({
      body: JSON.stringify(data), headers: expect.objectContaining({ "X-Workspace-ID": "ws-1" }),
    }));
  });

  it("submits every selected squad when creating a project", async () => {
    const execution_squads = [{ state: "configured", squad_id: "squad-1" }, { state: "needs_runtime", template_key: "review" }];
    const request = respond({ ...PROJECT, execution_squads }, 201);
    const data = { title: "Launch", execution_squads: [{ squad_id: "squad-1" }, { template_key: "review" }] };
    const result = await new ApiClient(BASE).createProject(data, { workspaceId: "ws-1" });
    expect(result.execution_squads).toEqual(execution_squads);
    expect(request).toHaveBeenCalledWith(`${BASE}/api/projects`, expect.objectContaining({ body: JSON.stringify(data) }));
  });

  it("pins project resource reads to the project workspace", async () => {
    setCurrentWorkspace("other", "ws-other");
    const request = respond({ resources: [], total: 0 });
    await new ApiClient(BASE).listProjectResources(PROJECT.id, { workspaceId: "ws-1" });
    expect(request).toHaveBeenCalledWith(`${BASE}/api/projects/project-1/resources`, expect.objectContaining({
      headers: expect.objectContaining({ "X-Workspace-ID": "ws-1", "X-Workspace-Slug": "" }),
    }));
  });

  it.each([{ resources: "bad" }, { resources: [{ id: "wrong" }], total: 1 }])(
    "does not treat unreadable project resources as an unconstrained execution context %#", async (body) => {
      respond(body);
      await expect(new ApiClient(BASE).listProjectResources(PROJECT.id)).rejects.toThrow();
    },
  );

  it.each([{ daemon_id: 42, local_path: "/repo" }, { daemon_id: "machine-1", local_path: "" }])(
    "rejects an unreadable local-directory mapping instead of removing its constraint %#", async (resource_ref) => {
      respond({ resources: [{ id: "r1", project_id: PROJECT.id, workspace_id: PROJECT.workspace_id,
        resource_type: "local_directory", resource_ref }], total: 1 });
      await expect(new ApiClient(BASE).listProjectResources(PROJECT.id)).rejects.toThrow();
    },
  );
});
