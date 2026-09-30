// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Agent } from "@multica/core/types";
import type { AgentListFilters } from "@multica/core/agents/stores";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";
import { AgentDiscoveryToolbar } from "./agent-discovery-toolbar";
import { AgentListToolbar } from "./agent-list-toolbar";
import type { AgentListRow } from "./agents-page";

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("test-workspace") };
});
afterEach(cleanup);
const filters: AgentListFilters = { availability: [], runtimes: [], owners: [], models: [], access: [], categories: [], squads: [] };
const adapter: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/test-workspace/agents", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
function wrap(children: React.ReactNode) {
  return render(<I18nProvider locale="en" resources={{ en: { agents: enAgents, common: enCommon } }}><NavigationProvider value={adapter}>{children}</NavigationProvider></I18nProvider>);
}
const BASE_AGENT: Agent = {
  id: "agent-base",
  workspace_id: "workspace-1",
  runtime_id: "runtime-1",
  name: "Base Agent",
  description: "",
  instructions: "",
  avatar_url: null,
  runtime_mode: "cloud",
  runtime_config: {},
  custom_args: [],
  visibility: "workspace",
  permission_mode: "private",
  invocation_targets: [],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "claude",
  owner_id: "user-1",
  skills: [],
  created_at: "2026-06-01T00:00:00Z",
  updated_at: "2026-06-01T00:00:00Z",
  archived_at: null,
  archived_by: null,
};


describe("agent directory toolbar controls", () => {
  it("clears each selected category and squad using keyboard buttons", async () => {
    const toggle = vi.fn();
    wrap(<AgentDiscoveryToolbar rows={[]} squads={[]} filters={{ ...filters, categories: ["custom:Research"], squads: ["missing-squad"] }} onToggleFilter={toggle} />);
    const user = userEvent.setup();
    screen.getByRole("button", { name: "Remove category: Research" }).focus();
    await user.keyboard("{Enter}");
    expect(toggle).toHaveBeenCalledWith("categories", "custom:Research");
    screen.getByRole("button", { name: "Remove squad: missing-squad" }).focus();
    await user.keyboard(" ");
    expect(toggle).toHaveBeenCalledWith("squads", "missing-squad");
  });

  it("links the recommended assistant directly to its detail page", async () => {
    const row: AgentListRow = { agent: { ...BASE_AGENT, id: "mika-1", name: "Mika", system_key: "mika" }, category: { key: "preset:other", name: "Generalist", preset: "other" }, runtime: null, presence: null, activity: null, runCount: 0, lastActiveDays: null, owner: null, isOwnedByMe: true, canManage: true };
    wrap(<AgentDiscoveryToolbar rows={[row]} squads={[]} filters={filters} onToggleFilter={vi.fn()} />);
    const link = screen.getByRole("link", { name: "Mika" });
    expect(link).toHaveAttribute("href", "/test-workspace/agents/mika-1");
    await userEvent.click(link);
    expect(adapter.push).toHaveBeenCalledWith("/test-workspace/agents/mika-1");
  });

  it("counts only extra dimensions in More filters and exposes a separate keyboard clear-all", async () => {
    const clear = vi.fn();
    wrap(<AgentListToolbar scope="mine" onScopeChange={vi.fn()} scopeCounts={{ mine: 0, all: 0, archived: 0 }} search="" onSearchChange={vi.fn()} filters={{ ...filters, availability: ["online"], categories: ["preset:other"], squads: ["squad-1"] }} onToggleFilter={vi.fn()} onClearFilters={clear} sortField="name" sortDirection="asc" onSortFieldChange={vi.fn()} onSortDirectionChange={vi.fn()} hiddenColumns={[]} groupBy="none" onGroupByChange={vi.fn()} onToggleColumn={vi.fn()} allRows={[]} members={[]} visibleCount={0} />);
    const more = screen.getByRole("button", { name: "More filters" });
    expect(more).toHaveTextContent("1");
    expect(more.querySelector("[role=button], button")).toBeNull();
    const clearButton = screen.getByRole("button", { name: "Clear filters (3)" });
    clearButton.focus();
    await userEvent.keyboard("{Enter}");
    expect(clear).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});
