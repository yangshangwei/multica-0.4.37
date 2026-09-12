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
