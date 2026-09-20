// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithI18n } from "../../test/i18n";
import { computeSkillListFacets } from "../hooks/use-skill-list-facets";
import { SkillCategoryChips } from "./skill-category-chips";

// Narrow-container counterpart of the sidebar. Same facets, same single-select
// contract; the sidebar suite covers the sources group, which the chip row
// deliberately leaves to the toolbar's Filter dropdown.

const facets = computeSkillListFacets([
  { agents: [], creator: null, labels: [], originType: "manual", meta: { category: "engineering", icon: null } },
  { agents: [], creator: null, labels: [], originType: "github", meta: { category: "engineering", icon: null } },
  { agents: [], creator: null, labels: [], originType: "github", meta: { category: "writing", icon: null } },
]);

function renderChips(categories: ("engineering" | "writing")[] = []) {
  const onSelectCategory = vi.fn();
  renderWithI18n(
    <SkillCategoryChips
      facets={facets}
      filters={{ usage: [], categories, origins: [], agents: [], creators: [], labels: [] }}
      onSelectCategory={onSelectCategory}
    />,
  );
  return { onSelectCategory };
}

describe("SkillCategoryChips", () => {
  it("renders All plus every category with counts and marks All pressed by default", () => {
    renderChips();
    const group = screen.getByRole("group", { name: "Categories" });
    const all = within(group).getByRole("button", { name: /^All/ });
    expect(all).toHaveAttribute("aria-pressed", "true");
    expect(all).toHaveAttribute("data-active");
    expect(all).toHaveTextContent("3");
    expect(within(group).getByRole("button", { name: /Engineering/ })).toHaveTextContent("2");
    expect(within(group).getByRole("button", { name: /Writing/ })).toHaveTextContent("1");
    expect(within(group).getByRole("button", { name: /Data/ })).toHaveTextContent("0");
    // Seven chips: All + six categories. No source buttons in the narrow row.
    expect(within(group).getAllByRole("button")).toHaveLength(7);
    expect(within(group).queryByRole("button", { name: /GitHub/ })).toBeNull();
  });

  it("marks the selected category active and reports single-select clicks", () => {
    const { onSelectCategory } = renderChips(["engineering"]);
    const group = screen.getByRole("group", { name: "Categories" });
    const engineering = within(group).getByRole("button", { name: /Engineering/ });
    expect(engineering).toHaveAttribute("aria-pressed", "true");
    expect(engineering).toHaveAttribute("data-active");
    expect(within(group).getByRole("button", { name: /^All/ })).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(within(group).getByRole("button", { name: /Writing/ }));
    expect(onSelectCategory).toHaveBeenCalledWith("writing");
    fireEvent.click(within(group).getByRole("button", { name: /^All/ }));
    expect(onSelectCategory).toHaveBeenCalledWith(null);
  });
});
