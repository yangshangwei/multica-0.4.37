// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithI18n } from "../../test/i18n";
import { computeSkillListFacets } from "../hooks/use-skill-list-facets";
import { SkillCategorySidebar } from "./skill-category-sidebar";

const facets = computeSkillListFacets([
  { agents: [], creator: null, labels: [], originType: "manual", meta: { category: "engineering", icon: null } },
  { agents: [], creator: null, labels: [], originType: "github", meta: { category: "engineering", icon: null } },
  { agents: [], creator: null, labels: [], originType: "github", meta: { category: "writing", icon: null } },
]);

function renderSidebar(categories: ("engineering" | "writing")[] = [], origins: ("manual" | "github")[] = []) {
  const onSelectCategory = vi.fn();
  const onToggleOrigin = vi.fn();
  renderWithI18n(
    <SkillCategorySidebar
      facets={facets}
      filters={{ usage: [], categories, origins, agents: [], creators: [], labels: [] }}
      onSelectCategory={onSelectCategory}
      onToggleOrigin={onToggleOrigin}
    />,
  );
  return { onSelectCategory, onToggleOrigin };
}

describe("SkillCategorySidebar", () => {
  it("shows every category with its count and marks All active by default", () => {
    renderSidebar();
    const nav = screen.getByRole("navigation", { name: "Categories" });
    expect(within(nav).getByRole("button", { name: /^All/ })).toHaveAttribute("data-active");
    expect(within(nav).getByRole("button", { name: /Development & integration/ })).toHaveTextContent("2");
    expect(within(nav).getByRole("button", { name: /Collaboration & knowledge/ })).toHaveTextContent("1");
    expect(within(nav).getByRole("button", { name: /Data & automation/ })).toHaveTextContent("0");
  });

  it("marks the selected category active and reports clicks", () => {
    const { onSelectCategory } = renderSidebar(["engineering"]);
    const nav = screen.getByRole("navigation", { name: "Categories" });
    expect(within(nav).getByRole("button", { name: /Development & integration/ })).toHaveAttribute("data-active");
    expect(within(nav).getByRole("button", { name: /^All/ })).not.toHaveAttribute("data-active");

    fireEvent.click(within(nav).getByRole("button", { name: /Development & integration/ }));
    expect(onSelectCategory).toHaveBeenCalledWith("engineering");
    fireEvent.click(within(nav).getByRole("button", { name: /^All/ }));
    expect(onSelectCategory).toHaveBeenCalledWith(null);
  });

  it("lists only sources with skills and toggles the origin filter", () => {
    const { onToggleOrigin } = renderSidebar([], ["github"]);
    const nav = screen.getByRole("navigation", { name: "Categories" });
    const github = within(nav).getByRole("button", { name: /From GitHub/ });
    expect(github).toHaveAttribute("aria-pressed", "true");
    expect(github).toHaveTextContent("2");
    expect(within(nav).queryByRole("button", { name: /ClawHub/i })).not.toBeInTheDocument();

    fireEvent.click(within(nav).getByRole("button", { name: /Created manually/ }));
    expect(onToggleOrigin).toHaveBeenCalledWith("manual");
  });
});
