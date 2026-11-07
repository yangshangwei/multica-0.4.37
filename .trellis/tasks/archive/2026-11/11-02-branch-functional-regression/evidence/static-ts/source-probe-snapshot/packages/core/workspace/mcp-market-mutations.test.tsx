// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, renderHook, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "../api";
import { useAssignWorkspaceMcpServer, useCreateWorkspaceMcpServerFromTemplate } from "./mutations";
import type { WorkspaceMcpServer } from "../types";

vi.mock("../api", () => ({
  api: { listAgentMcpServers: vi.fn(), addAgentMcpServer: vi.fn(), createWorkspaceMcpServerFromTemplate: vi.fn() },
}));

const server: WorkspaceMcpServer = {
  id: "server-1", workspace_id: "ws-1", name: "browser", transport: "stdio",
  enabled: false, created_at: "", updated_at: "",
};

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function setup() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return { qc, ...renderHook(() => useAssignWorkspaceMcpServer("ws-1"), { wrapper }) };
}

it("preserves disabled bindings, reports partial failure, and retries without duplicate writes", async () => {
  const attached = new Set(["existing"]);
  vi.spyOn(api, "listAgentMcpServers").mockImplementation(async (id) => attached.has(id) ? [server] : []);
  const add = vi.spyOn(api, "addAgentMcpServer").mockImplementation(async (id) => {
    if (id === "failed") throw new Error("Forbidden");
    attached.add(id);
    return [{ ...server, enabled: true }];
  });
  const { result, qc } = setup();
  qc.setQueryData(["agents", "new", "mcp-servers"], []);
  let outcome;
  await act(async () => {
    outcome = await result.current.mutateAsync({
      serverId: server.id, agentIds: ["existing", "new", "failed", "new"],
    });
  });
  expect(outcome).toEqual({
    succeeded: ["existing", "new"], failed: [{ agentId: "failed", message: "Forbidden" }],
  });
  expect(add).toHaveBeenCalledTimes(2);
  expect(qc.getQueryState(["agents", "new", "mcp-servers"])?.isInvalidated).toBe(true);
  add.mockClear();
  await act(async () => {
    await result.current.mutateAsync({ serverId: server.id, agentIds: ["existing", "new", "failed"] });
  });
  expect(add).toHaveBeenCalledTimes(1);
  expect(add).toHaveBeenCalledWith("failed", server.id, expect.objectContaining({ workspaceId: "ws-1" }));
  qc.clear();
});

it("cannot mark an assignment successful from an empty or foreign response", async () => {
  vi.spyOn(api, "listAgentMcpServers").mockResolvedValue([]);
  vi.spyOn(api, "addAgentMcpServer").mockImplementation(async (id) => (
    id === "empty" ? [] : [{ ...server, workspace_id: "other" }]
  ));
  const { result, qc } = setup();
  let outcome;
  await act(async () => {
    outcome = await result.current.mutateAsync({ serverId: server.id, agentIds: ["empty", "foreign"] });
  });
  expect(outcome).toMatchObject({
    succeeded: [], failed: [{ agentId: "empty" }, { agentId: "foreign" }],
  });
  qc.clear();
});

it("forwards template source and releases sensitive input variables after reset", async () => {
  vi.mocked(api.createWorkspaceMcpServerFromTemplate).mockResolvedValue(server);
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  const { result } = renderHook(() => useCreateWorkspaceMcpServerFromTemplate("ws-1"), { wrapper });
  const templateInputs = { token: "sensitive" };
  await act(async () => {
    await result.current.mutateAsync({ name: "search", templateKey: "search", templateVersion: "sha256:opaque", templateSource: "deployment", templateInputs });
  });
  expect(api.createWorkspaceMcpServerFromTemplate).toHaveBeenCalledWith("ws-1", "search", "search", "sha256:opaque", templateInputs, "deployment");
  expect(qc.getMutationCache().getAll()[0]?.options.gcTime).toBe(0);
  act(() => result.current.reset());
  await vi.waitFor(() => expect(qc.getMutationCache().getAll()).toHaveLength(0));
  qc.clear();
});
