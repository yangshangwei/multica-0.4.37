// @vitest-environment jsdom
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import enSettings from "../locales/en/settings.json";
import enAgents from "../locales/en/agents.json";
import { McpAgentDiscovery } from "./mcp-agent-discovery";

const mocks = vi.hoisted(() => ({
  assign: vi.fn(),
  create: vi.fn(),
  recipe: vi.fn(),
  role: "member",
  assigned: [] as unknown[],
}));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: mocks.role }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  workspaceMcpServersOptions: () => ({
    queryKey: ["library"],
    queryFn: async () => [
      {
        id: "existing",
        name: "browser",
        transport: "stdio",
        template_key: "playwright",
      },
    ],
  }),
  agentMcpServersOptions: () => ({
    queryKey: ["assigned"],
    queryFn: async () => mocks.assigned,
  }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: async () => [] }),
}));
vi.mock("@multica/core/workspace/mutations", () => ({
  useAssignWorkspaceMcpServer: () => ({ mutateAsync: mocks.assign }),
  useCreateWorkspaceMcpServer: () => ({ mutateAsync: mocks.create }),
  useCreateWorkspaceMcpServerFromTemplate: () => ({
    reset: vi.fn(),
    mutateAsync: mocks.recipe,
  }),
}));
vi.mock("../settings/hooks/use-mcp-server-templates", () => ({
  useMcpServerTemplates: () => ({
    data: [
      {
        key: "playwright",
        title: "Playwright",
        description: "Browser tools",
        version: "1",
        config: { command: "npx" },
      },
    ],
    isPending: false,
  }),
}));
function Wrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <I18nProvider
        locale="en"
        resources={{ en: { settings: enSettings, agents: enAgents } }}
      >
        {children}
      </I18nProvider>
    </QueryClientProvider>
  );
}
function renderDiscovery(unsupported = false) {
  return render(
    <McpAgentDiscovery
      workspaceId="ws"
      context={{
        agent: { id: "agent", name: "Ada" },
        managedNames: new Set(["browser"]),
        runtimeNames: new Set(),
        unsupported,
      }}
      onClose={vi.fn()}
    />,
    { wrapper: Wrapper },
  );
}
describe("agent MCP discovery", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.role = "member";
    mocks.assigned = [];
    mocks.assign.mockResolvedValue({ succeeded: ["agent"], failed: [] });
  });
  it("offers existing instances first and lets an agent owner reuse one without creating", async () => {
    const user = userEvent.setup();
    renderDiscovery();
    expect(screen.getByRole("tab", { name: "Shared configurations" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    const reuse = await screen.findByRole("button", {
      name: "Use browser: Assign to Ada",
    });
    expect(screen.getByText(/already has a local configuration/)).toBeVisible();
    await user.click(reuse);
    await waitFor(() =>
      expect(mocks.assign).toHaveBeenCalledWith({
        serverId: "existing",
        agentIds: ["agent"],
      }),
    );
    expect(mocks.create).not.toHaveBeenCalled();
    expect(mocks.recipe).not.toHaveBeenCalled();
    await user.click(screen.getByRole("tab", { name: "MCP market" }));
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    expect(
      screen.queryByRole("button", { name: "Save and continue" }),
    ).toBeNull();
  });
  it("shows an existing disabled assignment without offering to re-enable it", async () => {
    mocks.assigned = [{ id: "existing", name: "browser", enabled: false }];
    renderDiscovery();
    expect(
      await screen.findByRole("button", {
        name: "Already assigned · disabled",
      }),
    ).toBeDisabled();
    expect(mocks.assign).not.toHaveBeenCalled();
  });
  it("shows and announces a slow assignment until its result is confirmed", async () => {
    const user = userEvent.setup();
    let resolveAssignment!: (value: {
      succeeded: string[];
      failed: { agentId: string; message: string }[];
    }) => void;
    mocks.assign.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveAssignment = resolve;
      }),
    );
    renderDiscovery();
    const reuse = await screen.findByRole("button", {
      name: "Use browser: Assign to Ada",
    });
    const liveRegions = screen.getAllByRole("status");
    await user.click(reuse);

    expect(screen.getByRole("button", { name: "Assigning…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Done" })).toBeDisabled();
    const status = screen.getByText("Assigning browser to Ada…");
    expect(status).toHaveAttribute("role", "status");
    expect(liveRegions).toContain(status);
    expect(mocks.assign).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveAssignment({ succeeded: ["agent"], failed: [] });
    });
    expect(status).toHaveTextContent("browser assigned to Ada.");
    expect(
      screen.getByRole("button", { name: "Already assigned" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Done" })).toBeEnabled();
  });
  it("blocks assignment for an explicitly unsupported runtime and names the reason", async () => {
    renderDiscovery(true);
    expect(
      await screen.findByRole("button", { name: "Use browser: Assign to Ada" }),
    ).toBeDisabled();
    expect(screen.getByText(/does not support MCP/)).toBeVisible();
  });
  it("keeps a failed existing-instance assignment available to retry", async () => {
    const user = userEvent.setup();
    mocks.assign.mockResolvedValueOnce({
      succeeded: [],
      failed: [{ agentId: "agent", message: "Try later" }],
    });
    renderDiscovery();
    await user.click(
      await screen.findByRole("button", { name: "Use browser: Assign to Ada" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("Try later");
    expect(screen.queryByText("Assigning browser to Ada…")).toBeNull();
    await user.click(
      screen.getByRole("button", { name: "Use browser: Assign to Ada" }),
    );
    expect(
      await screen.findByRole("button", { name: "Already assigned" }),
    ).toBeDisabled();
    expect(mocks.assign).toHaveBeenCalledTimes(2);
  });
});
