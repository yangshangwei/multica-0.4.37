// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  EMPTY_SKILL_PRESENTATION,
  SKILL_CATEGORIES,
  SKILL_CATEGORY_DEFAULT_ICON,
  SKILL_ICON_NAMES,
  readSkillPresentationMeta,
  resolveSkillIconName,
  writeSkillPresentationMeta,
} from "./presentation";

describe("skill presentation constants", () => {
  it("every category has a default icon inside the whitelist", () => {
    for (const category of SKILL_CATEGORIES) {
      expect(SKILL_ICON_NAMES).toContain(SKILL_CATEGORY_DEFAULT_ICON[category]);
    }
  });

  it("icon whitelist is sorted and unique so the Go parity test can diff it", () => {
    const sorted = [...SKILL_ICON_NAMES].sort();
    expect([...SKILL_ICON_NAMES]).toEqual(sorted);
    expect(new Set(SKILL_ICON_NAMES).size).toBe(SKILL_ICON_NAMES.length);
  });
});

describe("readSkillPresentationMeta", () => {
  it("returns the shared empty meta for missing or malformed presentation", () => {
    expect(readSkillPresentationMeta(undefined)).toBe(EMPTY_SKILL_PRESENTATION);
    expect(readSkillPresentationMeta({})).toBe(EMPTY_SKILL_PRESENTATION);
    expect(readSkillPresentationMeta({ presentation: "nope" })).toBe(EMPTY_SKILL_PRESENTATION);
    expect(readSkillPresentationMeta({ presentation: [] })).toBe(EMPTY_SKILL_PRESENTATION);
    expect(readSkillPresentationMeta({ presentation: null })).toBe(EMPTY_SKILL_PRESENTATION);
  });

  it("falls back per field on invalid values", () => {
    expect(
      readSkillPresentationMeta({
        presentation: { category: "marketing", icon: "not-an-icon" },
      }),
    ).toEqual({ category: "other", icon: null });
    expect(
      readSkillPresentationMeta({
        presentation: { category: 3, icon: 1 },
      }),
    ).toEqual({ category: "other", icon: null });
  });

  it("reads valid values and canonicalizes a default-matching icon to null", () => {
    expect(
      readSkillPresentationMeta({
        presentation: { category: "engineering", icon: "git-pull-request" },
      }),
    ).toEqual({ category: "engineering", icon: "git-pull-request" });
    expect(
      readSkillPresentationMeta({ presentation: { category: "engineering", icon: "code" } }),
    ).toEqual({ category: "engineering", icon: null });
  });

  it("ignores a legacy tags array left over from an earlier config shape", () => {
    expect(
      readSkillPresentationMeta({
        presentation: { category: "writing", tags: ["docs"] },
      }),
    ).toEqual({ category: "writing", icon: null });
    expect(
      readSkillPresentationMeta({ presentation: { tags: ["docs"] } }),
    ).toBe(EMPTY_SKILL_PRESENTATION);
  });
});

describe("resolveSkillIconName", () => {
  it("prefers the override and otherwise uses the category default", () => {
    expect(resolveSkillIconName({ category: "data", icon: "table" })).toBe("table");
    expect(resolveSkillIconName({ category: "data", icon: null })).toBe("database");
  });
});

describe("writeSkillPresentationMeta", () => {
  it("preserves sibling config keys and does not mutate the input", () => {
    const config = { origin: { type: "github" }, presentation: { category: "data" } };
    const out = writeSkillPresentationMeta(config, {
      category: "writing",
      icon: "mail",
    });
    expect(out).toEqual({
      origin: { type: "github" },
      presentation: { category: "writing", icon: "mail" },
    });
    expect(config.presentation).toEqual({ category: "data" });
  });

  it("drops the presentation key entirely when everything is default", () => {
    expect(
      writeSkillPresentationMeta({ origin: { type: "manual" }, presentation: { category: "data" } }, EMPTY_SKILL_PRESENTATION),
    ).toEqual({ origin: { type: "manual" } });
  });

  it("omits an icon equal to the category default and writes category for icon-only meta", () => {
    expect(
      writeSkillPresentationMeta({}, { category: "research", icon: "microscope" }),
    ).toEqual({ presentation: { category: "research" } });
    expect(
      writeSkillPresentationMeta({}, { category: "other", icon: "rocket" }),
    ).toEqual({ presentation: { category: "other", icon: "rocket" } });
  });

  it("does not carry a legacy tags key through a write", () => {
    expect(
      writeSkillPresentationMeta(
        { presentation: { category: "data", tags: ["x"] } },
        { category: "data", icon: null },
      ),
    ).toEqual({ presentation: { category: "data" } });
  });

  it("round-trips through the reader", () => {
    const meta = { category: "operations" as const, icon: "server" as const };
    expect(readSkillPresentationMeta(writeSkillPresentationMeta({}, meta))).toEqual(meta);
  });
});
