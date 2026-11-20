import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import enAutopilots from "../../locales/en/autopilots.json";
import { AutopilotsPage } from "./autopilots-page";
import { useAutopilotsViewStore } from "@multica/core/autopilots/stores";
import { NavigationProvider } from "../../navigation";

const listAutopilots = vi.hoisted(() => vi.fn());
const listAutopilotTemplates = vi.hoisted(() => vi.fn());
const push = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api: { listAutopilots, listAutopilotTemplates } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-test" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({
  newAutopilotTemplate: () => "/acme/autopilots/new/template",
  autopilotDetail: (id: string) => `/acme/autopilots/${id}`,
  docsPage: (slug: string) => `/acme/docs/${slug}`,
}) }));
vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Scout" }) }));
vi.mock("../../navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../navigation")>()),
  useRowLink: () => ({}),
}));
vi.mock("./autopilot-dialog", () => ({ AutopilotDialog: () => <div role="dialog">Custom automation</div> }));
vi.mock("./autopilot-list-actions", () => ({ AutopilotBatchToolbar: () => null, AutopilotRowActions: () => null }));
vi.mock("./autopilot-list-toolbar", () => ({ AutopilotListToolbar: () => <div data-testid="instance-filters" />, actorFilterValue: (type: string, id: string) => `${type}:${id}` }));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(
    <NavigationProvider value={{
      push, replace: vi.fn(), back: vi.fn(), pathname: "/acme/autopilots",
      searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
    }}>
      <QueryClientProvider client={client}><AutopilotsPage /></QueryClientProvider>
    </NavigationProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useAutopilotsViewStore.setState(useAutopilotsViewStore.getInitialState());
  listAutopilots.mockResolvedValue({ autopilots: [], total: 0 });
  listAutopilotTemplates.mockResolvedValue([{
    key: "daily-change-review", version: 1, category: "review", category_label: "Review",
    title: "Daily change review", description: "Review recent changes.", cron_expression: "0 18 * * *",
    execution_mode: "create_issue", avatar_emoji: "", prompt: "Review.",
  }]);
});

describe("automation catalog entry", () => {
  it("welcomes an empty workspace with real templates and blank creation", async () => {
    renderPage();
    expect(await screen.findByRole("heading", { name: "Let your agents get to work on schedule" })).toBeInTheDocument();
    await userEvent.setup().click(await screen.findByRole("button", { name: /Daily change review/ }));
    expect(push).toHaveBeenCalledWith("/acme/autopilots/new/template?template=daily-change-review&return_to=autopilots");
    expect(screen.queryByText("No autopilots yet")).not.toBeInTheDocument();
    expect(screen.queryByTestId("instance-filters")).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: enAutopilots.page.new_autopilot })).toHaveLength(1);
    await userEvent.setup().click(screen.getByRole("button", { name: enAutopilots.page.new_autopilot }));
    expect(screen.getByRole("dialog")).toHaveTextContent("Custom automation");
  });

  it("opens the complete template catalog and in-app automation documentation", async () => {
    renderPage();
    const docs = await screen.findByRole("link", { name: "Read documentation" });
    expect(docs).toHaveAttribute("href", "/acme/docs/autopilots");
    await userEvent.setup().click(docs);
    expect(push).toHaveBeenLastCalledWith("/acme/docs/autopilots");
    const browse = screen.getByRole("link", { name: "Browse all templates" });
    expect(browse).toHaveAttribute("href", "/acme/autopilots/new/template");
    await userEvent.setup().click(browse);
    expect(push).toHaveBeenLastCalledWith("/acme/autopilots/new/template");
  });

  it("keeps creation and documentation usable while templates load", async () => {
    listAutopilotTemplates.mockReturnValue(new Promise(() => {}));
    renderPage();
    expect(await screen.findByRole("heading", { name: "Let your agents get to work on schedule" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Loading");
    expect(screen.getByRole("link", { name: "Read documentation" })).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: enAutopilots.page.new_autopilot }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("recovers a failed template load without hiding the welcome actions", async () => {
    listAutopilotTemplates.mockRejectedValueOnce(new Error("Template service unavailable"));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent(enAutopilots.template_picker.load_failed);
    expect(screen.getByRole("button", { name: enAutopilots.page.new_autopilot })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Read documentation" })).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: /Daily change review/ })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("offers blank creation when the server has no templates", async () => {
    listAutopilotTemplates.mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText(enAutopilots.template_picker.empty)).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: enAutopilots.page.new_autopilot }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("prioritizes existing automations and opens templates from the new action", async () => {
    listAutopilots.mockResolvedValue({ autopilots: [{
      id: "ap-1", title: "Customized daily review", status: "active", execution_mode: "create_issue",
      assignee_type: "agent", assignee_id: "agent-1", created_at: "2026-09-12T00:00:00Z",
    }], total: 1 });
    renderPage();
    expect(await screen.findByTestId("instance-filters")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Built-in templates" })).not.toBeInTheDocument();
    expect(listAutopilotTemplates).not.toHaveBeenCalled();
    await userEvent.setup().click(screen.getByRole("button", { name: enAutopilots.page.new_autopilot }));
    expect(push).toHaveBeenCalledWith("/acme/autopilots/new/template");
  });

  it("keeps the management view when its status filter has no matches", async () => {
    useAutopilotsViewStore.setState({ scope: "active" });
    listAutopilots.mockResolvedValue({ autopilots: [{
      id: "ap-1", title: "Paused review", status: "paused", execution_mode: "create_issue",
      assignee_type: "agent", assignee_id: "agent-1", created_at: "2026-09-12T00:00:00Z",
    }], total: 1 });
    renderPage();
    expect(await screen.findByTestId("instance-filters")).toBeInTheDocument();
    expect(listAutopilotTemplates).not.toHaveBeenCalled();
  });

  it("does not offer first-use templates before the automation list loads", () => {
    listAutopilots.mockReturnValue(new Promise(() => {}));
    renderPage();
    expect(listAutopilotTemplates).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Start from scratch" })).not.toBeInTheDocument();
  });

  it("reports list errors without treating them as an empty workspace", async () => {
    listAutopilots.mockRejectedValue(new Error("Unable to load automations"));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent("Unable to load automations");
    expect(listAutopilotTemplates).not.toHaveBeenCalled();
  });
});
