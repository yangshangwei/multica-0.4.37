import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { AutopilotsPage } from "./autopilots-page";

const listAutopilots = vi.hoisted(() => vi.fn());
const listAutopilotTemplates = vi.hoisted(() => vi.fn());
const push = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api: { listAutopilots, listAutopilotTemplates } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-test" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({
  newAutopilotTemplate: () => "/acme/autopilots/new/template",
  autopilotDetail: (id: string) => `/acme/autopilots/${id}`,
}) }));
vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Scout" }) }));
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push }), useRowLink: () => ({}) }));
vi.mock("./autopilot-dialog", () => ({ AutopilotDialog: () => <div role="dialog">Custom automation</div> }));
vi.mock("./autopilot-list-actions", () => ({ AutopilotBatchToolbar: () => null, AutopilotRowActions: () => null }));
vi.mock("./autopilot-list-toolbar", () => ({ AutopilotListToolbar: () => <div data-testid="instance-filters" />, actorFilterValue: (type: string, id: string) => `${type}:${id}` }));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(<QueryClientProvider client={client}><AutopilotsPage /></QueryClientProvider>);
}

beforeEach(() => {
  vi.clearAllMocks();
  listAutopilots.mockResolvedValue({ autopilots: [], total: 0 });
  listAutopilotTemplates.mockResolvedValue([{
    key: "daily-change-review", version: 1, category: "review", category_label: "Review",
    title: "Daily change review", description: "Review recent changes.", cron_expression: "0 18 * * *",
    execution_mode: "create_issue", avatar_emoji: "", prompt: "Review.",
  }]);
});

describe("automation catalog entry", () => {
  it("shows templates directly above the empty list and keeps custom creation", async () => {
    renderPage();
    const catalog = await screen.findByRole("region", { name: "Built-in templates" });
    await userEvent.setup().click(await within(catalog).findByRole("button", { name: /Daily change review/ }));
    expect(push).toHaveBeenCalledWith("/acme/autopilots/new/template?template=daily-change-review");
    expect(screen.getByText("No autopilots yet")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Start from scratch" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("Custom automation");
  });

  it("keeps templates above instance filters after automations exist", async () => {
    listAutopilots.mockResolvedValue({ autopilots: [{
      id: "ap-1", title: "Customized daily review", status: "active", execution_mode: "create_issue",
      assignee_type: "agent", assignee_id: "agent-1", created_at: "2026-09-12T00:00:00Z",
    }], total: 1 });
    renderPage();
    const catalog = await screen.findByRole("region", { name: "Built-in templates" });
    const filters = await screen.findByTestId("instance-filters");
    expect(catalog.compareDocumentPosition(filters) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
