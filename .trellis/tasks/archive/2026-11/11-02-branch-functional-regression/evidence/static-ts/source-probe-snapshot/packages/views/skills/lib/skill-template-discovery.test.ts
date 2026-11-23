// @vitest-environment node

import { describe, expect, it } from "vitest";
import { createI18n } from "@multica/core/i18n/react";
import type { SkillSummary, SkillTemplate } from "@multica/core/types";
import en from "../../locales/en/skills.json";
import zh from "../../locales/zh-Hans/skills.json";
import { getSkillPresentation } from "./skill-presentation";
import { getRelatedWorkspaceSkills, getSkillTemplateDiscoveryItems } from "./skill-template-discovery";

const i18n = createI18n("en", { en: { skills: en } });
const t = i18n.getFixedT("en", "skills");
const templateName = "multica-code-review";
const presentSkill = (skill: SkillSummary) => getSkillPresentation(skill, t);

function template(name: string, description = "Team template instructions."): SkillTemplate {
  return { name, description, version: 1, content: "# Instructions", files: [{ path: "notes.md", content: "Keep this file." }] };
}

function skill(id: string, name = templateName, config: SkillSummary["config"] = {}): SkillSummary {
  return {
    id, name, config, workspace_id: "workspace-id", description: zh.builtin_role_skills[templateName].description,
    created_by: null, created_at: "2026-09-26T00:00:00Z", updated_at: "2026-09-26T00:00:00Z",
  };
}

describe("skill template discovery", () => {
  it.each([
    { names: [], builtin: 0, deployment: 0 },
    { names: ["team-review", "team-release"], builtin: 0, deployment: 2 },
    { names: ["multica-code-review", "multica-debugging"], builtin: 2, deployment: 0 },
    { names: ["team-review", "multica-code-review", "team-release"], builtin: 1, deployment: 2 },
  ])("derives honest source counts for $names", ({ names, builtin, deployment }) => {
    const templates = names.map((name) => template(name));
    const items = getSkillTemplateDiscoveryItems(templates, t);

    expect(items.filter((item) => item.source === "builtin")).toHaveLength(builtin);
    expect(items.filter((item) => item.source === "deployment")).toHaveLength(deployment);
    expect(items).toHaveLength(builtin + deployment);
    expect(items.map((item) => item.template)).toEqual(templates);
  });

  it("keeps summaries separate from full presentation and original template data", () => {
    const builtin = template(templateName, zh.builtin_role_skills[templateName].description);
    const deployment = template("team-review", "Custom review instructions.");
    const templates = [deployment, builtin];
    const before = structuredClone(templates);
    const [mountedItem, builtinItem] = getSkillTemplateDiscoveryItems(templates, t);

    expect(mountedItem).toMatchObject({
      template: deployment, source: "deployment", summary: deployment.description,
      presentation: { name: deployment.name, description: deployment.description, isBuiltin: false, searchNames: [deployment.name] },
    });
    expect(mountedItem?.presentation.searchText).toContain(deployment.description.toLowerCase());
    expect(builtinItem).toMatchObject({
      source: "builtin", summary: en.builtin_role_skills[templateName].summary,
      presentation: { description: en.builtin_role_skills[templateName].description, isBuiltin: true },
    });
    expect(mountedItem?.template).toBe(deployment);
    expect(builtinItem?.template).toBe(builtin);
    expect(templates).toEqual(before);
  });

  it("ignores empty, whitespace-only and malformed names without changing valid names", () => {
    const templates = ["", " \n", undefined, null, 42, []].map((name) => template(name as string));
    const valid = template("team-review");
    expect(getSkillTemplateDiscoveryItems([...templates, valid], t).map((item) => item.template))
      .toEqual([valid]);
  });
});

describe("related workspace skills", () => {
  it("returns every official instance and exact template copy once in query order", () => {
    const officialOrigin = { type: "builtin_role_skill", name: templateName };
    const renamed = skill("renamed", "team-review", { template_source: { name: templateName, version: 99 } });
    const both = skill("both", templateName, { origin: officialOrigin, template_source: { name: templateName, version: 1 } });
    const official = skill("official", templateName, { origin: officialOrigin });
    const unrelated = skill("manual");
    const skills = [renamed, unrelated, both, official];
    const before = structuredClone(skills);
    const related = getRelatedWorkspaceSkills(templateName, skills, presentSkill);

    expect(related).toEqual([renamed, both, official]);
    related.forEach((item, index) => expect(item).toBe([renamed, both, official][index]));
    expect(skills).toEqual(before);
  });

  it.each([
    undefined, null, "multica-code-review", [], { name: 1 }, {},
    { name: "multica-code-review-copy" }, { name: " multica-code-review" },
    Object.assign([], { name: templateName }),
  ])("rejects malformed or nonmatching template_source (%j)", (source) => {
    expect(getRelatedWorkspaceSkills(templateName, [skill("invalid", "team-review", { template_source: source })], presentSkill))
      .toEqual([]);
  });

  it.each([
    undefined, null, "builtin_role_skill", [],
    { type: "manual", name: templateName },
    { type: "builtin_role_skill", name: "multica-debugging" },
    Object.assign([], { type: "builtin_role_skill", name: templateName }),
  ])("excludes same-name skills without verified official provenance (%j)", (origin) => {
    expect(getRelatedWorkspaceSkills(templateName, [skill("unverified", templateName, { origin })], presentSkill)).toEqual([]);
  });

  it("requires matching canonical identity even with official origin", () => {
    const renamed = skill("renamed", "team-review", { origin: { type: "builtin_role_skill", name: templateName } });
    const unknown = skill("unknown", "multica-future-skill", { origin: { type: "builtin_role_skill", name: "multica-future-skill" } });
    expect(getRelatedWorkspaceSkills(templateName, [renamed], presentSkill)).toEqual([]);
    expect(getRelatedWorkspaceSkills(unknown.name, [unknown], presentSkill)).toEqual([]);
  });

  it("keeps valid official provenance independent of malformed source metadata", () => {
    const official = skill("official", templateName, { origin: { type: "builtin_role_skill", name: templateName }, template_source: [] });
    expect(getRelatedWorkspaceSkills(templateName, [official], presentSkill)).toEqual([official]);
  });

  it("relates deployment copies by exact source name without a built-in label", () => {
    const copy = skill("copy", "local-guide", { template_source: { name: "team-guide" } });
    expect(getRelatedWorkspaceSkills("team-guide", [copy], presentSkill)).toEqual([copy]);
    expect(getRelatedWorkspaceSkills("missing-template", [copy], presentSkill)).toEqual([]);
  });
});
