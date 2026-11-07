import type { TFunction } from "i18next";
import type { SkillSummary, SkillTemplate } from "@multica/core/types";
import {
  getBuiltinRoleSkillPresentation,
  getBuiltinRoleSkillSummary,
  type SkillPresentation,
} from "./skill-presentation";

export interface SkillTemplateDiscoveryItem {
  template: SkillTemplate;
  presentation: SkillPresentation;
  summary: string;
  source: "builtin" | "deployment";
}

export function getSkillTemplateDiscoveryItems(
  templates: readonly SkillTemplate[],
  t: TFunction<"skills">,
): SkillTemplateDiscoveryItem[] {
  return templates
    .filter((template) => typeof template.name === "string" && template.name.trim())
    .map((template) => {
      const presentation = getBuiltinRoleSkillPresentation(template.name, t, template.description) ?? {
        name: template.name,
        description: template.description,
        searchNames: [template.name.toLowerCase()],
        searchText: `${template.name}\n${template.description}`.toLowerCase(),
        isBuiltin: false,
      };

      return {
        template,
        presentation,
        summary: getBuiltinRoleSkillSummary(template.name, t, template.description) ?? presentation.description,
        source: presentation.isBuiltin ? "builtin" : "deployment",
      };
    });
}

export function getRelatedWorkspaceSkills(
  templateName: string,
  skills: readonly SkillSummary[],
  presentSkill: (skill: SkillSummary) => SkillPresentation,
): SkillSummary[] {
  return skills.filter((skill) => {
    if (skill.name === templateName && presentSkill(skill).isBuiltin) return true;

    const source = skill.config?.template_source;
    return source !== null && typeof source === "object" && !Array.isArray(source) &&
      "name" in source && typeof source.name === "string" && source.name === templateName;
  });
}
