import type { TFunction } from "i18next";
import type { SkillSummary } from "@multica/core/types";
import enSkills from "../../locales/en/skills.json";
import zhSkills from "../../locales/zh-Hans/skills.json";

const BUILTIN_ROLE_SKILL_NAMES = [
  "multica-release-check",
  "multica-documentation-change",
  "multica-security-review",
  "multica-architecture-decision-record",
  "multica-code-review",
  "multica-requirement-clarification",
  "multica-test-report",
] as const;

export type SkillPresentationInput = Pick<SkillSummary, "name" | "description"> &
  Partial<Pick<SkillSummary, "config">>;

export interface SkillPresentation {
  name: string;
  description: string;
  searchNames: string[];
  searchText: string;
  isBuiltin: boolean;
}

/** Only for names supplied by the built-in role-template catalog. */
export function getBuiltinRoleSkillPresentation(
  name: string,
  t: TFunction<"skills">,
  storedDescription?: string,
): SkillPresentation | null {
  const key = BUILTIN_ROLE_SKILL_NAMES.find((candidate) => candidate === name);
  if (!key) return null;

  // Production mounts only the current locale. Read secondary-language
  // search copy from the catalogs instead of asking i18next for an unloaded locale.
  const sourceDescription = enSkills.builtin_role_skills[key].description;
  const chinese = zhSkills.builtin_role_skills[key];
  const description = storedDescription ?? sourceDescription;
  const hasDefaultDescription = description.trim() === sourceDescription.trim();
  const translatedDescription = hasDefaultDescription
    ? t(($) => $.builtin_role_skills[key].description)
    : description;
  const searchNames = [name, name.replace(/[-_]+/g, " "), chinese.name]
    .map((value) => value.toLowerCase());

  return {
    name: t(($) => $.builtin_role_skills[key].name),
    description: translatedDescription,
    searchNames,
    searchText: [
      ...searchNames,
      description,
      ...(hasDefaultDescription ? [chinese.description] : []),
    ].join("\n").toLowerCase(),
    isBuiltin: true,
  };
}

/** Workspace records need provenance; an English name alone is not enough. */
export function getSkillPresentation(
  skill: SkillPresentationInput,
  t: TFunction<"skills">,
): SkillPresentation {
  const origin = skill.config?.origin;
  if (
    origin &&
    typeof origin === "object" &&
    "type" in origin &&
    origin.type === "builtin_role_skill" &&
    "name" in origin &&
    origin.name === skill.name
  ) {
    const builtin = getBuiltinRoleSkillPresentation(skill.name, t, skill.description);
    if (builtin) return builtin;
  }

  return {
    name: skill.name,
    description: skill.description,
    searchNames: [skill.name.toLowerCase()],
    searchText: [skill.name, skill.description].join("\n").toLowerCase(),
    isBuiltin: false,
  };
}
