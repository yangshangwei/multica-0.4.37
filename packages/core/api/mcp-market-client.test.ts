// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { WorkspaceMcpServerSchema } from "./schemas";

const server = {
  id: "server-1",
  workspace_id: "ws-1",
  name: "browser",
  transport: "stdio",
  template_key: "playwright",
  template_version: "1",
};

afterEach(() => {
  vi.unstubAllGlobals();
  setCurrentWorkspace(null, null);
});

it("creates a trusted recipe with the captured workspace and no config", async () => {
  setCurrentWorkspace("other", "ws-other");
  const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
    new Response(JSON.stringify(server), { status: 201 }),
  );
  vi.stubGlobal("fetch", fetchMock);
  await expect(
    new ApiClient("https://example.test").createWorkspaceMcpServerFromTemplate(
      "ws-1", "browser", "playwright", "1",
    ),
  ).resolves.toMatchObject(server);
  expect(fetchMock).toHaveBeenCalledWith(
    "https://example.test/api/workspaces/ws-1/mcp-servers",
    expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ name: "browser", template_key: "playwright", template_version: "1" }),
      headers: expect.objectContaining({ "X-Workspace-ID": "ws-1", "X-Workspace-Slug": "" }),
    }),
  );
});

it.each([
  {}, null, { ...server, id: "" }, { ...server, workspace_id: "other" },
  { ...server, template_key: null },
])("does not claim malformed creation succeeded %#", async (body) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), { status: 201 }),
  ));
  await expect(
    new ApiClient("https://example.test").createWorkspaceMcpServerFromTemplate(
      "ws-1", "browser", "playwright", "1",
    ),
  ).rejects.toThrow();
});

it("parses safe source identity while stripping raw configuration", () => {
  const parsed = WorkspaceMcpServerSchema.parse({ ...server, config: { env: { SECRET: "hidden" } } });
  expect(parsed).toMatchObject(server);
  expect(parsed).not.toHaveProperty("config");
  expect(WorkspaceMcpServerSchema.parse({
    ...server, template_key: 5, template_version: { bad: true },
  })).toMatchObject({ template_key: null, template_version: null });
});

it("keeps assignment reads and writes in the captured workspace", async () => {
  setCurrentWorkspace("other", "ws-other");
  const fetchMock = vi.fn<typeof fetch>().mockImplementation(async () => (
    new Response(JSON.stringify([{ ...server, enabled: false }]))
  ));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://example.test");
  await client.listAgentMcpServers("agent-1", { workspaceId: "ws-1" });
  await client.addAgentMcpServer("agent-1", "server-1", { workspaceId: "ws-1" });
  for (const [, init] of fetchMock.mock.calls) {
    expect(init?.headers).toMatchObject({ "X-Workspace-ID": "ws-1", "X-Workspace-Slug": "" });
  }
});
