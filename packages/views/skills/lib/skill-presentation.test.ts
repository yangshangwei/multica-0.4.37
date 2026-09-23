// @vitest-environment node

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { createI18n } from "@multica/core/i18n/react";
import { parseFrontmatter } from "@multica/core/skills/frontmatter";
import en from "../../locales/en/skills.json";
import zh from "../../locales/zh-Hans/skills.json";
import ja from "../../locales/ja/skills.json";
import ko from "../../locales/ko/skills.json";
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
  ["multica-debugging", "根因分析"],
  ["multica-requirement-clarification", "需求澄清"],
  ["multica-test-report", "测试报告"],
  ["multica-progress-report", "进展报告"],
  ["multica-reliability-engineering", "可靠性工程"],
  ["multica-agent-evaluation", "智能体评测"],
  ["multica-incident-learning", "事故学习"],
  ["multica-rollout-and-canary-verification", "发布后与 Canary 验证"],
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
  it("recognizes debugging as a platform built-in before materialization", () => {
    expect(getBuiltinRoleSkillPresentation("multica-debugging", zhT)).toMatchObject({
      name: "根因分析",
      isBuiltin: true,
    });
  });

  it.each([
    ["en", en, "multica-debugging"],
    ["zh-Hans", zh, "根因分析"],
    ["ja", ja, "根本原因分析"],
    ["ko", ko, "근본 원인 분석"],
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
  // and ships no four-language copy. getBuiltinRoleSkillPresentation must return
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
