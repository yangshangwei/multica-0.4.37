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
  "multica-debugging",
  "multica-requirement-clarification",
  "multica-test-report",
  "multica-progress-report",
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
  const english = enSkills.builtin_role_skills[key];
  const chinese = zhSkills.builtin_role_skills[key];
  const description = storedDescription ?? chinese.description;
  const hasDefaultDescription = [chinese.description, english.description]
    .some((value) => description.trim() === value.trim());
  let translatedDescription = hasDefaultDescription
    ? t(($) => $.builtin_role_skills[key].description)
    : description;
  let searchDescriptions = hasDefaultDescription
    ? [chinese.description, english.description]
    : [];

  // Existing workspace copies retain shipped defaults when templates change.
  // Recognize the exact old text so customized descriptions still stay untouched.
  if (
    (key === "multica-release-check" || key === "multica-architecture-decision-record") &&
    description.trim() === enSkills.builtin_role_skills[key].description_v1.trim()
  ) {
    translatedDescription = t(($) => $.builtin_role_skills[key].description_v1);
    searchDescriptions = [
      zhSkills.builtin_role_skills[key].description_v1,
      enSkills.builtin_role_skills[key].description_v1,
    ];
  }
  const translatedName = t(($) => $.builtin_role_skills[key].name);
  const searchNames = [name, name.replace(/[-_]+/g, " "), chinese.name, translatedName]
    .map((value) => value.toLowerCase());

  return {
    name: translatedName,
    description: translatedDescription,
    searchNames,
    searchText: [
      ...searchNames,
      description,
      translatedDescription,
      ...searchDescriptions,
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
