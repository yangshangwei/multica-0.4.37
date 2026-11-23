// @vitest-environment jsdom

import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import { workspaceKeys } from "@multica/core/workspace/queries";
import type { McpServerTemplate, WorkspaceMcpServer } from "@multica/core/types";
import enSettings from "../locales/en/settings.json";
import enAgents from "../locales/en/agents.json";
import { McpMarketPanel } from "./mcp-market-panel";

const api = vi.hoisted(() => ({
  getBaseUrl: () => "https://multica.test",
  listMembers: vi.fn(),
  listWorkspaceMcpServers: vi.fn(),
  listMcpServerTemplates: vi.fn(),
  createWorkspaceMcpServerFromTemplate: vi.fn(),
  createWorkspaceMcpServer: vi.fn(),
  listAgents: vi.fn(),
  addAgentMcpServer: vi.fn(),
}));

vi.mock("@multica/core/api", async (original) => ({
  ...await original<typeof import("@multica/core/api")>(),
  api,
}));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user" } };
  return { useAuthStore: Object.assign(
    (selector: (value: typeof state) => unknown) => selector(state),
    { getState: () => state },
  ) };
});
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const recipe: McpServerTemplate = {
  key: "playwright", title: "Playwright", description: "Browser tools",
  version: "1", source: "builtin", config: { command: "npx" },
};
const saved: WorkspaceMcpServer = {
  id: "saved", workspace_id: "ws", name: "playwright", transport: "stdio",
  template_key: "playwright", template_source: "builtin", template_version: "1",
  created_at: "", updated_at: "",
};

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>
      <I18nProvider locale="en" resources={{ en: { settings: enSettings, agents: enAgents } }}>
        {children}
      </I18nProvider>
    </QueryClientProvider>;
  }
  return { ...render(<McpMarketPanel workspaceId="ws" />, { wrapper: Wrapper }), client };
}

describe("McpMarketPanel", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    api.listMembers.mockResolvedValue([{ user_id: "user", role: "owner" }]);
    api.listWorkspaceMcpServers.mockResolvedValue([]);
    api.listMcpServerTemplates.mockResolvedValue([recipe]);
    api.listAgents.mockResolvedValue([]);
    api.createWorkspaceMcpServer.mockResolvedValue(saved);
    api.createWorkspaceMcpServerFromTemplate.mockImplementation(async () => {
      api.listWorkspaceMcpServers.mockResolvedValue([saved]);
      return saved;
    });
  });

  // Catalog identity/freshness and assignment matrices stay in mcp-market.test.tsx.
  // This suite covers the extracted panel's actual query/mutation lifetimes.
  it("keeps the first saved instance in its explicit assignment step after inventory invalidation", async () => {
    const user = userEvent.setup();
    const { client, rerender } = renderPanel();
    const card = await screen.findByRole("button", { name: "View configuration: Playwright" });
    expect(screen.queryByRole("tab")).toBeNull();
    expect(api.listAgents).not.toHaveBeenCalled();
    await user.click(card);
    await user.click(await screen.findByRole("button", { name: "Save and continue" }));
    const heading = await screen.findByRole("heading", { name: "Choose agents" });
    await waitFor(() => expect(client.getQueryData(workspaceKeys.mcpServers("ws"))).toEqual([saved]));
    expect(heading).toBeVisible();
    expect(heading).toHaveFocus();
    expect(api.createWorkspaceMcpServerFromTemplate).toHaveBeenCalledTimes(1);
    expect(api.createWorkspaceMcpServerFromTemplate).toHaveBeenCalledWith("ws", "playwright", "playwright", "1", undefined, "builtin");
    expect(api.addAgentMcpServer).not.toHaveBeenCalled();
    rerender(<McpMarketPanel workspaceId="ws" active={false} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    rerender(<McpMarketPanel workspaceId="ws" active />);
    expect(await screen.findByRole("heading", { name: "Choose agents" })).toBeVisible();
    expect(api.createWorkspaceMcpServerFromTemplate).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Skip for now" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(card).toHaveTextContent("playwright");
    await waitFor(() => expect(card).toHaveFocus());
  });

  it("lets ordinary members browse while retaining workspace creation restrictions", async () => {
    const user = userEvent.setup();
    api.listMembers.mockResolvedValue([{ user_id: "user", role: "member" }]);
    renderPanel();
    await user.click(await screen.findByRole("button", { name: "View configuration: Playwright" }));
    expect(screen.getByRole("heading", { name: "Set up Playwright" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Save and continue" })).toBeNull();
    expect(api.createWorkspaceMcpServerFromTemplate).not.toHaveBeenCalled();
  });

  it("does not refresh an inactive market until its type is reopened", async () => {
    const { client, rerender } = renderPanel();
    await screen.findByRole("button", { name: "View configuration: Playwright" });
    await waitFor(() => expect(client.isFetching()).toBe(0));
    const calls = api.listMcpServerTemplates.mock.calls.length;
    const memberCalls = api.listMembers.mock.calls.length;
    const inventoryCalls = api.listWorkspaceMcpServers.mock.calls.length;
    rerender(<McpMarketPanel workspaceId="ws" active={false} />);
    await act(async () => {
      await client.invalidateQueries();
    });
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(calls);
    expect(api.listMembers).toHaveBeenCalledTimes(memberCalls);
    expect(api.listWorkspaceMcpServers).toHaveBeenCalledTimes(inventoryCalls);
    rerender(<McpMarketPanel workspaceId="ws" active />);
    await waitFor(() => expect(api.listMcpServerTemplates.mock.calls.length).toBeGreaterThan(calls));
  });

  it("keeps discovery available when existing instances fail to load and enables saving after retry", async () => {
    const user = userEvent.setup();
    api.listWorkspaceMcpServers.mockRejectedValueOnce(new Error("Offline"));
    renderPanel();
    await screen.findByText(enSettings.mcp.market.workspace_error);
    await user.click(await screen.findByRole("button", { name: "View configuration: Playwright" }));
    expect(screen.queryByRole("button", { name: "Save and continue" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Close" }));
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByText(enSettings.mcp.market.workspace_error)).toBeNull());
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    expect(await screen.findByRole("button", { name: "Save and continue" })).toBeEnabled();
  });

  it("removes cached instance summaries and creation rights after membership access is denied", async () => {
    const user = userEvent.setup();
    api.listWorkspaceMcpServers.mockResolvedValue([saved]);
    const { client } = renderPanel();
    const card = await screen.findByRole("button", { name: "View configuration: Playwright" });
    await waitFor(() => expect(card).toHaveTextContent("playwright"));
    api.listMembers.mockRejectedValue(new ApiError("Forbidden", 403, ""));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.members("ws") }); });
    await screen.findByText(enSettings.query_feedback.access_denied);
    expect(card).not.toHaveTextContent("playwright");
    await user.click(card);
    expect(screen.queryByRole("button", { name: "Save and continue" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Use playwright" })).toBeNull();
  });

  it("uses the existing custom editor for unversioned builtin recipes and clears settled mutation inputs", async () => {
    const user = userEvent.setup();
    api.listMcpServerTemplates.mockResolvedValue([{ ...recipe, version: undefined }]);
    const { client, rerender } = renderPanel();
    const card = await screen.findByRole("button", { name: "View configuration: Playwright" });
    await user.click(card);
    await user.click(await screen.findByRole("button", { name: "Open custom configuration" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByLabelText("Server name")).toHaveValue("playwright");
    expect(within(dialog).getByLabelText("Command")).toHaveValue("npx");
    await user.type(within(dialog).getByLabelText("Command"), "-draft");
    rerender(<McpMarketPanel workspaceId="ws" active={false} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    rerender(<McpMarketPanel workspaceId="ws" active />);
    expect(await screen.findByLabelText("Command")).toHaveValue("npx-draft");
    await user.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(api.createWorkspaceMcpServer).toHaveBeenCalledWith("ws", "playwright", { command: "npx-draft" });
    expect(api.createWorkspaceMcpServerFromTemplate).not.toHaveBeenCalled();
    await waitFor(() => expect(client.getMutationCache().getAll().every((mutation) => mutation.state.variables === undefined)).toBe(true));
    await waitFor(() => expect(card).toHaveFocus());
  });

  it("returns to catalog search after closing a custom draft whose template was withdrawn", async () => {
    const user = userEvent.setup();
    api.listMcpServerTemplates.mockResolvedValue([{ ...recipe, version: undefined }]);
    const { client } = renderPanel();
    const card = await screen.findByRole("button", { name: "View configuration: Playwright" });
    await user.click(card);
    await user.click(await screen.findByRole("button", { name: "Open custom configuration" }));
    api.listMcpServerTemplates.mockResolvedValue([]);
    await act(async () => {
      await client.invalidateQueries({
        queryKey: workspaceKeys.mcpServerTemplates("ws", "en", api.getBaseUrl()),
        exact: true,
      });
    });
    await waitFor(() => expect(card).not.toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.getByRole("searchbox", { name: "Search MCP templates" })).toHaveFocus());
  });

  it("clears filters and an open configuration when the workspace identity changes", async () => {
    const user = userEvent.setup();
    const view = renderPanel();
    await screen.findByRole("button", { name: "View configuration: Playwright" });
    await user.type(screen.getByRole("searchbox"), "Playwright");
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.type(screen.getByRole("textbox", { name: "Configuration name" }), "-draft");
    view.rerender(<McpMarketPanel workspaceId="other" />);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("searchbox")).toHaveValue("");
    await waitFor(() => expect(api.listWorkspaceMcpServers).toHaveBeenCalledWith("other"));
  });
});
