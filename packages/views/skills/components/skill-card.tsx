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
import { LabelChip } from "../../labels/label-chip";
import { SkillPresentationIcon } from "./skill-presentation-icon";
import { SkillRowActions, type SkillActionsContext } from "./skill-list-actions";
import { originIcon } from "./skill-list-toolbar";
import type { PresentedSkillRow } from "./skills-page";

/** Fixed card height — the grid virtualizes by row on this contract. */
export const SKILL_CARD_HEIGHT = 172;

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
}: {
  row: PresentedSkillRow;
  ctx: SkillActionsContext;
  selected: boolean;
  onToggleSelected: () => void;
  linkProps: React.HTMLAttributes<HTMLDivElement>;
}) {
  const { t } = useT("skills");
  const { skill, presentation, meta, labels, agents, canEdit, originType } = row;
  const visibleLabels = labels.slice(0, MAX_LABELS);
  const extraLabels = labels.length - visibleLabels.length;
  const visibleAgents = agents.slice(0, MAX_AVATARS);
  const extraAgents = agents.length - visibleAgents.length;

  return (
    <div
      {...linkProps}
      data-selected={selected ? "" : undefined}
      data-testid="skill-card"
      style={{ height: SKILL_CARD_HEIGHT }}
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
            <span
              className="min-w-0 truncate text-body font-medium"
              title={skill.name}
            >
              {presentation.name}
            </span>
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
        <div className="flex shrink-0 items-center gap-0.5">
          <button
            type="button"
            aria-pressed={selected}
            aria-label={presentation.name}
            onClick={(e) => {
              e.stopPropagation();
              onToggleSelected();
            }}
            onAuxClick={(e) => e.stopPropagation()}
            className={cn(
              "flex items-center p-1 transition-opacity",
              selected
                ? ""
                : "opacity-30 group-hover/row:opacity-100 focus-visible:opacity-100",
            )}
          >
            <Checkbox checked={selected} tabIndex={-1} className="pointer-events-none" />
          </button>
          <SkillRowActions row={row} ctx={ctx} />
        </div>
      </div>

      <p
        className="line-clamp-2 min-h-[2lh] text-caption text-muted-foreground"
        title={presentation.description}
      >
        {presentation.description}
      </p>

      <div className="flex min-h-5 items-center gap-1 overflow-hidden">
        {visibleLabels.map((label) => (
          <LabelChip key={label.id} label={label} className="max-w-24" />
        ))}
        {extraLabels > 0 && (
          <Badge variant="outline">
            {t(($) => $.table.labels_more, { count: extraLabels })}
          </Badge>
        )}
      </div>

      <div className="mt-auto flex items-center gap-2">
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
          className="ml-auto shrink-0 text-muted-foreground"
          data-origin={originType}
        >
          {originIcon(originType)}
        </span>
      </div>
    </div>
  );
}
