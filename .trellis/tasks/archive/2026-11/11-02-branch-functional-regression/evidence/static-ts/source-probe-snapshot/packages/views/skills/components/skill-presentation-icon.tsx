"use client";

import type { SkillPresentationMeta } from "@multica/core/skills";
import { IconTile, type IconTileSize } from "../../common/icon-tile";
import { SKILL_CATEGORY_TONE, resolveSkillIcon } from "../lib/skill-presentation-icon";

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
  size?: IconTileSize;
  className?: string;
}) {
  const tone = SKILL_CATEGORY_TONE[meta.category];
  return (
    <IconTile
      icon={resolveSkillIcon(meta)}
      tone={`${tone.bg} ${tone.text}`}
      size={size}
      data-category={meta.category}
      className={className}
    />
  );
}
