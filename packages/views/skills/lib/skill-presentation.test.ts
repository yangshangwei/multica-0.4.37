// @vitest-environment node

import { readdirSync, readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { createI18n } from "@multica/core/i18n/react";
import { parseFrontmatter } from "@multica/core/skills/frontmatter";
import en from "../../locales/en/skills.json";
import zh from "../../locales/zh-Hans/skills.json";
import {
  getBuiltinRoleSkillPresentation,
  getBuiltinRoleSkillSummary,
  getSkillPresentation,
  type SkillPresentationInput,
} from "./skill-presentation";

const i18n = createI18n("zh-Hans", {
  en: { skills: en },
  "zh-Hans": { skills: zh },
});
const zhT = i18n.getFixedT("zh-Hans", "skills");
const enT = i18n.getFixedT("en", "skills");

const sourceDirectory = new URL("../../../../server/internal/service/builtin_role_skills/", import.meta.url);
const embeddedNames = readdirSync(sourceDirectory, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
  .sort();
const names = Object.entries(zh.builtin_role_skills).map(([name, entry]) =>
  [name as keyof typeof zh.builtin_role_skills, entry.name] as const,
);

// Defaults shipped in ca3a79d18 and retained by existing workspace copies.
const legacyDescriptions = [
  [
    "multica-release-check",
    "Use before any release or production-affecting action: the gate, the rollback, and the approval request that must precede execution.",
    "用于发布或影响生产环境的操作前：完成必要检查、准备回滚方案，并在执行前申请审批。",
  ],
  [
    "multica-architecture-decision-record",
    "Use when a technical decision will constrain later work: writes an ADR with context, the decision, the rejected alternatives and the consequences.",
    "用于会影响后续工作的技术决策：编写 ADR，记录背景、决策、被否决的备选方案，以及决策的影响。",
  ],
] as const;

function builtin(
  name: keyof typeof en.builtin_role_skills,
  description = zh.builtin_role_skills[name].description,
): SkillPresentationInput {
  return {
    name,
    description,
    config: { origin: { type: "builtin_role_skill", name, version: 1 } },
  };
}

describe("built-in role skill presentation", () => {
  it("covers every embedded role skill in both retained locale catalogs", () => {
    for (const catalog of [en, zh]) {
      expect(Object.keys(catalog.builtin_role_skills).sort()).toEqual(embeddedNames);
    }
  });

  it.each(embeddedNames)("recognizes the embedded template %s", (name) => {
    expect(getBuiltinRoleSkillPresentation(name, enT)?.isBuiltin).toBe(true);
  });

  it.each([
    ["en", en], ["zh-Hans", zh],
  ] as const)("localizes real source descriptions with only %s loaded", (locale, catalog) => {
    const instance = createI18n(locale, { [locale]: { skills: catalog } });
    const t = instance.getFixedT(locale, "skills");
    for (const [name] of names) {
      const content = readFileSync(new URL(`${name}/SKILL.md`, sourceDirectory), "utf8");
      const description = parseFrontmatter(content).frontmatter?.description;
      expect(typeof description).toBe("string");
      const skill = builtin(name, description);
      const before = structuredClone(skill);
      const presentation = getSkillPresentation(skill, t);
      expect(presentation).toMatchObject({
        name: catalog.builtin_role_skills[name].name,
        description: catalog.builtin_role_skills[name].description,
        isBuiltin: true,
      });
      for (const expected of [en.builtin_role_skills[name].description, zh.builtin_role_skills[name].description, presentation.description]) {
        expect(presentation.searchText).toContain(expected.toLowerCase());
      }
      expect(skill).toEqual(before);
    }
  });

  it.each(["multica-experience-validation", "multica-migration-review"] as const)(
    "preserves customized and unverified copies of %s",
    (name) => {
      const skill = builtin(name);
      const customDescription = `${skill.description} Team-specific scope.`;
      const custom = { ...skill, description: customDescription };
      expect(getSkillPresentation(custom, enT).description).toBe(customDescription);
      expect(getSkillPresentation(custom, enT).searchText)
        .not.toContain(en.builtin_role_skills[name].description.toLowerCase());
      for (const input of [{ ...skill, config: undefined }, { ...skill, name: "team-customized" }]) {
        expect(getSkillPresentation(input, enT)).toMatchObject({
          name: input.name, description: input.description, isBuiltin: false,
        });
      }
    },
  );

  it("recognizes debugging as a platform built-in before materialization", () => {
    expect(getBuiltinRoleSkillPresentation("multica-debugging", zhT)).toMatchObject({
      name: "根因分析",
      isBuiltin: true,
    });
  });

  it.each([
    ["en", en, "multica-debugging"],
    ["zh-Hans", zh, "根因分析"],
  ] as const)("shows debugging in a provider with only %s loaded", (locale, catalog, name) => {
    const instance = createI18n(locale, { [locale]: { skills: catalog } });
    const t = instance.getFixedT(locale, "skills");
    const skill = builtin("multica-debugging");
    const before = structuredClone(skill);

    expect(getBuiltinRoleSkillPresentation(skill.name, t)).toMatchObject({
      name,
      description: catalog.builtin_role_skills["multica-debugging"].description,
      isBuiltin: true,
    });
    const presentation = getSkillPresentation(skill, t);
    expect(presentation).toMatchObject({
      name,
      description: catalog.builtin_role_skills["multica-debugging"].description,
      isBuiltin: true,
    });
    expect(presentation.searchNames).toContain(name.toLowerCase());
    expect(presentation.searchText).toContain(presentation.description.toLowerCase());
    expect(skill).toEqual(before);
  });

  it("labels the progress report skill in the built-in catalog", () => {
    expect(getBuiltinRoleSkillPresentation("multica-progress-report", zhT)?.name)
      .toBe("进展报告");
  });

  it.each(legacyDescriptions)(
    "translates the historical default for %s without changing the workspace copy",
    (name, description, chinese) => {
      const skill = { ...builtin(name), description };
      const before = structuredClone(skill);
      for (const locale of ["en", "zh-Hans"] as const) {
        const instance = createI18n(locale, {
          [locale]: { skills: locale === "en" ? en : zh },
        });
        const presentation = getSkillPresentation(skill, instance.getFixedT(locale, "skills"));
        expect(presentation.description).toBe(locale === "en" ? description : chinese);
        expect(presentation.searchText).toContain(chinese.toLowerCase());
        expect(presentation.searchText).toContain(description.toLowerCase());
      }
      expect(skill).toEqual(before);
    },
  );

  it.each(legacyDescriptions)(
    "preserves a customized historical description for %s",
    (name, description, chinese) => {
      const custom = `${description} Review only changes requested by our team.`;
      const presentation = getSkillPresentation({ ...builtin(name), description: custom }, zhT);
      expect(presentation.description).toBe(custom);
      expect(presentation.searchText).not.toContain(chinese.toLowerCase());
    },
  );

  it.each(["en", "zh-Hans"] as const)(
    "supports bilingual search when only the %s locale is mounted",
    (locale) => {
      const singleLocale = createI18n(locale, {
        [locale]: { skills: locale === "en" ? en : zh },
      });
      const presentation = getSkillPresentation(
        builtin("multica-code-review"),
        singleLocale.getFixedT(locale, "skills"),
      );
      expect(presentation.searchText).toContain("代码审查");
      expect(presentation.searchText).toContain("兼容性");
      expect(presentation.searchText).toContain("multica-code-review");
      expect(presentation.description).toBe(
        (locale === "en" ? en : zh).builtin_role_skills["multica-code-review"].description,
      );
    },
  );

  it.each(names)("localizes %s and keeps both languages searchable", (name, chinese) => {
    const skill = builtin(name);
    const before = structuredClone(skill);
    const translated = getSkillPresentation(skill, zhT);
    const english = getSkillPresentation(skill, enT);

    expect(translated.name).toBe(chinese);
    expect(translated.description).toBe(zh.builtin_role_skills[name].description);
    expect(translated.isBuiltin).toBe(true);
    expect(english.name).toBe(name);
    expect(english.description).toBe(en.builtin_role_skills[name].description);
    for (const presentation of [translated, english]) {
      expect(presentation.searchText).toContain(chinese.toLowerCase());
      expect(presentation.searchText).toContain(name);
      expect(presentation.searchText).toContain(skill.description.toLowerCase());
      expect(presentation.searchText).toContain(translated.description.toLowerCase());
      expect(presentation.searchText).toContain(english.description.toLowerCase());
    }
    expect(skill).toEqual(before);
  });

  it.each(names)("keeps the default-description comparison current with %s", (name) => {
    const content = readFileSync(
      new URL(`../../../../server/internal/service/builtin_role_skills/${name}/SKILL.md`, import.meta.url),
      "utf8",
    );
    expect(zh.builtin_role_skills[name].description).toBe(
      parseFrontmatter(content).frontmatter?.description,
    );
  });

  it.each(names)("still translates the previous English default for %s", (name) => {
    const skill = builtin(name, en.builtin_role_skills[name].description);
    const before = structuredClone(skill);
    expect(getSkillPresentation(skill, zhT).description).toBe(
      zh.builtin_role_skills[name].description,
    );
    expect(getSkillPresentation(skill, enT).description).toBe(skill.description);
    expect(skill).toEqual(before);
  });

  it.each([
    undefined,
    {},
    { origin: null },
    { origin: "builtin_role_skill" },
    { origin: Object.assign([], { type: "builtin_role_skill", name: "multica-code-review" }) },
    { origin: { type: "manual", name: "multica-code-review" } },
    { origin: { type: "builtin_role_skill", name: "multica-test-report" } },
  ])("does not translate an unverified workspace record (%j)", (config) => {
    const skill = { ...builtin("multica-code-review"), config };
    expect(getSkillPresentation(skill, zhT)).toMatchObject({
      name: skill.name,
      description: skill.description,
      isBuiltin: false,
    });
  });

  it.each(["Review only the billing migration.", "只审查计费迁移。"])(
    "preserves a user-renamed skill and a customized description (%s)",
    (description) => {
      const skill = builtin("multica-code-review");
      const renamed = { ...skill, name: "team-review" };
      expect(getSkillPresentation(renamed, zhT).name).toBe("team-review");

      const custom = { ...skill, description };
      const presentation = getSkillPresentation(custom, zhT);
      expect(presentation.name).toBe("代码审查");
      expect(presentation.description).toBe(custom.description);
      expect(presentation.searchText).toContain(description.toLowerCase());
      expect(presentation.searchText).not.toContain("兼容性");
      expect(presentation.searchText).not.toContain(
        en.builtin_role_skills["multica-code-review"].description.toLowerCase(),
      );
    },
  );

  // An operator-mounted template carries a name outside BUILTIN_ROLE_SKILL_NAMES
  // and ships no localized copy. getBuiltinRoleSkillPresentation must return
  // null for it so the "start from a template" panel falls back to the entry's
  // own raw description for both display and search. See the fallback in
  // template-skill-create-panel.tsx.
  it("returns null for a mounted template name so the panel uses its raw description", () => {
    const mountedName = "team-code-style";
    const description = "Our house style for reviews";
    expect(getBuiltinRoleSkillPresentation(mountedName, zhT, description)).toBeNull();

    // The panel's fallback shape: raw description, and a lowercased searchText
    // that matches on both the name and the description.
    const fallback = {
      name: mountedName,
      description,
      searchText: `${mountedName}\n${description}`.toLowerCase(),
    };
    expect(fallback.searchText).toContain("team-code-style");
    expect(fallback.searchText).toContain("house style");
  });

  it("treats unknown built-ins as ordinary skills", () => {
    const skill = {
      name: "multica-future-skill",
      description: "A future skill",
      config: { origin: { type: "builtin_role_skill", name: "multica-future-skill" } },
    };
    expect(getSkillPresentation(skill, zhT)).toMatchObject({
      name: skill.name,
      isBuiltin: false,
    });
    expect(getBuiltinRoleSkillPresentation(skill.name, zhT)).toBeNull();
  });

  it("can label trusted template skills before workspace materialization", () => {
    expect(getBuiltinRoleSkillPresentation("multica-code-review", zhT)?.name).toBe("代码审查");
  });

  it("supports ordinary English phrases as well as hyphenated identifiers", () => {
    expect(getSkillPresentation(builtin("multica-code-review"), zhT).searchText)
      .toContain("code review");
  });
});

describe("built-in role template summaries", () => {
  it.each([
    ["en", en], ["zh-Hans", zh],
  ] as const)("uses concise display copy for every default with only %s loaded", (locale, catalog) => {
    const instance = createI18n(locale, { [locale]: { skills: catalog } });
    const t = instance.getFixedT(locale, "skills");

    for (const [name] of names) {
      const summary = catalog.builtin_role_skills[name].summary;
      expect(summary.trim()).not.toBe("");
      expect(summary.length).toBeLessThan(catalog.builtin_role_skills[name].description.length);
      for (const description of [undefined, en.builtin_role_skills[name].description, ` ${zh.builtin_role_skills[name].description}\n`]) {
        expect(getBuiltinRoleSkillSummary(name, t, description)).toBe(summary);
      }
    }
  });

  it.each(legacyDescriptions)("summarizes only the recognized historical English default for %s", (name, description, chinese) => {
    expect(getBuiltinRoleSkillSummary(name, zhT, ` ${description}\n`))
      .toBe(zh.builtin_role_skills[name].summary);
    expect(getBuiltinRoleSkillSummary(name, zhT, chinese)).toBe(chinese);
  });

  it.each(names)("keeps a customized description for %s verbatim", (name) => {
    const custom = ` ${en.builtin_role_skills[name].description} Team-specific scope.\n`;
    expect(getBuiltinRoleSkillSummary(name, zhT, custom)).toBe(custom);
    expect(getBuiltinRoleSkillSummary(name, enT, "")).toBe("");
  });

  it("does not treat a known-name prefix or an unknown template as a built-in", () => {
    for (const name of ["multica-code-review-copy", "multica-future-skill", "team-review"]) {
      expect(getBuiltinRoleSkillSummary(name, zhT, "Use our review checklist.")).toBeNull();
    }
  });

  it("does not add summary copy to full descriptions or search text", () => {
    const name = "multica-code-review";
    const summary = getBuiltinRoleSkillSummary(name, enT);
    const presentation = getBuiltinRoleSkillPresentation(name, enT);

    expect(presentation?.description).toBe(en.builtin_role_skills[name].description);
    expect(summary).not.toBe(presentation?.description);
    expect(presentation?.searchText).not.toContain(summary?.toLowerCase());
    expect(presentation?.searchText).toContain(en.builtin_role_skills[name].description.toLowerCase());
    expect(presentation?.searchText).toContain(zh.builtin_role_skills[name].description.toLowerCase());
  });
});
