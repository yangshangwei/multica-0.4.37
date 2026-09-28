// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { SkillTemplateDiscoveryItem } from "./skill-template-discovery";
import { filterSkillMarketItems, getSkillMarketCategories } from "./skill-market";

function item(name: string, source: "builtin" | "deployment", category?: string): SkillTemplateDiscoveryItem {
  return {
    template: { name, category, description: "Team review", content: "", version: 1, files: [] },
    presentation: {
      name,
      description: "Team review",
      searchNames: [name.toLowerCase()],
      searchText: `${name} Team review 审查`.toLowerCase(),
      isBuiltin: source === "builtin",
    },
    summary: "Team review",
    source,
  };
}

const ITEMS = [
  item("design-review", "deployment", "design"),
  item("code-review", "deployment", "quality"),
  item("builtin-review", "builtin", "quality"),
  item("unknown-category", "builtin", "future-category"),
];

describe("skill market filters", () => {
  it("combines source, category and normalized multilingual search", () => {
    expect(filterSkillMarketItems(ITEMS, { source: "deployment", category: "quality", search: " REVIEW " })
      .map(({ template }) => template.name)).toEqual(["code-review"]);
    expect(filterSkillMarketItems(ITEMS, { source: "all", category: null, search: "审查" })).toEqual(ITEMS);
    expect(filterSkillMarketItems(ITEMS, { source: "builtin", category: "quality", search: "absent" })).toEqual([]);
  });

  it("lists only available source categories in the shared display order", () => {
    expect(getSkillMarketCategories(ITEMS, "all")).toEqual(["design", "quality", "other"]);
    expect(getSkillMarketCategories(ITEMS, "deployment")).toEqual(["design", "quality"]);
    expect(getSkillMarketCategories([], "builtin")).toEqual([]);
  });

  it("uses the tolerant presentation reader for missing or unknown categories", () => {
    expect(filterSkillMarketItems(ITEMS, { source: "all", category: "other", search: "" })
      .map(({ template }) => template.name)).toEqual(["unknown-category"]);
  });
});
