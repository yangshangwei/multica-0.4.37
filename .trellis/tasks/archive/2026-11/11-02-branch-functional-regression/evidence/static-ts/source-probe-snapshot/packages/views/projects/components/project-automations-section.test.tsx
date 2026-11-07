import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { ProjectAutomationsSection } from "./project-automations-section";

const listAutopilots = vi.hoisted(() => vi.fn());
const push = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api: { listAutopilots } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-test" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({
  newAutopilotTemplate: () => "/acme/autopilots/new/template",
  autopilotDetail: (id: string) => `/acme/autopilots/${id}`,
}) }));
vi.mock("../../navigation/context", () => ({ useNavigation: () => ({ push }) }));

function renderSection(defaultSquadId: string | null = "squad-1") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(<QueryClientProvider client={client}>
    <ProjectAutomationsSection projectId="project-1" defaultSquadId={defaultSquadId} />
  </QueryClientProvider>);
}

beforeEach(() => {
  vi.clearAllMocks();
  listAutopilots.mockResolvedValue({ autopilots: [], total: 0 });
});

describe("project automations", () => {
  it("opens templates with the project and default squad without writing", async () => {
    renderSection();
    expect(await screen.findByText("No automations for this project yet.")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Add automation" });
    expect(link).toHaveAttribute("href", "/acme/autopilots/new/template?project_id=project-1&assignee_type=squad&assignee_id=squad-1");
    await userEvent.setup().click(link);
    expect(push).toHaveBeenCalledWith(link.getAttribute("href"));
  });

  it("lists only current-workspace project automations with status and next run", async () => {
    const automation = { id: "ap-1", workspace_id: "ws-test", project_id: "project-1", title: "Daily check", status: "active", next_run_at: "2026-09-13T09:00:00Z" };
    listAutopilots.mockResolvedValue({ autopilots: [
      automation,
      { ...automation, id: "ap-2", title: "Paused review", status: "paused", next_run_at: null },
      { ...automation, id: "ap-3", title: "Other project", project_id: "project-2" },
      { ...automation, id: "ap-4", title: "Archived", status: "archived" },
      { ...automation, id: "ap-5", title: "Foreign workspace", workspace_id: "other-workspace" },
    ], total: 5 });
    renderSection();
    expect(await screen.findByRole("link", { name: /Daily check/ })).toHaveAttribute("href", "/acme/autopilots/ap-1");
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText(/Next run:/)).toBeInTheDocument();
    expect(screen.getByText("Paused")).toBeInTheDocument();
    expect(screen.queryByText("Other project")).not.toBeInTheDocument();
    expect(screen.queryByText("Archived")).not.toBeInTheDocument();
    expect(screen.queryByText("Foreign workspace")).not.toBeInTheDocument();
  });

  it("keeps creation reachable after a list failure and supports retry", async () => {
    listAutopilots.mockRejectedValueOnce(new Error("offline"));
    renderSection(null);
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load project automations.");
    expect(screen.getByRole("link", { name: "Add automation" })).toHaveAttribute("href", "/acme/autopilots/new/template?project_id=project-1");
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(await screen.findByText("No automations for this project yet.")).toBeInTheDocument();
  });
});
