// @vitest-environment jsdom
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import type {
  McpServerTemplate,
  WorkspaceMcpServer,
} from "@multica/core/types";
import enSettings from "../locales/en/settings.json";
import enAgents from "../locales/en/agents.json";
import { ApiError } from "@multica/core/api";
import { McpLibraryCatalog } from "./mcp-market";
import { McpSetupDialog } from "./mcp-setup-dialog";

const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  resetCreate: vi.fn(),
  assign: vi.fn(),
  custom: vi.fn(),
  templates: [] as unknown[] | undefined,
  templatesPending: false,
  templatesError: false,
  refetch: vi.fn(),
  queryOptions: vi.fn(),
  agents: [] as unknown[],
}));
vi.mock("../settings/hooks/use-mcp-server-templates", () => ({
  useMcpServerTemplates: (_ws: string, options: unknown) => {
    mocks.queryOptions(options);
    return ({
    data: mocks.templates,
    isPending: mocks.templatesPending,
    isError: mocks.templatesError,
    refetch: mocks.refetch,
  }); },
}));
vi.mock("@multica/core/workspace/mutations", () => ({
  useCreateWorkspaceMcpServerFromTemplate: () => ({
    mutateAsync: mocks.create,
    reset: mocks.resetCreate,
    isPending: false,
  }),
  useAssignWorkspaceMcpServer: () => ({
    mutateAsync: mocks.assign,
    isPending: false,
  }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({
    queryKey: ["agents"],
    queryFn: async () => mocks.agents,
  }),
}));
const template: McpServerTemplate = {
  key: "playwright",
  title: "Playwright",
  description: "Browse the web",
  config: { command: "npx" },
  version: "1",
  category: "browser",
  requirements: ["Node.js and npx"],
};
const documentationTemplates: McpServerTemplate[] = [
  {
    key: "microsoft-learn",
    title: "Microsoft Learn",
    description: "Search official Microsoft documentation and code examples.",
    config: { type: "http", url: "https://learn.microsoft.com/api/mcp" },
    version: "1",
    category: "documentation",
    documentationUrl: "https://learn.microsoft.com/en-us/training/support/mcp",
    requirements: ["Streamable HTTP support and network access", "Public documentation only; no training or user profile information"],
  },
  {
    key: "deepwiki",
    title: "DeepWiki",
    description: "Explore documentation and ask questions about public GitHub repositories.",
    config: { type: "http", url: "https://mcp.deepwiki.com/mcp" },
    version: "1",
    category: "documentation",
    documentationUrl: "https://docs.devin.ai/work-with-devin/deepwiki-mcp",
    requirements: ["Streamable HTTP support and network access", "Indexed public repositories only; private repositories are not supported"],
  },
];
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
function renderCatalog(
  props: Partial<Parameters<typeof McpLibraryCatalog>[0]> = {},
) {
  return render(
    <McpLibraryCatalog
      workspaceId="ws"
      servers={[]}
      loaded
      canManage
      onCustom={mocks.custom}
      {...props}
    >
      <p>Workspace inventory</p>
    </McpLibraryCatalog>,
    { wrapper: Wrapper },
  );
}
describe("MCP market", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.templatesPending = false;
    mocks.templatesError = false;
    mocks.refetch.mockImplementation(async () => ({ data: mocks.templates, isError: mocks.templatesError }));
    mocks.agents = [
      { id: "a", name: "Ada" },
      { id: "b", name: "Ben" },
    ];
    mocks.templates = [
      template,
      {
        ...template,
        key: "thinking",
        title: "Sequential thinking",
        category: "reasoning",
      },
    ];
    mocks.create.mockResolvedValue({
      id: "saved",
      name: "playwright",
      transport: "stdio",
    });
    mocks.assign.mockResolvedValue({ succeeded: ["a", "b"], failed: [] });
  });
  it("filters sources independently of search/category, refreshes deployment immediately, and keeps same-key instances separate", async () => {
    const user = userEvent.setup();
    mocks.templates!.push({ ...template, source: "deployment", title: "Team browser", config: {}, transport: "http", version: "sha256:abc", category: "coding" });
    renderCatalog({ servers: [{ id: "deployed", workspace_id: "ws", name: "internal-browser", transport: "http", template_key: "playwright", template_source: "deployment", created_at: "", updated_at: "" }] });
    await user.click(screen.getByRole("tab", { name: "MCP market" }));
    expect(screen.getByRole("tab", { name: "MCP market" })).toHaveTextContent("MCP market3");
    const builtin = screen.getByRole("button", { name: "View configuration: Playwright" });
    expect(within(builtin).queryByText(/internal-browser/)).toBeNull();
    await user.click(screen.getByRole("button", { name: "Reasoning" }));
    await user.type(screen.getByRole("searchbox"), "browser");
    const before = mocks.refetch.mock.calls.length;
    await user.click(screen.getByRole("button", { name: "Deployment provided" }));
    expect(mocks.refetch).toHaveBeenCalledTimes(before + 1);
    expect(screen.getByRole("searchbox")).toHaveValue("browser");
    expect(screen.queryByRole("button", { name: "Reasoning" })).toBeNull();
    expect(screen.getByRole("button", { name: "View configuration: Team browser" })).toHaveTextContent("internal-browser");
    expect(screen.getByRole("button", { name: "Deployment provided" })).toHaveTextContent("1");
    expect(screen.getByRole("button", { name: "Platform built-in" })).toHaveTextContent("2");
  });
  it("polls only while the market is selected and refreshes on re-entry", async () => {
    const user = userEvent.setup();
    renderCatalog();
    expect(mocks.queryOptions).toHaveBeenLastCalledWith({ poll: true });
    await user.click(screen.getByRole("tab", { name: "Shared configurations" }));
    expect(mocks.queryOptions).toHaveBeenLastCalledWith({ poll: false });
    const before = mocks.refetch.mock.calls.length;
    await user.click(screen.getByRole("tab", { name: "MCP market" }));
    expect(mocks.queryOptions).toHaveBeenLastCalledWith({ poll: true });
    expect(mocks.refetch).toHaveBeenCalledTimes(before + 1);
  });
  it("explains an empty deployment source without exposing server setup details", async () => {
    const user = userEvent.setup();
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "Deployment provided" }));
    expect(screen.getByText("No deployment-provided MCP templates yet. Contact your deployment administrator to add one.")).toBeVisible();
    expect(screen.queryByText(/MULTICA_MCP/)).toBeNull();
  });
  it("saves a config-free deployment snapshot then retains assignment when the catalog removes it", async () => {
    const user = userEvent.setup();
    const deployment = { ...template, source: "deployment", title: "Team browser", config: {}, transport: "http", version: "sha256:abc" };
    mocks.templates = [deployment];
    const view = renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Team browser" }));
    expect(screen.queryByText(/sha256/)).toBeNull();
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).toHaveBeenCalledWith({ name: "playwright", templateSource: "deployment", templateKey: "playwright", templateVersion: "sha256:abc" });
    await screen.findByRole("heading", { name: "Choose agents" });
    mocks.templates = [];
    view.rerender(<McpLibraryCatalog workspaceId="ws" servers={[]} loaded canManage onCustom={mocks.custom}><p>Workspace inventory</p></McpLibraryCatalog>);
    expect(screen.getByRole("heading", { name: "Choose agents" })).toBeVisible();
    expect(mocks.custom).not.toHaveBeenCalled();
    expect(mocks.assign).not.toHaveBeenCalled();
  });
  it("blocks changed snapshots, preserves drafts on refresh failure, and clears inputs only when reloading", async () => {
    const user = userEvent.setup();
    const deployment = { ...template, source: "deployment", config: {}, transport: "http", version: "sha256:old", inputs: [{ key: "token", label: "Access token", description: "", required: true, secret: true }] };
    mocks.templates = [deployment];
    const view = renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.type(screen.getByLabelText("Access token", { exact: false }), "old-secret");
    const rerender = () => view.rerender(<McpLibraryCatalog workspaceId="ws" servers={[]} loaded canManage onCustom={mocks.custom}><p>Workspace inventory</p></McpLibraryCatalog>);
    mocks.templatesError = true;
    rerender();
    expect(screen.getByLabelText("Access token", { exact: false })).toHaveValue("old-secret");
    expect(screen.getByRole("button", { name: "Save and continue" })).toBeEnabled();
    mocks.templatesError = false;
    mocks.templates = [{ ...deployment, version: "sha256:new" }];
    rerender();
    expect(screen.getByRole("button", { name: "Save and continue" })).toBeDisabled();
    expect(screen.getByLabelText("Access token", { exact: false })).toHaveValue("old-secret");
    await user.click(screen.getByRole("button", { name: "Reload template" }));
    expect(screen.getByLabelText("Access token", { exact: false })).toHaveValue("");
    await user.type(screen.getByLabelText("Access token", { exact: false }), "new-secret");
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ templateVersion: "sha256:new", templateInputs: { token: "new-secret" } }));
  });
  it.each(["mcp_template_changed", "mcp_template_unavailable", "mcp_catalog_unavailable"])("recovers from %s without leaking API details", async (code) => {
    const user = userEvent.setup();
    mocks.create.mockRejectedValueOnce(new ApiError("sensitive-path", code === "mcp_catalog_unavailable" ? 503 : 409, "", { code }));
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(screen.queryByText("sensitive-path")).toBeNull();
    expect(screen.getByRole("textbox", { name: "Configuration name" })).toHaveValue("playwright");
    if (code === "mcp_catalog_unavailable") {
      expect(screen.getByRole("button", { name: "Save and continue" })).toBeEnabled();
      expect(screen.getByRole("button", { name: "Refresh catalog" })).toBeEnabled();
    } else {
      expect(screen.getByRole("button", { name: "Save and continue" })).toBeDisabled();
      expect(mocks.refetch).toHaveBeenCalled();
    }
  });
  it("keeps inventory counts separate from filtered template results", async () => {
    const user = userEvent.setup();
    mocks.templates!.push({ ...template, key: "unusable", config: {} });
    renderCatalog();
    expect(screen.getByRole("tab", { name: "Shared configurations" })).toHaveTextContent("Shared configurations0");
    expect(screen.getByRole("tab", { name: "MCP market" })).toHaveTextContent("MCP market2");
    await user.click(screen.getByRole("button", { name: "Reasoning" }));
    expect(screen.getByRole("status")).toHaveTextContent("Templates found: 1");
    expect(screen.getByRole("tab", { name: "MCP market" })).toHaveTextContent("MCP market2");
  });
  it("treats a required constructor input as an empty draft until the user enters its value", async () => {
    const user = userEvent.setup();
    mocks.templates = [{
      ...template,
      source: "deployment",
      transport: "http",
      config: {},
      version: "sha256:constructor-input",
      inputs: [{ key: "constructor", label: "Account token", description: "", required: true, secret: true }],
    }];
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    const input = screen.getByLabelText("Account token", { exact: false });
    expect(input).toHaveValue("");
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("Enter Account token.");
    expect(input).toHaveFocus();
    await user.type(input, "account-secret");
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).toHaveBeenCalledWith({
      name: "playwright",
      templateSource: "deployment",
      templateKey: "playwright",
      templateVersion: "sha256:constructor-input",
      templateInputs: { constructor: "account-secret" },
    });
    await screen.findByRole("heading", { name: "Choose agents" });
    expect(screen.queryByLabelText("Account token", { exact: false })).toBeNull();
  });
  it("requires and masks database input, preserves it on failure, then clears the form on save", async () => {
    const user = userEvent.setup();
    const database = {
      ...template, key: "dbhub", title: "DBHub", category: "database",
      inputs: [{ key: "database_url", label: "Database connection URL", description: "Use a database reachable from the agent runtime.", required: true, secret: true }],
    };
    mocks.templates = [database, { ...template, key: "serena", title: "Serena", category: "coding" }];
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "Coding" }));
    expect(screen.getByRole("button", { name: "View configuration: Serena" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Databases" }));
    expect(screen.queryByRole("button", { name: "View configuration: Serena" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "View configuration: DBHub" }));
    const input = screen.getByLabelText("Database connection URL", { exact: false });
    expect(input).toHaveAttribute("type", "password");
    expect(input).toHaveAttribute("aria-required", "true");
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).not.toHaveBeenCalled();
    expect(input).toHaveFocus();
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toHaveTextContent("Enter Database connection URL.");
    const databaseUrl = "postgresql://user:secret@localhost/app";
    await user.type(input, databaseUrl);
    mocks.create.mockRejectedValueOnce(new Error("Temporary server failure"));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Temporary server failure"));
    expect(input).toHaveValue(databaseUrl);
    expect(mocks.resetCreate).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).toHaveBeenLastCalledWith({
      name: "dbhub", templateSource: "builtin", templateKey: "dbhub", templateVersion: "1", templateInputs: { database_url: databaseUrl },
    });
    await screen.findByRole("heading", { name: "Choose agents" });
    expect(screen.queryByLabelText("Database connection URL", { exact: false })).toBeNull();
    expect(mocks.resetCreate).toHaveBeenCalledTimes(2);
    expect(mocks.assign).not.toHaveBeenCalled();
  });
  it("reuses an existing database configuration without asking for its secret again", async () => {
    const user = userEvent.setup();
    render(<McpSetupDialog workspaceId="ws" template={{
      ...template, key: "postgres-mcp", title: "Postgres MCP Pro",
      inputs: [{ key: "database_url", label: "Database connection URL", description: "", required: true, secret: true }],
    }} available servers={[{
      id: "existing", workspace_id: "ws", name: "dev-db", transport: "stdio", template_key: "postgres-mcp", template_version: "1", created_at: "", updated_at: "",
    }]} canManage onCustom={mocks.custom} onClose={vi.fn()} />, { wrapper: Wrapper });
    await user.click(screen.getByRole("button", { name: "Use dev-db" }));
    await screen.findByRole("heading", { name: "Choose agents" });
    expect(mocks.create).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("Database connection URL", { exact: false })).toBeNull();
  });
  it("does not present unknown inventories as zero counts", () => {
    mocks.templates = undefined;
    mocks.templatesPending = true;
    renderCatalog({ servers: undefined, loaded: false });
    expect(screen.getByRole("tab", { name: "Shared configurations" })).toHaveTextContent(/^Shared configurations$/);
    expect(screen.getByRole("tab", { name: "MCP market" })).toHaveTextContent(/^MCP market$/);
  });
  it.each(documentationTemplates)("discovers $title by documentation category and search, then explicitly assigns its saved HTTP recipe", async (recipe) => {
    const user = userEvent.setup();
    mocks.templates!.push(...documentationTemplates);
    mocks.create.mockResolvedValue({
      id: "saved-http",
      name: recipe.key,
      transport: "http",
      template_key: recipe.key,
      template_version: recipe.version,
    });
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "Documentation & knowledge" }));
    expect(screen.getByRole("status")).toHaveTextContent("Templates found: 2");
    expect(screen.queryByRole("button", { name: "View configuration: Playwright" })).toBeNull();
    await user.type(screen.getByRole("searchbox", { name: "Search MCP templates" }), recipe.key);
    expect(screen.getByRole("status")).toHaveTextContent("Templates found: 1");
    expect(screen.getByRole("tab", { name: "MCP market" })).toHaveTextContent("MCP market4");
    await user.click(screen.getByRole("button", { name: `View configuration: ${recipe.title}` }));
    for (const requirement of recipe.requirements ?? []) {
      expect(screen.getByText(requirement, { exact: true })).toBeVisible();
    }
    expect(screen.getByRole("link", { name: "Documentation" })).toHaveAttribute("href", recipe.documentationUrl);
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).toHaveBeenCalledWith({
      name: recipe.key,
      templateSource: "builtin",
      templateKey: recipe.key,
      templateVersion: "1",
    });
    const ada = await screen.findByRole("checkbox", { name: "Ada" });
    expect(ada).not.toBeChecked();
    expect(mocks.assign).not.toHaveBeenCalled();
    await user.click(ada);
    await user.click(screen.getByRole("button", { name: "Assign selected" }));
    expect(mocks.assign).toHaveBeenCalledWith({ serverId: "saved-http", agentIds: ["a"] });
    expect(mocks.create).toHaveBeenCalledTimes(1);
  });
  it("hides category filters the loaded catalog does not use", () => {
    renderCatalog();
    expect(screen.getByRole("button", { name: "Browser" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Reasoning" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Documentation & knowledge" })).toBeNull();
  });
  it("defaults a loaded empty workspace to market and lets members browse without creation", async () => {
    const user = userEvent.setup();
    renderCatalog({ canManage: false });
    expect(screen.getByRole("tab", { name: "MCP market" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    expect(
      screen.getByRole("heading", { name: "Set up Playwright" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Save and continue" }),
    ).toBeNull();
  });
  it("keeps an explicit workspace choice while the library finishes loading", async () => {
    const user = userEvent.setup();
    const view = renderCatalog({ loaded: false });
    await user.click(screen.getByRole("tab", { name: "Shared configurations" }));
    view.rerender(
      <McpLibraryCatalog
        workspaceId="ws"
        servers={[]}
        loaded
        canManage
        onCustom={mocks.custom}
      >
        <p>Workspace inventory</p>
      </McpLibraryCatalog>,
    );
    expect(screen.getByRole("tab", { name: "Shared configurations" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });
  it("filters by category and search without disabling renamed template instances", async () => {
    const user = userEvent.setup();
    renderCatalog({
      servers: [
        {
          id: "existing",
          workspace_id: "ws",
          name: "qa-browser",
          transport: "stdio",
          created_at: "",
          updated_at: "",
          template_key: "playwright",
        } satisfies WorkspaceMcpServer,
      ],
    });
    await user.click(screen.getByRole("tab", { name: "MCP market" }));
    expect(screen.getByText(/qa-browser/)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Reasoning" }));
    expect(
      screen.queryByRole("button", { name: "View configuration: Playwright" }),
    ).toBeNull();
    await user.type(
      screen.getByRole("searchbox", { name: "Search MCP templates" }),
      "missing",
    );
    expect(screen.getByText("No matching templates")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(
      screen.getByRole("button", { name: "View configuration: Playwright" }),
    ).toBeEnabled();
  });
  it("announces result counts as filters change without moving search focus", async () => {
    const user = userEvent.setup();
    renderCatalog();
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Templates found: 2");
    await user.click(screen.getByRole("button", { name: "Reasoning" }));
    expect(status).toHaveTextContent("Templates found: 1");
    const search = screen.getByRole("searchbox", { name: "Search MCP templates" });
    await user.type(search, "missing");
    expect(status).toHaveTextContent("Templates found: 0");
    expect(search).toHaveFocus();
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(status).toHaveTextContent("Templates found: 2");
  });
  it("keeps the saved ID after partial failure and retries only failed assignments", async () => {
    const user = userEvent.setup();
    mocks.assign
      .mockResolvedValueOnce({
        succeeded: ["a"],
        failed: [{ agentId: "b", message: "Permission changed" }],
      })
      .mockResolvedValueOnce({ succeeded: ["b"], failed: [] });
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(mocks.create).toHaveBeenCalledWith({
      name: "playwright",
      templateSource: "builtin",
      templateKey: "playwright",
      templateVersion: "1",
    });
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Choose agents" }),
      ).toBeVisible(),
    );
    expect(screen.getByRole("heading", { name: "Choose agents" })).toHaveFocus();
    const ada = await screen.findByRole("checkbox", { name: "Ada" });
    expect(ada).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Ben" })).not.toBeChecked();
    expect(mocks.assign).not.toHaveBeenCalled();
    await user.click(ada);
    await user.click(screen.getByRole("checkbox", { name: "Ben" }));
    await user.click(screen.getByRole("button", { name: "Assign selected" }));
    expect(await screen.findByText("Permission changed")).toBeVisible();
    expect(screen.getByRole("checkbox", { name: "Ada" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    await user.click(screen.getByRole("button", { name: "Retry failed" }));
    await waitFor(() =>
      expect(mocks.assign).toHaveBeenLastCalledWith({
        serverId: "saved",
        agentIds: ["b"],
      }),
    );
    expect(mocks.create).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Done" })).toBeEnabled();
  });
  it("retains the assignment step when the new saved configuration updates the library", async () => {
    const user = userEvent.setup();
    const view = renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    await screen.findByRole("heading", { name: "Choose agents" });
    view.rerender(
      <McpLibraryCatalog
        workspaceId="ws"
        servers={[
          {
            id: "saved",
            workspace_id: "ws",
            name: "playwright",
            transport: "stdio",
            created_at: "",
            updated_at: "",
            template_key: "playwright",
          },
        ]}
        loaded
        canManage
        onCustom={mocks.custom}
      >
        <p>Workspace inventory</p>
      </McpLibraryCatalog>,
    );
    expect(
      screen.getByRole("heading", { name: "Choose agents" }),
    ).toBeVisible();
    expect(mocks.create).toHaveBeenCalledTimes(1);
  });
  it("keeps a failed creation editable without granting access", async () => {
    const user = userEvent.setup();
    mocks.create.mockRejectedValueOnce(new Error("Server unavailable"));
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Server unavailable",
    );
    expect(
      screen.getByRole("textbox", { name: "Configuration name" }),
    ).toHaveValue("playwright");
    expect(mocks.assign).not.toHaveBeenCalled();
  });
  it("explains the required name format before submission and keeps it with errors", async () => {
    const user = userEvent.setup();
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    const name = screen.getByRole("textbox", { name: "Configuration name" });
    expect(name).toBeRequired();
    expect(name).toHaveAccessibleDescription(
      "Required. Use English letters, numbers, hyphens, and underscores.",
    );
    await user.clear(name);
    await user.type(name, "浏览器");
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(name).toHaveFocus();
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription(
      /Required\. Use English letters, numbers, hyphens, and underscores\..*Use only/,
    );
    expect(mocks.create).not.toHaveBeenCalled();
  });
  it("resumes assignment for an existing custom configuration without creating again", async () => {
    const user = userEvent.setup();
    render(
      <McpSetupDialog
        workspaceId="ws"
        initialServer={{
          id: "custom",
          workspace_id: "ws",
          name: "my-tool",
          transport: "http",
          created_at: "",
          updated_at: "",
        }}
        servers={[]}
        canManage
        onClose={vi.fn()}
      />,
      { wrapper: Wrapper },
    );
    await user.click(await screen.findByRole("checkbox", { name: "Ada" }));
    await user.click(screen.getByRole("button", { name: "Assign selected" }));
    expect(mocks.assign).toHaveBeenCalledWith({
      serverId: "custom",
      agentIds: ["a"],
    });
    expect(mocks.create).not.toHaveBeenCalled();
  });
  it("does not switch to an existing configuration while creation is pending", async () => {
    const user = userEvent.setup();
    let finishCreate!: (value: { id: string; name: string }) => void;
    mocks.create.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishCreate = resolve;
        }),
    );
    renderCatalog({
      servers: [
        {
          id: "existing",
          workspace_id: "ws",
          name: "qa-browser",
          transport: "stdio",
          created_at: "",
          updated_at: "",
          template_key: "playwright",
        },
      ],
    });
    await user.click(screen.getByRole("tab", { name: "MCP market" }));
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(
      screen.getByRole("button", { name: "Use qa-browser" }),
    ).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Use qa-browser" }));
    expect(screen.queryByRole("heading", { name: "Choose agents" })).toBeNull();
    finishCreate({ id: "new-server", name: "playwright" });
    await user.click(await screen.findByRole("checkbox", { name: "Ada" }));
    await user.click(screen.getByRole("button", { name: "Assign selected" }));
    expect(mocks.assign).toHaveBeenCalledWith({
      serverId: "new-server",
      agentIds: ["a"],
    });
  });
  it("only offers active user-created agents when inventory contains invalid or system entries", async () => {
    const user = userEvent.setup();
    mocks.agents = [
      null,
      { id: "system", name: "Mika", system_key: "mika" },
      { id: "archived", name: "Archived", archived_at: "2026-01-01" },
      { id: "missing-name" },
      { id: "a", name: "Ada" },
    ];
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    expect(await screen.findByRole("checkbox", { name: "Ada" })).toBeVisible();
    expect(screen.getAllByRole("checkbox")).toHaveLength(1);
  });
  it("can save and skip without assigning any agent", async () => {
    const user = userEvent.setup();
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(screen.getByRole("button", { name: "Save and continue" }));
    await user.click(screen.getByRole("button", { name: "Skip for now" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(mocks.assign).not.toHaveBeenCalled();
  });
  it("keeps old catalog entries on the ordinary custom path", async () => {
    const user = userEvent.setup();
    mocks.templates = [{ ...template, version: undefined }];
    renderCatalog();
    await user.click(screen.getByRole("button", { name: "View configuration: Playwright" }));
    await user.click(
      screen.getByRole("button", { name: "Open custom configuration" }),
    );
    expect(mocks.custom).toHaveBeenCalledWith({
      name: "playwright",
      config: { command: "npx" },
    });
    expect(mocks.create).not.toHaveBeenCalled();
  });
});
