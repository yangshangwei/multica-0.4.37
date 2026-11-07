import type { SkillCategory } from "@multica/core/skills";
import { useT } from "../../i18n";

/** Localized display name per category, for sidebars, chips and cells. */
export function useSkillCategoryLabels(): Record<SkillCategory, string> {
  const { t } = useT("skills");
  return {
    research: t(($) => $.categories.research),
    design: t(($) => $.categories.design),
    engineering: t(($) => $.categories.engineering),
    quality: t(($) => $.categories.quality),
    operations: t(($) => $.categories.operations),
    writing: t(($) => $.categories.writing),
    data: t(($) => $.categories.data),
    other: t(($) => $.categories.other),
  };
}
