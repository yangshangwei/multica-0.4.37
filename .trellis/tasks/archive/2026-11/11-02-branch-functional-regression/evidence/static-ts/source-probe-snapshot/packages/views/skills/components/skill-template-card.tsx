"use client";

import { ArrowRight, Check, ChevronDown } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { readSkillPresentationMeta } from "@multica/core/skills";
import type { SkillSummary } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@multica/ui/components/ui/dropdown-menu";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { useSkillCategoryLabels } from "../hooks/use-skill-category-labels";
import { useSkillPresentation } from "../hooks/use-skill-presentation";
import { getRelatedWorkspaceSkills, type SkillTemplateDiscoveryItem } from "../lib/skill-template-discovery";
import { SkillPresentationIcon } from "./skill-presentation-icon";

export function SkillTemplateCard({ item, skills, skillsError, onPreview }: {
  item: SkillTemplateDiscoveryItem;
  skills: readonly SkillSummary[] | undefined;
  skillsError: boolean;
  onPreview: (templateName: string, trigger: HTMLButtonElement) => void;
}) {
  const { t } = useT("skills");
  const paths = useWorkspacePaths();
  const categoryLabels = useSkillCategoryLabels();
  const presentSkill = useSkillPresentation();
  const meta = readSkillPresentationMeta({ presentation: item.template });
  const related = getRelatedWorkspaceSkills(item.template.name, skills ?? [], presentSkill);
  const firstRelated = related[0];
  const hasWorkspaceData = skills !== undefined && !skillsError;

  return (
    <article className="flex h-full min-w-0 flex-col rounded-lg border bg-card">
      <button
        type="button"
        aria-label={t(($) => $.market.preview_label, { name: item.presentation.name })}
        onClick={(event) => onPreview(item.template.name, event.currentTarget)}
        className="flex min-w-0 flex-1 flex-col gap-3 rounded-lg p-4 text-left transition-colors hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
      >
        <span className="flex min-w-0 items-start gap-3">
          <SkillPresentationIcon meta={meta} />
          <span className="line-clamp-2 min-w-0 break-words text-title-sm font-medium" title={item.presentation.name}>
            {item.presentation.name}
          </span>
        </span>
        <span className="line-clamp-2 min-h-[2lh] break-words text-body text-muted-foreground" title={item.presentation.description}>
          {item.summary}
        </span>
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
          <span>{categoryLabels[meta.category]}</span>
          <span aria-hidden>·</span>
          <span>{item.source === "deployment" ? t(($) => $.market.source_deployment) : t(($) => $.market.source_builtin)}</span>
        </span>
        <span className="mt-auto flex items-center gap-1.5 pt-1 text-body font-medium">
          {t(($) => $.market.preview)}
          <ArrowRight aria-hidden className="size-3.5" />
        </span>
      </button>
      <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 px-4 pb-3 text-caption">
        {!hasWorkspaceData ? (
          <span className="text-muted-foreground">
            {skillsError ? t(($) => $.market.workspace_unknown) : t(($) => $.create.template.related_loading)}
          </span>
        ) : !firstRelated ? (
          <span className="text-muted-foreground">{t(($) => $.market.related_none)}</span>
        ) : (
          <>
            <span className="flex min-w-0 items-center gap-1 text-success">
              <Check aria-hidden className="size-3.5 shrink-0" />
              {t(($) => $.market.related_count, { count: related.length })}
            </span>
            {related.length === 1 ? (
              <AppLink
                href={paths.skillDetail(firstRelated.id)}
                newTabTitle={presentSkill(firstRelated).name}
                title={presentSkill(firstRelated).name}
                className="ml-auto rounded-sm py-1 font-medium underline underline-offset-4 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {t(($) => $.market.open_created)}
              </AppLink>
            ) : (
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button variant="ghost" size="sm" className="ml-auto h-7 px-1.5 text-caption" />}>
                  {t(($) => $.market.view_related)}
                  <ChevronDown aria-hidden className="size-3" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="max-w-[min(24rem,var(--available-width))]">
                  {related.map((skill) => {
                    const presentation = presentSkill(skill);
                    return (
                      <DropdownMenuItem
                        key={skill.id}
                        render={<AppLink href={paths.skillDetail(skill.id)} newTabTitle={presentation.name} />}
                        className="break-words"
                      >
                        {presentation.name}
                      </DropdownMenuItem>
                    );
                  })}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </>
        )}
      </div>
    </article>
  );
}
