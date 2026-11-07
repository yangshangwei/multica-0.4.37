import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../../test/i18n";
import { computeSkillListFacets } from "../hooks/use-skill-list-facets";
import { SkillListToolbar } from "./skill-list-toolbar";

const facets = computeSkillListFacets([]);
const emptyFilters = {
  usage: [], categories: [], origins: [], agents: [], creators: [], labels: [],
};

function renderToolbar(overrides: Partial<ComponentProps<typeof SkillListToolbar>> = {}) {
  const props: ComponentProps<typeof SkillListToolbar> = {
    search: "",
    onSearchChange: vi.fn(),
    filters: emptyFilters,
    onToggleFilter: vi.fn(),
    onClearFilters: vi.fn(),
    sortField: "updated",
    sortDirection: "desc",
    onSortFieldChange: vi.fn(),
    onSortDirectionChange: vi.fn(),
    hiddenColumns: [],
    onToggleColumn: vi.fn(),
    viewMode: "card",
    onViewModeChange: vi.fn(),
    facets,
    visibleCount: 0,
    ...overrides,
  };
  renderWithI18n(<SkillListToolbar {...props} />);
  return props;
}

describe("SkillListToolbar", () => {
  it("keeps compact filter and sort controls named when their text is hidden", () => {
    renderToolbar();
    const filter = screen.getByRole("button", { name: "Filter" });
    const sort = screen.getByRole("button", { name: "Sort by: Updated, Descending" });
    // jsdom does not evaluate Tailwind breakpoints; emulate the compact
    // presentation by hiding the same visible text nodes.
    for (const control of [filter, sort]) {
      for (const span of control.querySelectorAll("span")) span.hidden = true;
    }
    expect(filter).toHaveAccessibleName("Filter");
    expect(sort).toHaveAccessibleName("Sort by: Updated, Descending");
  });

  it("exposes active filters and clears them through the real menu keyboard flow", async () => {
    const user = userEvent.setup();
    const { onClearFilters } = renderToolbar({
      filters: { ...emptyFilters, categories: ["engineering"] },
    });
    const filter = screen.getByRole("button", { name: "Filter: 1 filter" });
    expect(within(filter).queryByRole("button")).not.toBeInTheDocument();
    filter.focus();
    await user.keyboard("{ArrowDown}");
    await user.keyboard("{End}");
    const clear = await screen.findByRole("menuitem", { name: "Clear filters" });
    expect(clear).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onClearFilters).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("offers only sorting in card view", async () => {
    const user = userEvent.setup();
    const { onSortDirectionChange } = renderToolbar();
    await user.click(screen.getByRole("button", { name: "Sort by: Updated, Descending" }));
    expect(screen.queryByText("Columns")).not.toBeInTheDocument();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Descending" }));
    expect(onSortDirectionChange).toHaveBeenCalledWith("asc");
  });

  it("keeps list column preferences editable", async () => {
    const user = userEvent.setup();
    const { onToggleColumn } = renderToolbar({ viewMode: "list" });
    await user.click(screen.getByRole("button", { name: "Sort by: Updated, Descending" }));
    expect(screen.getByText("Columns")).toBeInTheDocument();
    await user.click(screen.getByRole("switch", { name: "Category" }));
    expect(onToggleColumn).toHaveBeenCalledWith("category");
  });
});
