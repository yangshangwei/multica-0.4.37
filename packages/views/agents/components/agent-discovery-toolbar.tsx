"use client";

import { useId } from "react";
import { Trans } from "react-i18next";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { ChevronDown, Folder, Users, X } from "lucide-react";
import { AGENT_CATEGORY_PRESET_ORDER } from "@multica/core/agents";
import type { Squad } from "@multica/core/types";
import type { AgentListFilters } from "@multica/core/agents/stores";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useT } from "../../i18n";
import { PAGE_GUTTER } from "../../layout/page-header";
import { cn } from "@multica/ui/lib/utils";
import type { AgentListRow } from "./agents-page";

export function AgentDiscoveryToolbar({ rows, squads, filters, onToggleFilter }: {
  rows: AgentListRow[];
  squads: Squad[];
  filters: AgentListFilters;
  onToggleFilter: (key: keyof AgentListFilters, value: string) => void;
}) {
  const { t } = useT("agents");
  const generalHintId = useId();
  const paths = useWorkspacePaths();
  const selectedSquads = filters.squads ?? [];
  const selectedCategories = filters.categories ?? [];
  const categoryCounts = new Map<string, number>();
  const categoryNames = new Map<string, string>();
  const categoriesIncomplete = rows.some((row) => row.category === null);
  for (const row of rows) {
    if (!row.category) continue;
    categoryCounts.set(row.category.key, (categoryCounts.get(row.category.key) ?? 0) + 1);
    categoryNames.set(row.category.key, row.category.name);
  }
  for (const preset of AGENT_CATEGORY_PRESET_ORDER) {
    categoryNames.set(`preset:${preset}`, t(($) => $.discovery.roles[preset]));
  }
  const categoryLabel = (key: string) => categoryNames.get(key) ?? key.slice("custom:".length);
  const knownKeys = new Set([...categoryCounts.keys(), ...selectedCategories]);
  const categoryOptions = [
    ...AGENT_CATEGORY_PRESET_ORDER.map((preset) => `preset:${preset}`).filter((key) => knownKeys.has(key)),
    ...[...knownKeys].filter((key) => key.startsWith("custom:")).sort((a, b) => categoryLabel(a).localeCompare(categoryLabel(b))),
  ];
  const categoryCount = (key: string) => `${categoryCounts.get(key) ?? 0}${categoriesIncomplete && key.startsWith("preset:") ? "+" : ""}`;
  const showGeneralHint = knownKeys.has("preset:other");
  const mika = rows.find(({ agent }) => agent.system_key === "mika" && !agent.archived_at)?.agent;
  const activeSquads = squads.filter((squad) => !squad.archived_at);
  const selectedName = selectedSquads.length === 1
    ? activeSquads.find((squad) => squad.id === selectedSquads[0])?.name
    : null;

  return (
    <div className={cn("flex shrink-0 flex-wrap items-center gap-2 pb-3", PAGE_GUTTER)}>
      <div className="flex flex-wrap items-center gap-1" aria-label={t(($) => $.category.all)}>
        <Button
          size="sm" variant="ghost" aria-pressed={selectedCategories.length === 0}
          className={selectedCategories.length === 0 ? "bg-accent font-semibold text-accent-foreground" : "text-muted-foreground"}
          onClick={() => selectedCategories.forEach((key) => onToggleFilter("categories", key))}
        >
          {t(($) => $.category.all)}
        </Button>
        {AGENT_CATEGORY_PRESET_ORDER.filter((kind) => knownKeys.has(`preset:${kind}`)).map((kind) => (
          <Button
            key={kind} size="sm" variant="ghost" aria-pressed={selectedCategories.includes(`preset:${kind}`)}
            aria-describedby={kind === "other" && showGeneralHint ? generalHintId : undefined}
            className={selectedCategories.includes(`preset:${kind}`) ? "bg-accent font-semibold text-accent-foreground" : "text-muted-foreground"}
            onClick={() => onToggleFilter("categories", `preset:${kind}`)}
          >
            {t(($) => $.discovery.roles[kind])}
            <span className="text-caption tabular-nums text-muted-foreground">{categoryCount(`preset:${kind}`)}</span>
          </Button>
        ))}
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger render={
          <Button size="sm" variant="outline" aria-label={t(($) => $.category.label)} className={cn("max-w-full gap-1.5 sm:ml-auto", selectedCategories.length > 0 && "border-primary/40 bg-accent text-accent-foreground")}>
            <Folder className="size-3.5 shrink-0" aria-hidden="true" />
            <span className="max-w-40 truncate">
              {selectedCategories.length === 1 ? categoryLabel(selectedCategories[0]!) : t(($) => selectedCategories.length ? $.category.label : $.category.all)}
            </span>
            {selectedCategories.length > 1 && <span className="tabular-nums">{selectedCategories.length}</span>}
            <ChevronDown className="size-3 shrink-0" aria-hidden="true" />
          </Button>
        } />
        <DropdownMenuContent align="end" className="max-h-72 max-w-80 overflow-y-auto">
          <DropdownMenuItem onClick={() => selectedCategories.forEach((category) => onToggleFilter("categories", category))}>
            {t(($) => $.category.all)}
          </DropdownMenuItem>
          {categoryOptions.map((category) => (
            <DropdownMenuCheckboxItem key={category} checked={selectedCategories.includes(category)}
              onCheckedChange={() => onToggleFilter("categories", category)}>
              <span className="min-w-0 flex-1 truncate" title={categoryLabel(category)}>{categoryLabel(category)}</span>
              <span className="ml-auto pl-3 text-caption tabular-nums text-muted-foreground">{categoryCount(category)}</span>
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {(activeSquads.length > 0 || selectedSquads.length > 0) && (
        <DropdownMenu>
          <DropdownMenuTrigger render={
            <Button size="sm" variant="outline" aria-label={t(($) => $.discovery.squads)} className={cn("max-w-full gap-1.5", selectedSquads.length > 0 && "border-primary/40 bg-accent text-accent-foreground")}>
              <Users className="size-3.5 shrink-0" aria-hidden="true" />
              <span className="max-w-56 truncate">
                {selectedName ?? t(($) => selectedSquads.length ? $.discovery.squads : $.discovery.all_squads)}
              </span>
              {selectedSquads.length > 1 && <span className="tabular-nums">{selectedSquads.length}</span>}
              <ChevronDown className="size-3 shrink-0" aria-hidden="true" />
            </Button>
          } />
          <DropdownMenuContent align="end" className="max-h-72 max-w-80 overflow-y-auto">
            <DropdownMenuItem onClick={() => selectedSquads.forEach((id) => onToggleFilter("squads", id))}>
              {t(($) => $.discovery.all_squads)}
            </DropdownMenuItem>
            {activeSquads.map((squad) => (
              <DropdownMenuCheckboxItem
                key={squad.id} checked={selectedSquads.includes(squad.id)}
                onCheckedChange={() => onToggleFilter("squads", squad.id)}
              >
                <span className="truncate">{squad.name}</span>
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      {(selectedCategories.length > 0 || selectedSquads.length > 0) && (
        <div className="flex basis-full flex-wrap gap-1.5">
          {selectedCategories.map((category) => (
            <Button key={category} size="sm" variant="secondary" className="max-w-full gap-1.5"
              aria-label={t(($) => $.discovery.clear_category, { name: categoryLabel(category) })}
              onClick={() => onToggleFilter("categories", category)}>
              <Folder className="size-3.5 shrink-0" aria-hidden="true" />
              <span className="truncate">{categoryLabel(category)}</span>
              <X className="size-3 shrink-0" aria-hidden="true" />
            </Button>
          ))}
          {selectedSquads.map((id) => {
            const name = squads.find((squad) => squad.id === id)?.name ?? id;
            return (
              <Button key={id} size="sm" variant="secondary" className="max-w-full gap-1.5"
                aria-label={t(($) => $.discovery.clear_squad, { name })}
                onClick={() => onToggleFilter("squads", id)}>
                <Users className="size-3.5 shrink-0" aria-hidden="true" />
                <span className="truncate">{name}</span>
                <X className="size-3 shrink-0" aria-hidden="true" />
              </Button>
            );
          })}
        </div>
      )}
      {showGeneralHint && (
        <p id={generalHintId} className="basis-full px-2 text-caption leading-relaxed text-muted-foreground">
          {mika ? (
            <Trans ns="agents" i18nKey={($) => $.discovery.mika_hint_link} values={{ name: mika.name }}
              components={{ mika: <AppLink href={paths.agentDetail(mika.id)} newTabTitle={mika.name}
                className="rounded-sm font-medium text-foreground underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-ring" /> }} />
          ) : t(($) => $.discovery.general_hint)}
        </p>
      )}
    </div>
  );
}
