// @vitest-environment jsdom

import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import type { AgentRoleTemplate } from "@multica/core/types";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NavigationProvider } from "../navigation";
import { ScrollRestorationProvider, type ScrollRestorationAdapter } from "../platform/scroll-restoration";
import { renderWithI18n } from "../test/i18n";
import enAgents from "../locales/en/agents.json";
import { AgentTemplatesMarketPanel } from "./agent-templates-market-panel";

const mocks = vi.hoisted(() => ({ roles: vi.fn(), agents: vi.fn(), create: vi.fn(), push: vi.fn() }));
vi.mock("@multica/core/api", () => ({ api: {
  listAgentRoleTemplates: mocks.roles,
  listAgents: mocks.agents,
  createAgentFromTemplate: mocks.create,
} }));

const ROLE: AgentRoleTemplate = {
  key: "product-analyst", name: "Product Analyst", title: "Product Analyst", version: 1,
  description: "Clarifies requirements before work starts.", autonomy_level: "observer",
  avatar_emoji: "🔍", max_concurrent_tasks: 1, skill_names: [], instructions: "Analyze requirements.",
};

function setup({ active = true, adapter, locale = "en" }: {
  active?: boolean;
  adapter?: ScrollRestorationAdapter;
  locale?: "en" | "zh-Hans";
} = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  function Panel({ active }: { active: boolean }) {
    const content = <AgentTemplatesMarketPanel workspaceId="ws-1" active={active} />;
    return (
      <QueryClientProvider client={client}>
        <WorkspaceSlugProvider slug="acme">
          <NavigationProvider value={{
            pathname: "/acme/marketplace", searchParams: new URLSearchParams("type=agent-templates&squad=ignore"),
            hash: "", push: mocks.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => path,
          }}>
            {adapter ? <ScrollRestorationProvider adapter={adapter}>{content}</ScrollRestorationProvider> : content}
          </NavigationProvider>
        </WorkspaceSlugProvider>
      </QueryClientProvider>
    );
  }
  const view = renderWithI18n(<Panel active={active} />, { locale });
  return { ...view, client, setActive: (value: boolean) => view.rerender(<Panel active={value} />) };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.roles.mockResolvedValue([ROLE]);
  mocks.agents.mockRejectedValue(new Error("Agent inventory is unavailable"));
});

describe("AgentTemplatesMarketPanel", () => {
  it("browses templates independently of saved agents and links explicitly to configuration", async () => {
    mocks.roles.mockResolvedValue([ROLE, { ...ROLE, key: "future", title: "Future Role", autonomy_level: "future-policy" }]);
    setup();
    const card = await screen.findByRole("listitem", { name: "Product Analyst" });
    expect(within(card).getByText("Observer")).toBeVisible();
    expect(within(card).getByRole("link", { name: "Use template" })).toHaveAttribute(
      "href", "/acme/agents/new/template?template=product-analyst&source=marketplace",
    );
    expect(within(screen.getByRole("listitem", { name: "Future Role" })).getByRole("link", { name: "Use template" })).toBeVisible();
    expect(screen.queryByText("future-policy")).not.toBeInTheDocument();
    expect(screen.queryByText(/existing agent|No agents created/)).not.toBeInTheDocument();
    expect(mocks.agents).not.toHaveBeenCalled();
    expect(mocks.create).not.toHaveBeenCalled();
    await userEvent.setup().click(within(card).getByRole("link", { name: "Use template" }));
    expect(mocks.push).toHaveBeenCalledExactlyOnceWith("/acme/agents/new/template?template=product-analyst&source=marketplace");
    expect(mocks.create).not.toHaveBeenCalled();
  });

  it("gates its query observer while retaining cached cards across inactive visits", async () => {
    const view = setup({ active: false });
    expect(mocks.roles).not.toHaveBeenCalled();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    view.setActive(true);
    const card = await screen.findByRole("listitem", { name: "Product Analyst" });
    expect(mocks.roles).toHaveBeenCalledExactlyOnceWith("en");
    view.setActive(false);
    await act(async () => { await view.client.invalidateQueries({ queryKey: ["agent-role-templates", "en"] }); });
    expect(mocks.roles).toHaveBeenCalledOnce();
    expect(card).toBeInTheDocument();
    view.setActive(true);
    await waitFor(() => expect(mocks.roles).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("listitem", { name: "Product Analyst" })).toBe(card);
  });

  it("distinguishes loading, retryable failure and a successful empty catalog", async () => {
    let reject!: (error: Error) => void;
    mocks.roles.mockImplementationOnce(() => new Promise<AgentRoleTemplate[]>((_resolve, rejectPromise) => { reject = rejectPromise; }));
    setup();
    expect(screen.getByRole("status")).toBeVisible();
    expect(screen.queryByText(enAgents.role_templates.empty)).not.toBeInTheDocument();
    await act(async () => { reject(new Error("offline")); });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(enAgents.role_templates.load_failed);
    mocks.roles.mockResolvedValue([]);
    await userEvent.setup().click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(enAgents.role_templates.empty)).toBeVisible();
  });

  it("restores scrolling only after templates arrive and records an explicit zero", async () => {
    let resolve!: (templates: AgentRoleTemplate[]) => void;
    mocks.roles.mockImplementationOnce(() => new Promise<AgentRoleTemplate[]>((resolvePromise) => { resolve = resolvePromise; }));
    const state = new Map<string, string>([["agent-template-market-scroll", "240"]]);
    const adapter: ScrollRestorationAdapter = {
      get: () => ({ top: 900, height: 2000 }),
      getViewState: (key) => state.get(key),
      setViewState: (key, value) => { if (value !== undefined) state.set(key, value); },
    };
    setup({ adapter });
    const root = screen.getByRole("region", { name: "Agent templates" });
    fireEvent.scroll(root);
    expect(state.get("agent-template-market-scroll")).toBe("240");
    await act(async () => { resolve([ROLE]); });
    await screen.findByRole("listitem", { name: "Product Analyst" });
    expect(root.scrollTop).toBe(240);
    fireEvent.scroll(root, { target: { scrollTop: 0 } });
    expect(state.get("agent-template-market-scroll")).toBe("0");
  });

  it("uses Chinese market copy without borrowing the creation page title", async () => {
    setup({ locale: "zh-Hans" });
    expect(screen.getByRole("heading", { name: "智能体模板" })).toBeVisible();
    expect(await screen.findByRole("link", { name: "使用模板" })).toBeVisible();
    expect(screen.queryByRole("heading", { name: "从角色模板开始" })).not.toBeInTheDocument();
    expect(mocks.roles).toHaveBeenCalledWith("zh");
  });
});
