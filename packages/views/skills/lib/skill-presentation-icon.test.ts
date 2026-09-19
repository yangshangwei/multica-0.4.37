// @vitest-environment node
import { describe, expect, it } from "vitest";
import { SKILL_CATEGORIES, SKILL_ICON_NAMES } from "@multica/core/skills";
import { SKILL_CATEGORY_TONE, SKILL_ICON_COMPONENTS, resolveSkillIcon } from "./skill-presentation-icon";

describe("skill presentation icon map", () => {
  it("has a component for every whitelisted icon and nothing else", () => {
    expect(Object.keys(SKILL_ICON_COMPONENTS).sort()).toEqual([...SKILL_ICON_NAMES].sort());
    for (const name of SKILL_ICON_NAMES) expect(SKILL_ICON_COMPONENTS[name]).toBeTruthy();
  });

  it("has a tone for every category", () => {
    expect(Object.keys(SKILL_CATEGORY_TONE).sort()).toEqual([...SKILL_CATEGORIES].sort());
  });

  it("resolves override first, then category default", () => {
    expect(resolveSkillIcon({ category: "data", icon: "table" })).toBe(
      SKILL_ICON_COMPONENTS.table,
    );
    expect(resolveSkillIcon({ category: "data", icon: null })).toBe(
      SKILL_ICON_COMPONENTS.database,
    );
  });
});
