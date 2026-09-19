// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { DEFAULT_HIDDEN_COLUMNS, EMPTY_SKILL_FILTERS, useSkillsViewStore } from "./view-store";

describe("useSkillsViewStore", () => {
  beforeEach(() => {
    useSkillsViewStore.setState({
      viewMode: "card",
      filters: EMPTY_SKILL_FILTERS,
      hiddenColumns: DEFAULT_HIDDEN_COLUMNS,
    });
  });

  it("defaults to the card view and hides labels/source/created", () => {
    expect(useSkillsViewStore.getState().viewMode).toBe("card");
    expect(DEFAULT_HIDDEN_COLUMNS).toEqual(["labels", "source", "created"]);
  });

  it("toggleFilter treats labels as a multi-select dimension of label ids", () => {
    const s = useSkillsViewStore.getState();
    s.toggleFilter("labels", "lbl-1");
    s.toggleFilter("labels", "lbl-2");
    expect(useSkillsViewStore.getState().filters.labels).toEqual(["lbl-1", "lbl-2"]);
    s.toggleFilter("labels", "lbl-1");
    expect(useSkillsViewStore.getState().filters.labels).toEqual(["lbl-2"]);
    s.clearFilters();
    expect(useSkillsViewStore.getState().filters.labels).toEqual([]);
  });

  it("selectCategory is single-select and toggles off on repeat", () => {
    const s = useSkillsViewStore.getState();
    s.selectCategory("engineering");
    expect(useSkillsViewStore.getState().filters.categories).toEqual(["engineering"]);
    s.selectCategory("data");
    expect(useSkillsViewStore.getState().filters.categories).toEqual(["data"]);
    s.selectCategory("data");
    expect(useSkillsViewStore.getState().filters.categories).toEqual([]);
    s.selectCategory("data");
    s.selectCategory(null);
    expect(useSkillsViewStore.getState().filters.categories).toEqual([]);
  });

  it("clearFilters also clears the category dimension", () => {
    const s = useSkillsViewStore.getState();
    s.selectCategory("writing");
    s.toggleFilter("origins", "github");
    s.clearFilters();
    expect(useSkillsViewStore.getState().filters).toEqual(EMPTY_SKILL_FILTERS);
  });

  it("setViewMode switches and persists into the partialized payload", () => {
    useSkillsViewStore.getState().setViewMode("list");
    expect(useSkillsViewStore.getState().viewMode).toBe("list");
    const persisted = useSkillsViewStore.persist.getOptions().partialize?.(
      useSkillsViewStore.getState(),
    ) as { viewMode?: string };
    expect(persisted.viewMode).toBe("list");
  });

  it("merge backfills categories and labels for a payload persisted before the dimensions existed", () => {
    const merge = useSkillsViewStore.persist.getOptions().merge!;
    const merged = merge(
      { filters: { usage: ["used"], origins: [], agents: [], creators: [] } },
      useSkillsViewStore.getState(),
    ) as ReturnType<typeof useSkillsViewStore.getState>;
    expect(merged.filters.categories).toEqual([]);
    expect(merged.filters.labels).toEqual([]);
    expect(merged.filters.usage).toEqual(["used"]);
    expect(merged.viewMode).toBe("card");
  });
});
