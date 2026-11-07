// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  getAgentCategory,
  getAgentCategoryKey,
  getAgentCategoryNames,
  isAgentCategoryValid,
  resolveAgentDirectoryCategory,
} from "./category";
import type { AgentRoleMetadata } from "./discovery";

describe("agent categories", () => {
  it("validates Unicode code points after trimming and rejects embedded controls", () => {
    expect(isAgentCategoryValid(" \n研发\t ")).toBe(true);
    expect(isAgentCategoryValid("😀".repeat(50))).toBe(true);
    expect(isAgentCategoryValid("😀".repeat(51))).toBe(false);
    expect(isAgentCategoryValid("")).toBe(true);
    for (const value of ["Dev\nOps", "Dev\tOps", "Dev\u0000Ops", "Dev\u007fOps", "Dev\u0085Ops"]) {
      expect(isAgentCategoryValid(value)).toBe(false);
    }
  });

  it("uses the same blank key for missing categories and keeps custom names distinct", () => {
    expect(getAgentCategory({})).toBe("");
    expect(getAgentCategory({ category: "   " })).toBe("");
    expect(getAgentCategory({ category: "  研发  " })).toBe("研发");
    expect(getAgentCategoryNames([
      {}, { category: "" }, { category: "Operations" }, { category: "Engineering" },
      { category: " Engineering " }, { category: "engineering" },
    ])).toEqual(["engineering", "Engineering", "Operations"]);
  });
});

describe("effective directory category", () => {
  const coordinator: AgentRoleMetadata = {
    templateKey: "team-lead", title: "Team lead", kind: "coordinator",
  };

  it.each([
    ["General-purpose agents", "other"], ["通用智能体", "other"],
    ["Specialists", "specialist"], ["专业角色", "specialist"],
    ["Coordinators", "coordinator"], ["统筹角色", "coordinator"],
    ["  SPECIALISTS  ", "specialist"],
  ])("uses a stable key for the preset name %s", (name, preset) => {
    expect(getAgentCategoryKey(name)).toBe(`preset:${preset}`);
  });

  it("separates literal custom names from reserved keys without folding custom casing", () => {
    expect(getAgentCategoryKey("  Research  ")).toBe("custom:Research");
    expect(getAgentCategoryKey("research")).toBe("custom:research");
    expect(getAgentCategoryKey("preset:specialist")).toBe("custom:preset:specialist");
    expect(getAgentCategoryKey("custom:Research")).toBe("custom:custom:Research");
  });

  it("classifies a manually selected specialist before template provenance", () => {
    expect(resolveAgentDirectoryCategory({ category: "专业角色" }, null)).toEqual({
      key: "preset:specialist", preset: "specialist", name: "专业角色",
    });
    expect(resolveAgentDirectoryCategory({ category: "Specialists", template_key: "team-lead" }, coordinator)).toEqual({
      key: "preset:specialist", preset: "specialist", name: "Specialists",
    });
  });

  it("preserves custom overrides even while template metadata is unavailable", () => {
    expect(resolveAgentDirectoryCategory({ category: "  Research  ", template_key: "team-lead" }, null, false)).toEqual({
      key: "custom:Research", preset: null, name: "Research",
    });
  });

  it("uses template role and then general-purpose fallback for cleared categories", () => {
    expect(resolveAgentDirectoryCategory({ category: " ", template_key: "team-lead" }, coordinator)).toMatchObject({
      key: "preset:coordinator", preset: "coordinator",
    });
    expect(resolveAgentDirectoryCategory({ template_key: "unknown" }, null)).toMatchObject({
      key: "preset:other", preset: "other",
    });
    expect(resolveAgentDirectoryCategory({}, null, false)).toMatchObject({
      key: "preset:other", preset: "other",
    });
  });

  it("leaves template fallback unresolved until required metadata is ready", () => {
    expect(resolveAgentDirectoryCategory({ category: "", template_key: "team-lead" }, null, false)).toBeNull();
  });
});
