"use client";

import { ChevronDown, Users } from "lucide-react";
import type { AgentRoleKind } from "@multica/core/agents";
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

export const AGENT_ROLE_ORDER: AgentRoleKind[] = [
  "coordinator", "specialist", "other",
];

export function AgentDiscoveryToolbar({ rows, squads, filters, onToggleFilter }: {
  rows: AgentListRow[];
  squads: Squad[];
  filters: AgentListFilters;
  onToggleFilter: (key: keyof AgentListFilters, value: string) => void;
}) {
  const { t } = useT("agents");
  const roles = filters.roles ?? [];
  const selectedSquads = filters.squads ?? [];
  const counts = new Map<AgentRoleKind, number>();
  for (const row of rows) {
    const kind = row.role?.kind ?? "other";
    counts.set(kind, (counts.get(kind) ?? 0) + 1);
  }
  const activeSquads = squads.filter((squad) => !squad.archived_at);
  const selectedName = selectedSquads.length === 1
    ? activeSquads.find((squad) => squad.id === selectedSquads[0])?.name
    : null;

  return (
    <div className={cn("flex shrink-0 flex-wrap items-center gap-2 pb-3", PAGE_GUTTER)}>
      <div className="flex flex-wrap items-center gap-1" aria-label={t(($) => $.discovery.all_roles)}>
        <Button
          size="sm" variant="ghost" aria-pressed={roles.length === 0}
          className={roles.length === 0 ? "bg-accent font-semibold text-accent-foreground" : "text-muted-foreground"}
          onClick={() => roles.forEach((role) => onToggleFilter("roles", role))}
        >
          {t(($) => $.discovery.all_roles)}
        </Button>
        {AGENT_ROLE_ORDER.filter((kind) => counts.has(kind) || roles.includes(kind)).map((kind) => (
          <Button
            key={kind} size="sm" variant="ghost" aria-pressed={roles.includes(kind)}
            className={roles.includes(kind) ? "bg-accent font-semibold text-accent-foreground" : "text-muted-foreground"}
            onClick={() => onToggleFilter("roles", kind)}
          >
            {t(($) => $.discovery.roles[kind])}
            <span className="text-caption tabular-nums text-muted-foreground">{counts.get(kind) ?? 0}</span>
          </Button>
        ))}
      </div>
      {(activeSquads.length > 0 || selectedSquads.length > 0) && (
        <DropdownMenu>
          <DropdownMenuTrigger render={
            <Button size="sm" variant="outline" aria-label={t(($) => $.discovery.squads)} className="max-w-full gap-1.5 sm:ml-auto">
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
    </div>
  );
}
