// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { WorkspaceMcpServerSchema } from "./schemas";

const server = {
  id: "server-1", workspace_id: "ws-1", name: "browser", transport: "stdio",
  template_source: "deployment", template_key: "playwright", template_version: "sha256:opaque",
};

afterEach(() => vi.unstubAllGlobals());

it("sends source identity and accepts only the matching trusted creation response", async () => {
  const fetchMock = vi.fn<typeof fetch>().mockImplementation(async () => new Response(JSON.stringify(server), { status: 201 }));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://example.test");
  await expect(client.createWorkspaceMcpServerFromTemplate("ws-1", "browser", "playwright", "sha256:opaque", { token: "private" }, "deployment")).resolves.toMatchObject(server);
  expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
    name: "browser", template_source: "deployment", template_key: "playwright", template_version: "sha256:opaque", template_inputs: { token: "private" },
  });
  for (const template_source of [undefined, null, "builtin", "future"]) {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ ...server, template_source }), { status: 201 }));
    await expect(client.createWorkspaceMcpServerFromTemplate("ws-1", "browser", "playwright", "sha256:opaque", undefined, "deployment")).rejects.toThrow("could not be confirmed");
  }
});

it("negotiates source metadata for every MCP summary request", async () => {
  const fetchMock = vi.fn<typeof fetch>().mockImplementation(async (_url, init) => {
    const list = String(_url).includes("/agents/") || init?.method === undefined;
    return new Response(JSON.stringify(list ? [server] : server));
  });
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://example.test");
  await client.listWorkspaceMcpServers("ws-1");
  await client.createWorkspaceMcpServer("ws-1", "custom", { command: "node" });
  await client.createWorkspaceMcpServerFromTemplate("ws-1", "browser", "playwright", "sha256:opaque", undefined, "deployment");
  await client.updateWorkspaceMcpServer("ws-1", "server-1", { name: "renamed" });
  await client.listAgentMcpServers("agent-1");
  await client.addAgentMcpServer("agent-1", "server-1");
  await client.setAgentMcpServerEnabled("agent-1", "server-1", false);
  await client.removeAgentMcpServer("agent-1", "server-1");
  expect(fetchMock).toHaveBeenCalledTimes(8);
  for (const [url] of fetchMock.mock.calls) {
    expect(new URL(String(url)).searchParams.get("mcp_source_version")).toBe("1");
  }
});

it("defaults only absent summary sources to builtin and strips private values", () => {
  const { template_source: _source, ...legacy } = server;
  expect(WorkspaceMcpServerSchema.parse(legacy).template_source).toBe("builtin");
  for (const template_source of ["deployment", "future", null]) {
    expect(WorkspaceMcpServerSchema.parse({ ...server, template_source }).template_source).toBe(template_source);
  }
  expect(WorkspaceMcpServerSchema.parse({ ...legacy, template_source: 42 }).template_source).toBeNull();
  expect(WorkspaceMcpServerSchema.parse({ ...legacy, template_key: null }).template_source).toBeNull();
  const parsed = WorkspaceMcpServerSchema.parse({ ...server, command: "secret", config: { token: "secret" }, template_inputs: { token: "secret" } });
  expect(parsed).toMatchObject(server);
  expect(JSON.stringify(parsed)).not.toContain("secret");
});

it.each([
  [409, "mcp_template_changed"], [409, "mcp_template_unavailable"], [503, "mcp_catalog_unavailable"],
])("preserves stable recovery code %s/%s", async (status, code) => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => new Response(JSON.stringify({ error: "Cannot create", code }), { status })));
  const promise = new ApiClient("https://example.test").createWorkspaceMcpServerFromTemplate("ws-1", "browser", "playwright", "sha256:opaque", undefined, "deployment");
  await expect(promise).rejects.toBeInstanceOf(ApiError);
  await expect(promise).rejects.toMatchObject({ status, body: { code } });
});

it("reports malformed catalog responses as errors instead of authoritative removal", async () => {
  const fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://example.test");
  for (const response of [null, { templates: "bad" }, { templates: [{ key: "search", source: "deployment", transport: "http" }] }]) {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(response)));
    await expect(client.listMcpServerTemplates("ws-1", "zh")).rejects.toThrow();
  }
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ templates: null })));
  await expect(client.listMcpServerTemplates("ws-1", "zh")).resolves.toEqual([]);
});
