import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createStore } from "zustand/vanilla";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import { viewStoreSlice, type IssueViewState } from "@multica/core/issues/stores/view-store";
import { useIssuesScopeStore } from "@multica/core/issues/stores/issues-scope-store";
import { renderWithI18n } from "../../test/i18n";
import { IssuesHeader } from "./issues-header";
import { MyIssuesHeader } from "../../my-issues/components/my-issues-header";

const { savedView, setActive } = vi.hoisted(() => ({
  savedView: { id: "saved-1", name: "Release tasks", owner_id: "user-1", scope_variant: "any", query: {} },
  setActive: vi.fn(),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/issue-views/use-active-view", () => ({
  useActiveIssueView: () => ({ activeView: savedView, views: [savedView, { ...savedView, id: "saved-2", name: "Follow-up" }], viewsReady: true, setActive, missing: false }),
}));
vi.mock("@multica/core/auth", () => ({ useAuthStore: (select: (state: unknown) => unknown) => select({ user: { id: "user-1" } }) }));
vi.mock("./view-bar", () => ({ ViewBar: () => null }));
vi.mock("./save-view-dialog", () => ({ SaveViewDialog: () => null }));
vi.mock("./filter-chips-bar", () => ({ FilterChipsBar: () => null }));
vi.mock("./workspace-agent-working-chip", () => ({ WorkspaceAgentWorkingChip: () => null }));
vi.mock("./table-view", () => ({ TableColumnPicker: () => null }));

function renderHeader(element: React.ReactNode) {
  const store = createStore<IssueViewState>()(viewStoreSlice);
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(<QueryClientProvider client={qc}><ViewStoreProvider store={store}>{element}</ViewStoreProvider></QueryClientProvider>);
}

afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe("Compact task view selection", () => {
  it("shows the saved view and exits it when choosing the already-current builtin scope", async () => {
    useIssuesScopeStore.getState().setScope("issues", "all");
    renderHeader(<IssuesHeader scopedIssues={[]} workingAgents={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "Release tasks" }));
    fireEvent.click(await screen.findByRole("menuitemradio", { name: "All" }));
    expect(setActive).toHaveBeenCalledWith(null);
    expect(useIssuesScopeStore.getState().scopes.issues).toBe("all");
  });

  it("keeps saved views reachable with keyboard navigation", async () => {
    const user = userEvent.setup();
    renderHeader(<IssuesHeader scopedIssues={[]} workingAgents={[]} />);
    screen.getByRole("button", { name: "Release tasks" }).focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(screen.getByRole("menuitemradio", { name: "All" })).toHaveFocus());
    await user.keyboard("{End}");
    expect(screen.getByRole("menuitemradio", { name: "Follow-up" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(setActive).toHaveBeenCalledWith("saved-2");
  });

  it("uses the same saved-view exit path in My Issues", async () => {
    const change = vi.fn();
    renderHeader(<MyIssuesHeader allIssues={[]} workingAgents={[]} scope="all" onScopeChange={change} onTableFacetChange={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "Release tasks" }));
    const builtin = await screen.findAllByRole("menuitemradio");
    fireEvent.click(builtin[0]!);
    expect(setActive).toHaveBeenCalledWith(null);
    expect(change).toHaveBeenCalledWith("all");
  });
});
