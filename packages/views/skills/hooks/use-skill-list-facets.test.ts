// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Agent, Label, MemberWithUser } from "@multica/core/types";
import { computeSkillListFacets, type SkillFacetRow } from "./use-skill-list-facets";

const agentA = { id: "a1", name: "Alpha" } as Agent;
const agentB = { id: "a2", name: "Beta" } as Agent;
const owner = { user_id: "u1", name: "Owner" } as MemberWithUser;
const pr = { id: "l1", name: "pr", color: "#3b82f6" } as Label;
const quality = { id: "l2", name: "quality", color: "#22c55e" } as Label;

function row(overrides: Partial<SkillFacetRow>): SkillFacetRow {
  return {
    agents: [],
    creator: null,
    originType: "manual",
    meta: { category: "other", icon: null },
    labels: [],
    ...overrides,
  };
}

describe("computeSkillListFacets", () => {
  it("returns zero counts for every category on an empty list", () => {
    const facets = computeSkillListFacets([]);
    expect(facets.total).toBe(0);
    expect(facets.categoryCounts).toEqual({
      research: 0, writing: 0, engineering: 0, operations: 0, data: 0, other: 0,
    });
    expect(facets.originCounts.size).toBe(0);
    expect(facets.labelOptions.size).toBe(0);
  });

  it("counts categories, origins, usage, agents, creators and labels", () => {
    const facets = computeSkillListFacets([
      row({ meta: { category: "engineering", icon: null }, labels: [pr, quality], agents: [agentA, agentB], creator: owner, originType: "github" }),
      row({ meta: { category: "engineering", icon: "bug" }, labels: [pr], agents: [agentA], creator: owner }),
      row({ meta: { category: "writing", icon: null }, originType: "clawhub" }),
    ]);
    expect(facets.total).toBe(3);
    expect(facets.usedCount).toBe(2);
    expect(facets.unusedCount).toBe(1);
    expect(facets.categoryCounts.engineering).toBe(2);
    expect(facets.categoryCounts.writing).toBe(1);
    expect(facets.categoryCounts.other).toBe(0);
    expect(facets.originCounts.get("github")).toBe(1);
    expect(facets.originCounts.get("manual")).toBe(1);
    expect(facets.originCounts.get("clawhub")).toBe(1);
    expect(facets.agentOptions.get("a1")?.count).toBe(2);
    expect(facets.agentOptions.get("a2")?.count).toBe(1);
    expect(facets.creatorOptions.get("u1")?.count).toBe(2);
    expect(facets.labelOptions.get("l1")).toEqual({ label: pr, count: 2 });
    expect(facets.labelOptions.get("l2")).toEqual({ label: quality, count: 1 });
    // Keyed by id, so a renamed label never splits into two options.
    expect([...facets.labelOptions.keys()]).toEqual(["l1", "l2"]);
  });
});
