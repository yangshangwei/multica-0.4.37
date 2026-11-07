import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createStore } from "zustand/vanilla";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import {
  type IssueViewState,
  viewStoreSlice,
} from "@multica/core/issues/stores/view-store";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import { renderWithI18n } from "../../test/i18n";
import { IssueFilterMenu } from "./issues-header";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));

function renderFilterMenu() {
  const names = ["alpha beta", "alpha gamma", "中文"];
  setApiInstance({
    listIssueStatuses: async () => ({ statuses: [], categories: [], total: 0 }),
    listProperties: async () => ({ properties: [] }),
    listMembers: async () => names.map((name) => ({ user_id: name, name })),
    listAgents: async () => [],
    listSquads: async () => [],
    listProjects: async () => ({
      projects: names.map((title) => ({ id: title, title, icon: null })),
    }),
    listLabels: async () => ({
      labels: names.map((name) => ({ id: name, name, color: "#3b82f6" })),
    }),
  } as unknown as ApiClient);
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  const store = createStore<IssueViewState>()(viewStoreSlice);
  renderWithI18n(
    <QueryClientProvider client={queryClient}>
      <ViewStoreProvider store={store}>
        <IssueFilterMenu trigger={<button type="button">Filter</button>} />
        <button type="button">After filters</button>
      </ViewStoreProvider>
    </QueryClientProvider>,
  );
  return store;
}

async function openSearch(section: string) {
  await userEvent.click(screen.getByRole("button", { name: "Filter" }));
  await userEvent.click(await screen.findByRole("menuitem", { name: section }));
  await screen.findByRole("menuitemcheckbox", { name: "alpha beta" });
  const input = screen.getByRole("textbox");
  await userEvent.click(input);
  return input;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// Keep the actual Base UI popup: its typeahead used to prevent text entry.
describe("IssueFilterMenu search inputs", () => {
  it.each(["Assignee", "Creator", "Project", "Label"])(
    "types letters and spaces in the %s submenu and filters its options",
    async (section) => {
      renderFilterMenu();
      const input = await openSearch(section);

      await userEvent.type(input, "alpha beta");

      expect(input).toHaveValue("alpha beta");
      expect(input).toHaveFocus();
      expect(screen.getByRole("menuitemcheckbox", { name: "alpha beta" })).toBeInTheDocument();
      expect(screen.queryByRole("menuitemcheckbox", { name: "alpha gamma" })).not.toBeInTheDocument();
    },
  );

  it("keeps editing keys in the input and lets Escape dismiss the submenu", async () => {
    const store = renderFilterMenu();
    const input = await openSearch("Assignee");
    await userEvent.type(input, "alpha");

    for (const key of ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End", "Enter"]) {
      expect(fireEvent.keyDown(input, { key })).toBe(true);
      expect(input).toHaveFocus();
    }
    expect(store.getState().assigneeFilters).toEqual([]);

    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("textbox")).not.toBeInTheDocument());
    expect(screen.getByRole("menuitem", { name: "Assignee" })).toHaveFocus();
  });

  it("lets Tab close the menu without selecting an option", async () => {
    const store = renderFilterMenu();
    const input = await openSearch("Assignee");

    await userEvent.tab();
    expect(input).not.toHaveFocus();
    await userEvent.tab();

    await waitFor(() => expect(screen.queryAllByRole("menu")).toHaveLength(0));
    expect(store.getState().assigneeFilters).toEqual([]);
  });

  it("keeps IME candidate controls local until Chinese composition finishes", async () => {
    const store = renderFilterMenu();
    const input = await openSearch("Assignee");
    fireEvent.compositionStart(input);
    fireEvent.change(input, { target: { value: "中文" } });
    for (const key of ["ArrowDown", "Enter", "Escape"]) {
      expect(fireEvent.keyDown(input, { key, isComposing: true, keyCode: 229 })).toBe(true);
      expect(input).toHaveFocus();
    }
    fireEvent.compositionEnd(input, { data: "中文" });

    expect(input).toHaveValue("中文");
    expect(screen.getByRole("menuitemcheckbox", { name: "中文" })).toBeInTheDocument();
    expect(store.getState().assigneeFilters).toEqual([]);
  });
});
