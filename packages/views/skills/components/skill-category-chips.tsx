"use client";

import { SKILL_CATEGORIES } from "@multica/core/skills";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { SkillPresentationIcon } from "./skill-presentation-icon";
import { useSkillCategoryLabels } from "../hooks/use-skill-category-labels";
import type { SkillCategoryNavProps } from "./skill-category-sidebar";

const CHIP_CLASS =
  "flex h-7 shrink-0 snap-start items-center gap-1.5 rounded-full border px-2.5 text-caption text-muted-foreground transition-colors hover:bg-accent hover:text-foreground data-active:border-foreground/40 data-active:font-medium data-active:text-foreground";

/**
 * Narrow-container (< @2xl) counterpart of the sidebar: the same categories
 * as one horizontally scrollable row of chips under the toolbar. Sources are
 * left to the Filter dropdown here — the row is already the widest thing on
 * a phone.
 */
export function SkillCategoryChips({
  facets,
  filters,
  onSelectCategory,
  className,
}: Omit<SkillCategoryNavProps, "onToggleOrigin">) {
  const { t } = useT("skills");
  const labels = useSkillCategoryLabels();
  const first = filters.categories[0];
  const active = filters.categories.length === 1 && first ? first : null;

  return (
    <div
      role="group"
      aria-label={t(($) => $.categories.section_title)}
      className={cn(
        "flex shrink-0 snap-x gap-1.5 overflow-x-auto px-6 pb-2 [scrollbar-width:none]",
        className,
      )}
    >
      <button
        type="button"
        className={CHIP_CLASS}
        data-active={active === null ? "" : undefined}
        aria-pressed={active === null}
        onClick={() => onSelectCategory(null)}
      >
        {t(($) => $.categories.all)}
        <span className="tabular-nums">{facets.total}</span>
      </button>
      {SKILL_CATEGORIES.map((category) => (
        <button
          key={category}
          type="button"
          className={CHIP_CLASS}
          data-active={active === category ? "" : undefined}
          aria-pressed={active === category}
          onClick={() => onSelectCategory(category)}
        >
          <SkillPresentationIcon
            meta={{ category, icon: null }}
            size="sm"
            className="size-4 rounded"
          />
          {labels[category]}
          <span className="tabular-nums">{facets.categoryCounts[category]}</span>
        </button>
      ))}
    </div>
  );
}
