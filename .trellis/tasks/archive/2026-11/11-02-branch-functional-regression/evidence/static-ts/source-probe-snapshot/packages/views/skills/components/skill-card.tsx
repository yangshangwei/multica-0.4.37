"use client";

import { Lock } from "lucide-react";
import { resolvePublicFileUrl } from "@multica/core/workspace/avatar-url";
import { Badge } from "@multica/ui/components/ui/badge";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink, rowLinkInteractiveProps } from "../../navigation";
import { getBuiltinRoleSkillSummary } from "../lib/skill-presentation";
import { LabelChip } from "../../labels/label-chip";
import { SkillPresentationIcon } from "./skill-presentation-icon";
import { SkillRowActions, type SkillActionsContext } from "./skill-list-actions";
import { useOriginLabels } from "./skill-list-toolbar";
import type { PresentedSkillRow } from "./skills-page";

/** Fixed card height — the grid virtualizes by row on this contract. */
export const SKILL_CARD_HEIGHT = 144;
export const SKILL_CARD_WITH_LABELS_HEIGHT = 168;

const MAX_LABELS = 3;
const MAX_AVATARS = 3;

/**
 * One skill in the card grid. The whole card is a row link (the caller
 * spreads `useRowLink(...)` on it); the checkbox and the kebab stop
 * propagation themselves so they never trigger navigation.
 */
export function SkillCard({
  row,
  ctx,
  selected,
  onToggleSelected,
  linkProps,
  href,
  height = row.labels.length > 0 ? SKILL_CARD_WITH_LABELS_HEIGHT : SKILL_CARD_HEIGHT,
}: {
  row: PresentedSkillRow;
  ctx: SkillActionsContext;
  selected: boolean;
  onToggleSelected: () => void;
  linkProps: React.HTMLAttributes<HTMLDivElement>;
  href: string;
  height?: number;
}) {
  const { t } = useT("skills");
  const { skill, presentation, meta, labels, agents, canEdit, originType } = row;
  const originLabels = useOriginLabels();
  const description = presentation.isBuiltin
    ? getBuiltinRoleSkillSummary(skill.name, t, skill.description) ?? presentation.description
    : presentation.description;
  const visibleLabels = labels.slice(0, MAX_LABELS);
  const extraLabels = labels.length - visibleLabels.length;
  const visibleAgents = agents.slice(0, MAX_AVATARS);
  const extraAgents = agents.length - visibleAgents.length;

  return (
    <div
      {...linkProps}
      data-selected={selected ? "" : undefined}
      data-testid="skill-card"
      style={{ height }}
      className={cn(
        // `group/row` on purpose: SkillRowActions reveals its kebab on
        // `group-hover/row`, so the card shares the list row's hover contract.
        "group/row relative flex cursor-pointer flex-col gap-2 rounded-lg border bg-card p-3 transition-colors hover:bg-accent/40",
        selected && "border-primary/50 bg-accent/30",
      )}
    >
      <div className="flex items-start gap-2.5">
        <SkillPresentationIcon meta={meta} size="md" />
        <div className="min-w-0 flex-1 pt-0.5">
          <div className="flex items-center gap-1.5">
            <Tooltip>
              <TooltipTrigger render={
                <AppLink
                  href={href}
                  newTabTitle={presentation.name}
                  {...rowLinkInteractiveProps}
                  className="line-clamp-2 min-w-0 rounded-sm text-body font-medium break-words hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground"
                  title={presentation.name}
                >
                  {presentation.name}
                </AppLink>
              } />
              <TooltipContent>{presentation.name}</TooltipContent>
            </Tooltip>
            {!canEdit && (
              <Tooltip>
                <TooltipTrigger
                  render={<Lock className="size-3 shrink-0 text-faint-foreground" />}
                />
                <TooltipContent>{t(($) => $.table.lock_tooltip)}</TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
        <div {...rowLinkInteractiveProps} className="flex h-7 shrink-0 items-center gap-3.5 pointer-coarse:h-11">
          <Checkbox
            checked={selected}
            aria-label={t(($) => $.table.select_skill, { name: presentation.name })}
            onCheckedChange={onToggleSelected}
            className="border-faint-foreground after:-inset-2 pointer-coarse:after:-inset-3.5 focus-visible:border-foreground focus-visible:ring-foreground/50"
          />
          <SkillRowActions row={row} ctx={ctx} />
        </div>
      </div>

      <p
        className="line-clamp-2 shrink-0 text-caption text-muted-foreground break-words"
        title={presentation.description}
      >
        {description}
      </p>

      {visibleLabels.length > 0 && <div className="flex min-h-5 shrink-0 items-center gap-1 overflow-hidden">
        {visibleLabels.map((label) => (
          <LabelChip key={label.id} label={label} className="max-w-24" />
        ))}
        {extraLabels > 0 && (
          <Badge variant="outline">
            {t(($) => $.table.labels_more, { count: extraLabels })}
          </Badge>
        )}
      </div>}

      <div className="mt-auto flex min-w-0 shrink-0 items-center gap-2">
        {agents.length > 0 && (
          <div className="flex items-center -space-x-1.5">
            {visibleAgents.map((a) => (
              <span key={a.id} className="inline-flex rounded-full ring-2 ring-card">
                <ActorAvatar
                  name={a.name}
                  initials={a.name.slice(0, 2).toUpperCase()}
                  avatarUrl={resolvePublicFileUrl(a.avatar_url)}
                  isAgent
                  size="sm"
                />
              </span>
            ))}
            {extraAgents > 0 && (
              <span className="inline-flex size-5 items-center justify-center rounded-full bg-muted text-caption font-medium text-muted-foreground ring-2 ring-card">
                +{extraAgents}
              </span>
            )}
          </div>
        )}
        <span className="min-w-0 truncate text-caption text-muted-foreground">
          {agents.length > 0
            ? t(($) => $.presentation.used_by_count, { count: agents.length })
            : t(($) => $.presentation.used_by_none)}
        </span>
        <span
          className="ml-auto max-w-[45%] truncate text-caption text-muted-foreground"
          title={originLabels[originType]}
          data-origin={originType}
        >
          {originLabels[originType]}
        </span>
      </div>
    </div>
  );
}
