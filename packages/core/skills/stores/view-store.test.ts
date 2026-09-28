// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { DEFAULT_HIDDEN_COLUMNS, EMPTY_SKILL_FILTERS, useSkillsViewStore } from "./view-store";
import { setCurrentWorkspace } from "../../platform/workspace-storage";

// jsdom on purpose: the store persists through `defaultStorage`, which
// branches on `typeof window`. Under node every write would be dropped
// silently and the persistence assertions below could never fail.

const flush = () => new Promise((resolve) => queueMicrotask(() => resolve(null)));

// Node 25 ships a partial `localStorage` shim under jsdom that's missing
// `clear`/`removeItem`; replace it with a real in-memory Storage so persist
// can round-trip values.
beforeAll(() => {
  if (typeof globalThis.localStorage?.clear !== "function") {
    const values = new Map<string, string>();
    const storage: Storage = {
      get length() { return values.size; },
      clear: () => values.clear(),
      getItem: (k) => values.get(k) ?? null,
      key: (i) => Array.from(values.keys())[i] ?? null,
      removeItem: (k) => { values.delete(k); },
      setItem: (k, v) => { values.set(k, v); },
    };
    Object.defineProperty(globalThis, "localStorage", { configurable: true, value: storage });
    Object.defineProperty(window, "localStorage", { configurable: true, value: storage });
  }
});

beforeEach(async () => {
  setCurrentWorkspace(null, null);
  localStorage.clear();
  await useSkillsViewStore.persist.rehydrate();
  await flush();
});

afterEach(() => {
  setCurrentWorkspace(null, null);
});

describe("useSkillsViewStore", () => {
  it("defaults to the card view and hides labels/source/created", () => {
    expect(useSkillsViewStore.getState().viewMode).toBe("card");
    expect(DEFAULT_HIDDEN_COLUMNS).toEqual(["labels", "source", "created"]);
  });

  it("starts with an unresolved library view and unfiltered, expanded templates", () => {
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: null,
      marketSource: "all",
      marketCategory: null,
      templatesCollapsed: false,
    });
  });

  it("changes market preferences independently of workspace filters", () => {
    const state = useSkillsViewStore.getState();
    state.selectCategory("engineering");
    state.toggleFilter("origins", "github");
    state.setLibraryView("market");
    state.setMarketSource("deployment");
    state.setMarketCategory("data");
    state.setTemplatesCollapsed(true);

    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: "market",
      marketSource: "deployment",
      marketCategory: "data",
      templatesCollapsed: true,
      filters: { categories: ["engineering"], origins: ["github"] },
    });

    state.clearFilters();
    expect(useSkillsViewStore.getState().marketCategory).toBe("data");
    state.setLibraryView("workspace");
    state.setMarketSource("builtin");
    state.setMarketCategory(null);
    state.setTemplatesCollapsed(false);

    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: "workspace",
      marketSource: "builtin",
      marketCategory: null,
      templatesCollapsed: false,
    });

    state.setLibraryView(null);
    state.setMarketSource("all");
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: null,
      marketSource: "all",
    });
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

  it("merge falls back to the default, not the in-memory value, for a key the payload never stored", () => {
    const merge = useSkillsViewStore.persist.getOptions().merge!;
    const merged = merge(
      { hiddenColumns: ["source"] },
      { ...useSkillsViewStore.getState(), viewMode: "list", sortField: "name" },
    ) as ReturnType<typeof useSkillsViewStore.getState>;
    expect(merged.viewMode).toBe("card");
    expect(merged.sortField).toBe("updated");
    expect(merged.hiddenColumns).toEqual(["source"]);
  });
});

describe("useSkillsViewStore persistence", () => {
  it("persists the view mode under the workspace-namespaced key, without actions", async () => {
    setCurrentWorkspace("acme", "ws_a");
    await flush();
    useSkillsViewStore.getState().setViewMode("list");

    const raw = localStorage.getItem("multica_skills_view:acme");
    expect(raw).not.toBeNull();
    const parsed = JSON.parse(raw as string);
    expect(Object.keys(parsed.state).sort()).toEqual([
      "filters",
      "hiddenColumns",
      "libraryView",
      "marketCategory",
      "marketSource",
      "sortDirection",
      "sortField",
      "templatesCollapsed",
      "viewMode",
    ]);
    expect(parsed.state.viewMode).toBe("list");
    expect(parsed.version).toBe(1);
  });

  it("rehydrates saved market preferences after leaving the workspace", async () => {
    setCurrentWorkspace("acme", "ws_a");
    await flush();
    const state = useSkillsViewStore.getState();
    state.setLibraryView("market");
    state.setMarketSource("deployment");
    state.setMarketCategory("engineering");
    state.setTemplatesCollapsed(true);

    const raw = localStorage.getItem("multica_skills_view:acme");
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).state).toMatchObject({
      libraryView: "market",
      marketSource: "deployment",
      marketCategory: "engineering",
      templatesCollapsed: true,
    });

    setCurrentWorkspace(null, null);
    await flush();
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: null,
      marketSource: "all",
      marketCategory: null,
      templatesCollapsed: false,
    });

    setCurrentWorkspace("acme", "ws_a");
    await flush();
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: "market",
      marketSource: "deployment",
      marketCategory: "engineering",
      templatesCollapsed: true,
    });
  });

  it("isolates market preferences across workspaces and resets unsaved workspaces", async () => {
    setCurrentWorkspace("acme", "ws_a");
    await flush();
    const state = useSkillsViewStore.getState();
    state.setLibraryView("market");
    state.setMarketSource("deployment");
    state.setMarketCategory("engineering");
    state.setTemplatesCollapsed(true);

    setCurrentWorkspace("beta", "ws_b");
    await flush();
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: null,
      marketSource: "all",
      marketCategory: null,
      templatesCollapsed: false,
    });
    state.setLibraryView("workspace");
    state.setMarketSource("builtin");
    state.setMarketCategory("writing");

    setCurrentWorkspace("acme", "ws_a");
    await flush();
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: "market",
      marketSource: "deployment",
      marketCategory: "engineering",
      templatesCollapsed: true,
    });

    setCurrentWorkspace("beta", "ws_b");
    await flush();
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: "workspace",
      marketSource: "builtin",
      marketCategory: "writing",
      templatesCollapsed: false,
    });
  });

  it.each([0, 1])("backfills market defaults when switching to a v%i payload without them", async (version) => {
    localStorage.setItem(
      "multica_skills_view:beta",
      JSON.stringify({
        state: { viewMode: "list", hiddenColumns: ["source", "created"] },
        version,
      }),
    );
    setCurrentWorkspace("acme", "ws_a");
    await flush();
    const state = useSkillsViewStore.getState();
    state.setLibraryView("market");
    state.setMarketSource("deployment");
    state.setMarketCategory("engineering");
    state.setTemplatesCollapsed(true);

    setCurrentWorkspace("beta", "ws_b");
    await flush();
    await flush();
    expect(useSkillsViewStore.getState()).toMatchObject({
      libraryView: null,
      marketSource: "all",
      marketCategory: null,
      templatesCollapsed: false,
      viewMode: "list",
    });

    state.setSortField("name");
    const beta = JSON.parse(localStorage.getItem("multica_skills_view:beta") as string);
    expect(beta.state).toMatchObject({
      libraryView: null,
      marketSource: "all",
      marketCategory: null,
      templatesCollapsed: false,
    });
  });

  it("keeps each workspace's view mode apart and resets where nothing is saved", async () => {
    localStorage.setItem(
      "multica_skills_view:acme",
      JSON.stringify({ state: { viewMode: "list" }, version: 1 }),
    );

    setCurrentWorkspace("acme", "ws_a");
    await flush();
    await flush();
    expect(useSkillsViewStore.getState().viewMode).toBe("list");

    setCurrentWorkspace("beta", "ws_b");
    await flush();
    await flush();
    expect(useSkillsViewStore.getState().viewMode).toBe("card");
    expect(localStorage.getItem("multica_skills_view:acme")).not.toBeNull();
  });

  it("hides the labels column for a v0 payload that predates it", async () => {
    // Persisted by the pre-categories build: `labels` was not a column yet,
    // so its absence from hiddenColumns is not a choice to show it.
    localStorage.setItem(
      "multica_skills_view:acme",
      JSON.stringify({
        state: {
          hiddenColumns: ["source", "created"],
          filters: { usage: ["used"], origins: [], agents: [], creators: [] },
        },
        version: 0,
      }),
    );

    setCurrentWorkspace("acme", "ws_a");
    await flush();
    await flush();

    const state = useSkillsViewStore.getState();
    expect(state.hiddenColumns).toEqual(["source", "created", "labels"]);
    expect(state.filters.categories).toEqual([]);
    expect(state.filters.labels).toEqual([]);
    expect(state.filters.usage).toEqual(["used"]);
  });

  it("does not carry the previous workspace's view mode into a v0 payload that never stored one", async () => {
    localStorage.setItem(
      "multica_skills_view:acme",
      JSON.stringify({ state: { viewMode: "list" }, version: 1 }),
    );
    // Persisted before `viewMode` existed: the key is absent, not chosen.
    localStorage.setItem(
      "multica_skills_view:beta",
      JSON.stringify({
        state: {
          hiddenColumns: ["source", "created"],
          filters: { usage: [], origins: [], agents: [], creators: [] },
        },
        version: 0,
      }),
    );

    setCurrentWorkspace("acme", "ws_a");
    await flush();
    await flush();
    expect(useSkillsViewStore.getState().viewMode).toBe("list");

    setCurrentWorkspace("beta", "ws_b");
    await flush();
    await flush();
    expect(useSkillsViewStore.getState().viewMode).toBe("card");
    // The migration writes the merged state back; it must not have frozen
    // acme's choice into beta's payload.
    const beta = JSON.parse(localStorage.getItem("multica_skills_view:beta") as string);
    expect(beta.state.viewMode).toBe("card");
  });

  it("respects a v1 payload where the user chose to show labels", async () => {
    localStorage.setItem(
      "multica_skills_view:acme",
      JSON.stringify({ state: { hiddenColumns: ["source"] }, version: 1 }),
    );

    setCurrentWorkspace("acme", "ws_a");
    await flush();
    await flush();
    expect(useSkillsViewStore.getState().hiddenColumns).toEqual(["source"]);
  });
});
