import type { SkillCategory } from "@multica/core/skills";
import { useT } from "../../i18n";

/** Localized display name per category, for sidebars, chips and cells. */
export function useSkillCategoryLabels(): Record<SkillCategory, string> {
  const { t } = useT("skills");
  return {
    research: t(($) => $.categories.research),
    writing: t(($) => $.categories.writing),
    engineering: t(($) => $.categories.engineering),
    operations: t(($) => $.categories.operations),
    data: t(($) => $.categories.data),
    other: t(($) => $.categories.other),
  };
}
