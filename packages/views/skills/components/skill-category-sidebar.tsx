"use client";

import type { SkillCategory } from "@multica/core/skills";
import { SKILL_CATEGORIES } from "@multica/core/skills";
import type { SkillListFilters, SkillOriginType } from "@multica/core/skills/stores";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import type { SkillListFacets } from "../hooks/use-skill-list-facets";
import { useSkillCategoryLabels } from "../hooks/use-skill-category-labels";
import { SkillPresentationIcon } from "./skill-presentation-icon";
import { ORIGIN_TYPES, originIcon, useOriginLabels } from "./skill-list-toolbar";

export interface SkillCategoryNavProps {
  facets: SkillListFacets;
  filters: SkillListFilters;
  /** Sidebar semantics: single-select, `null` = "All". */
  onSelectCategory: (category: SkillCategory | null) => void;
  onToggleOrigin: (origin: SkillOriginType) => void;
  className?: string;
}

// Active state lives on weight + text colour (not on the background hover
// touches), so a selected row stays identifiable while hovered.
const ITEM_CLASS =
  "group/nav flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-body text-muted-foreground transition-colors hover:bg-accent hover:text-foreground data-active:font-medium data-active:text-foreground data-active:bg-accent/60";

function activeCategory(filters: SkillListFilters): SkillCategory | null {
  const first = filters.categories[0];
  return filters.categories.length === 1 && first ? first : null;
}

/**
 * Wide-container (≥ @2xl) category navigation: "All" + every
 * `SKILL_CATEGORIES` entry with counts, then a secondary "Sources" group
 * that toggles the same `filters.origins` dimension the toolbar's Filter
 * dropdown edits.
 */
export function SkillCategorySidebar({
  facets,
  filters,
  onSelectCategory,
  onToggleOrigin,
  className,
}: SkillCategoryNavProps) {
  const { t } = useT("skills");
  const labels = useSkillCategoryLabels();
  const originLabels = useOriginLabels();
  const active = activeCategory(filters);
  const sources = ORIGIN_TYPES.filter((type) => (facets.originCounts.get(type) ?? 0) > 0);

  return (
    <nav
      aria-label={t(($) => $.categories.section_title)}
      className={cn(
        "w-52 shrink-0 overflow-y-auto border-r px-3 py-3",
        className,
      )}
    >
      <ul className="space-y-0.5">
        <li>
          <button
            type="button"
            className={ITEM_CLASS}
            data-active={active === null ? "" : undefined}
            aria-current={active === null ? "true" : undefined}
            onClick={() => onSelectCategory(null)}
          >
            <span className="min-w-0 flex-1 truncate">
              {t(($) => $.categories.all)}
            </span>
            <span className="text-caption tabular-nums text-muted-foreground">
              {facets.total}
            </span>
          </button>
        </li>
        {SKILL_CATEGORIES.map((category) => (
          <li key={category}>
            <button
              type="button"
              className={ITEM_CLASS}
              data-active={active === category ? "" : undefined}
              aria-current={active === category ? "true" : undefined}
              onClick={() => onSelectCategory(category)}
            >
              <SkillPresentationIcon
                meta={{ category, icon: null }}
                size="sm"
              />
              <span className="min-w-0 flex-1 truncate">{labels[category]}</span>
              <span className="text-caption tabular-nums text-muted-foreground">
                {facets.categoryCounts[category]}
              </span>
            </button>
          </li>
        ))}
      </ul>

      {sources.length > 0 && (
        <div className="mt-4">
          <div className="px-2 pb-1 text-caption font-medium text-muted-foreground">
            {t(($) => $.categories.section_sources)}
          </div>
          <ul className="space-y-0.5">
            {sources.map((type) => {
              const checked = filters.origins.includes(type);
              return (
                <li key={type}>
                  <button
                    type="button"
                    className={ITEM_CLASS}
                    data-active={checked ? "" : undefined}
                    aria-pressed={checked}
                    onClick={() => onToggleOrigin(type)}
                  >
                    {originIcon(type)}
                    <span className="min-w-0 flex-1 truncate">
                      {originLabels[type]}
                    </span>
                    <span className="text-caption tabular-nums text-muted-foreground">
                      {facets.originCounts.get(type) ?? 0}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </nav>
  );
}
