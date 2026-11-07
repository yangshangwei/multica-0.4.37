import type { LucideIcon } from "lucide-react";
import { AVATAR_ICON_COMPONENTS } from "@multica/ui/lib/avatar-icon";
import {
  resolveSkillIconName,
  SKILL_ICON_NAMES,
  type SkillCategory,
  type SkillIconName,
  type SkillPresentationMeta,
} from "@multica/core/skills";

// Named imports on purpose: the whitelist in @multica/core is the contract,
// and a `Record<SkillIconName, ...>` makes the compiler refuse a whitelist
// entry that has no component here (or a stale component with no entry).
export const SKILL_ICON_COMPONENTS = Object.fromEntries(
  SKILL_ICON_NAMES.map((name) => [name, AVATAR_ICON_COMPONENTS[name]]),
) as Record<SkillIconName, LucideIcon>;

/**
 * Category tone classes. Colour always follows the category (never the icon
 * override) so every card in a category shares one hue. Tokens live in
 * `packages/ui/styles/tokens.css` as `--skill-<category>`.
 */
export const SKILL_CATEGORY_TONE: Record<
  SkillCategory,
  { text: string; bg: string; dot: string }
> = {
  research: { text: "text-skill-research", bg: "bg-skill-research/12", dot: "bg-skill-research" },
  design: { text: "text-skill-design", bg: "bg-skill-design/12", dot: "bg-skill-design" },
  engineering: {
    text: "text-skill-engineering",
    bg: "bg-skill-engineering/12",
    dot: "bg-skill-engineering",
  },
  quality: { text: "text-skill-quality", bg: "bg-skill-quality/12", dot: "bg-skill-quality" },
  operations: {
    text: "text-skill-operations",
    bg: "bg-skill-operations/12",
    dot: "bg-skill-operations",
  },
  writing: { text: "text-skill-writing", bg: "bg-skill-writing/12", dot: "bg-skill-writing" },
  data: { text: "text-skill-data", bg: "bg-skill-data/12", dot: "bg-skill-data" },
  other: { text: "text-skill-other", bg: "bg-skill-other/12", dot: "bg-skill-other" },
};

export function resolveSkillIcon(meta: SkillPresentationMeta): LucideIcon {
  return SKILL_ICON_COMPONENTS[resolveSkillIconName(meta)];
}
