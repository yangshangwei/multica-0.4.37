// @vitest-environment node

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { createI18n } from "@multica/core/i18n/react";
import { parseFrontmatter } from "@multica/core/skills/frontmatter";
import en from "../../locales/en/skills.json";
import zh from "../../locales/zh-Hans/skills.json";
import {
  getBuiltinRoleSkillPresentation,
  getSkillPresentation,
  type SkillPresentationInput,
} from "./skill-presentation";

const i18n = createI18n("zh-Hans", {
  en: { skills: en },
  "zh-Hans": { skills: zh },
});
const zhT = i18n.getFixedT("zh-Hans", "skills");
const enT = i18n.getFixedT("en", "skills");

const names = [
  ["multica-release-check", "发布检查"],
  ["multica-documentation-change", "文档更新"],
  ["multica-security-review", "安全审查"],
  ["multica-architecture-decision-record", "架构决策记录"],
  ["multica-code-review", "代码审查"],
  ["multica-requirement-clarification", "需求澄清"],
  ["multica-test-report", "测试报告"],
] as const;

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

function builtin(name: keyof typeof en.builtin_role_skills): SkillPresentationInput {
  return {
    name,
    description: en.builtin_role_skills[name].description,
    config: { origin: { type: "builtin_role_skill", name, version: 1 } },
  };
}

describe("built-in role skill presentation", () => {
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
    expect(english.description).toBe(skill.description);
    for (const presentation of [translated, english]) {
      expect(presentation.searchText).toContain(chinese);
      expect(presentation.searchText).toContain(name);
      expect(presentation.searchText).toContain(skill.description.toLowerCase());
      expect(presentation.searchText).toContain(translated.description.toLowerCase());
    }
    expect(skill).toEqual(before);
  });

  it.each(names)("keeps the default-description comparison current with %s", (name) => {
    const content = readFileSync(
      new URL(`../../../../server/internal/service/builtin_role_skills/${name}/SKILL.md`, import.meta.url),
      "utf8",
    );
    expect(en.builtin_role_skills[name].description).toBe(
      parseFrontmatter(content).frontmatter?.description,
    );
  });

  it.each([
    undefined,
    {},
    { origin: null },
    { origin: "builtin_role_skill" },
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

  it("preserves a user-renamed skill and a customized description", () => {
    const skill = builtin("multica-code-review");
    const renamed = { ...skill, name: "team-review" };
    expect(getSkillPresentation(renamed, zhT).name).toBe("team-review");

    const custom = { ...skill, description: "Review only the billing migration." };
    const presentation = getSkillPresentation(custom, zhT);
    expect(presentation.name).toBe("代码审查");
    expect(presentation.description).toBe(custom.description);
    expect(presentation.searchText).toContain("billing migration");
    expect(presentation.searchText).not.toContain("兼容性");
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
