import { readSkillPresentationMeta, SKILL_CATEGORIES, type SkillCategory } from "@multica/core/skills";
import type { SkillMarketSource } from "@multica/core/skills/stores";
import type { SkillTemplateDiscoveryItem } from "./skill-template-discovery";

interface SkillMarketFilters {
  source: SkillMarketSource;
  category: SkillCategory | null;
  search: string;
}

export function filterSkillMarketItems(items: readonly SkillTemplateDiscoveryItem[], filters: SkillMarketFilters): SkillTemplateDiscoveryItem[] {
  const search = filters.search.trim().toLowerCase();
  return items.filter((item) =>
    (filters.source === "all" || item.source === filters.source) &&
    (filters.category === null || readSkillPresentationMeta({ presentation: item.template }).category === filters.category) &&
    (!search || item.presentation.searchText.includes(search)),
  );
}

export function getSkillMarketCategories(items: readonly SkillTemplateDiscoveryItem[], source: SkillMarketFilters["source"]): SkillCategory[] {
  const available = new Set(items
    .filter((item) => source === "all" || item.source === source)
    .map((item) => readSkillPresentationMeta({ presentation: item.template }).category));
  return SKILL_CATEGORIES.filter((category) => available.has(category));
}
