"use client";

import type { SkillPresentationMeta } from "@multica/core/skills";
import { cn } from "@multica/ui/lib/utils";
import { SKILL_CATEGORY_TONE, resolveSkillIcon } from "../lib/skill-presentation-icon";

const SIZES = {
  sm: { box: "size-6 rounded-md", icon: "size-3.5" },
  md: { box: "size-10 rounded-lg", icon: "size-5" },
  lg: { box: "size-12 rounded-xl", icon: "size-6" },
} as const;

/**
 * A skill's presentation icon on its category-toned tile. Shape comes from
 * the icon override (or the category default); colour always from the
 * category. Decorative — callers put the accessible name on the text next
 * to it.
 */
export function SkillPresentationIcon({
  meta,
  size = "md",
  className,
}: {
  meta: SkillPresentationMeta;
  size?: keyof typeof SIZES;
  className?: string;
}) {
  const Icon = resolveSkillIcon(meta);
  const tone = SKILL_CATEGORY_TONE[meta.category];
  const s = SIZES[size];
  return (
    <span
      aria-hidden
      data-category={meta.category}
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        s.box,
        tone.bg,
        tone.text,
        className,
      )}
    >
      <Icon className={s.icon} />
    </span>
  );
}
