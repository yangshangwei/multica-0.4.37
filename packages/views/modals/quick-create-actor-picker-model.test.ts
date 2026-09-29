// @vitest-environment node
import { describe, expect, it } from "vitest";
import { actorKey, buildActorCatalog, actorShortcuts, searchActors, actorPage } from "./quick-create-actor-picker-model";

const agents = Array.from({ length: 500 }, (_, index) => ({
  id: String(index), name: `Agent ${String(index).padStart(3, "0")}`,
  description: index === 499 ? `${"x".repeat(350)} tail responsibility` : "",
}));
const squads = Array.from({ length: 50 }, (_, index) => ({ id: String(index), name: `Squad ${index}`, description: "Squad responsibility" }));
const agentRef = (id: string) => ({ type: "agent" as const, id });

describe("quick-create actor projection", () => {
  it("uses typed identity and safely projects saved descriptions without inferring capabilities", () => {
    const catalog = buildActorCatalog([
      { id: "same", name: "Doctor", description: null },
      { id: "invalid", name: 4, description: {} },
      { id: "markdown", name: "Writer", description: "**Saved** [responsibility](https://example.com)" },
    ], [{ id: "same", name: "Doctor", description: "Own squad description" }]);
    expect(catalog.map(actorKey)).toEqual(["agent:same", "agent:invalid", "agent:markdown", "squad:same"]);
    expect(catalog.map((a) => a.preview)).toEqual(["", "", "Saved responsibility", "Own squad description"]);
    expect(catalog[1]?.name).toBe("");
  });

  it("resolves only available references, preserves pin order and excludes every favorite from recent", () => {
    const catalog = buildActorCatalog(agents, squads);
    const favorites = [agentRef("4"), agentRef("1"), agentRef("2"), agentRef("3"), agentRef("4"), agentRef("missing")];
    const recent = [agentRef("3"), { type: "squad" as const, id: "3" }, ...Array.from({length: 10}, (_, i) => agentRef(String(i))), agentRef("0")];
    const shortcuts = actorShortcuts(catalog, favorites, recent);
    expect(shortcuts.favorites.map(actorKey)).toEqual(["agent:4", "agent:1", "agent:2", "agent:3"]);
    expect(shortcuts.recent.map(actorKey)).toEqual(["squad:3", "agent:0", "agent:5", "agent:6", "agent:7"]);
  });

  it("ranks exact name, name substring, name pinyin, then full description with deterministic ties", () => {
    const catalog = buildActorCatalog([
      { id: "z", name: "Li", description: "" },
      { id: "a", name: "Li", description: "" },
      { id: "substring", name: "Alice", description: "" },
      { id: "pinyin", name: "李云龙", description: "" },
      { id: "description", name: "Ada", description: "LI responsibility" },
    ], [{ id: "a", name: "Li", description: "" }]);
    expect(searchActors(catalog, " LI ", "all", "en").map(actorKey)).toEqual([
      "agent:a", "agent:z", "squad:a", "agent:substring", "agent:pinyin", "agent:description",
    ]);
    expect(searchActors(catalog, "li", "squad", "en").map(actorKey)).toEqual(["squad:a"]);
    expect(searchActors(catalog, "lyl", "agent", "zh-Hans").map(actorKey)).toEqual(["agent:pinyin"]);
  });

  it("searches all 550 actors including beyond the preview and every display page", () => {
    const catalog = buildActorCatalog(agents, squads);
    const all = searchActors(catalog, "   ", "all", "en");
    expect(all).toHaveLength(550);
    expect(actorPage(all, 50)).toEqual({ items: all.slice(0, 50), remaining: 500 });
    expect(actorPage(all, 550).items.at(-1)).toEqual(all.at(-1));
    expect(searchActors(catalog, "TAIL RESPONSIBILITY", "agent", "en").map(actorKey)).toEqual(["agent:499"]);
    expect(catalog[499]?.preview).not.toContain("tail responsibility");
    expect(searchActors(catalog, "TAIL RESPONSIBILITY", "squad", "en")).toEqual([]);
    expect(actorPage([], 50)).toEqual({ items: [], remaining: 0 });
  });
});
