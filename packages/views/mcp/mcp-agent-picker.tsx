"use client";

import { useId, useRef, useState } from "react";
import { Check, Search } from "lucide-react";
import type { Agent } from "@multica/core/types";
import { resolvePublicFileUrl } from "@multica/core/workspace/avatar-url";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { descriptionPreview } from "../issues/components/description-preview";

export type McpAgentChoice = Pick<Agent, "id" | "name"> &
  Partial<Pick<Agent, "description" | "avatar_url">>;

export function McpAgentPicker({
  agents,
  selected,
  succeeded,
  failures,
  disabled,
  showTools,
  onSelectionChange,
  onClearSelection,
}: {
  agents: readonly McpAgentChoice[];
  selected: ReadonlySet<string>;
  succeeded: ReadonlySet<string>;
  failures: readonly { agentId: string; message: string }[];
  disabled: boolean;
  showTools: boolean;
  onSelectionChange: (agentIds: readonly string[], checked: boolean) => void;
  onClearSelection: () => void;
}) {
  const { t } = useT("settings");
  const [search, setSearch] = useState("");
  const [selectedOnly, setSelectedOnly] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const id = useId();
  const query = search.trim().toLowerCase();
  const visible = agents.filter((agent) =>
    (!selectedOnly || selected.has(agent.id) || succeeded.has(agent.id)) &&
    (!query || agent.name.toLowerCase().includes(query) ||
      (typeof agent.description === "string" && agent.description.toLowerCase().includes(query))),
  );
  const visibleIds = new Set(visible.map((agent) => agent.id));
  const unfinished = agents.filter((agent) => selected.has(agent.id) && !succeeded.has(agent.id));
  const unfinishedResults = visible.filter((agent) => !succeeded.has(agent.id));
  const assignedCount = agents.filter((agent) => succeeded.has(agent.id)).length;
  const hiddenSelectedCount = unfinished.filter((agent) => !visibleIds.has(agent.id)).length;
  const failureById = new Map(failures.map((failure) => [failure.agentId, failure]));
  const hiddenFailures = failures.filter((failure) => !visibleIds.has(failure.agentId));
  const clearSearch = () => {
    if (disabled) return;
    setSearch("");
    searchRef.current?.focus();
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3" data-slot="mcp-agent-picker">
      <div className="shrink-0 space-y-3" data-slot="mcp-agent-toolbar">
        {showTools ? (
          <div className="flex items-center gap-2">
            <div className="relative min-w-0 flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
              <Input
                ref={searchRef}
                type="search"
                aria-label={t(($) => $.mcp.market.agents_search)}
                placeholder={t(($) => $.mcp.market.agents_search_placeholder)}
                value={search}
                onChange={(event) => { if (!disabled) setSearch(event.target.value); }}
                disabled={disabled}
                autoComplete="off"
                className="pl-9"
              />
            </div>
            {search ? (
              <Button size="sm" variant="outline" disabled={disabled} onClick={clearSearch} className="h-auto min-h-8 whitespace-normal py-1.5">
                {t(($) => $.mcp.market.agents_clear_search)}
              </Button>
            ) : null}
          </div>
        ) : null}
        <div role="status" aria-live="polite" aria-atomic="true" className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground tabular-nums">
          <span>{t(($) => $.mcp.market.agents_results_count, { count: visible.length })}</span>
          <span className="font-medium text-foreground">{t(($) => $.mcp.market.agents_selected_count, { count: unfinished.length })}</span>
          {assignedCount > 0 ? <span>{t(($) => $.mcp.market.agents_assigned_count, { count: assignedCount })}</span> : null}
          {hiddenSelectedCount > 0 ? <span>{t(($) => $.mcp.market.agents_hidden_selected_count, { count: hiddenSelectedCount })}</span> : null}
        </div>
        {showTools ? (
          <div className="flex flex-wrap gap-2 [&_[data-slot=button]]:h-auto [&_[data-slot=button]]:min-h-8 [&_[data-slot=button]]:whitespace-normal [&_[data-slot=button]]:py-1.5">
            <Button
              size="sm"
              variant={selectedOnly ? "secondary" : "outline"}
              aria-pressed={selectedOnly}
              disabled={disabled}
              onClick={() => { if (!disabled) setSelectedOnly((previous) => !previous); }}
              className={selectedOnly ? "font-semibold" : undefined}
            >
              {t(($) => $.mcp.market.agents_selected_only)}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={disabled || !unfinishedResults.some((agent) => !selected.has(agent.id))}
              onClick={() => onSelectionChange(unfinishedResults.map((agent) => agent.id), true)}
            >
              {t(($) => $.mcp.market.agents_select_results)}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={disabled || !unfinishedResults.some((agent) => selected.has(agent.id))}
              onClick={() => onSelectionChange(unfinishedResults.map((agent) => agent.id), false)}
            >
              {t(($) => $.mcp.market.agents_deselect_results)}
            </Button>
            <Button size="sm" variant="ghost" disabled={disabled || unfinished.length === 0} onClick={onClearSelection}>
              {t(($) => $.mcp.market.agents_clear_selection)}
            </Button>
          </div>
        ) : null}
      </div>
      <div className="min-h-0 overflow-y-auto overscroll-contain rounded-lg border" data-slot="mcp-agent-list">
        {hiddenFailures.length > 0 ? (
          <div className="space-y-1 border-b px-3 py-3 text-caption text-destructive" role="alert">
            {hiddenFailures.map((failure) => (
              <p key={failure.agentId} className="[overflow-wrap:anywhere]">
                {t(($) => $.mcp.market.agents_failure, { name: agents.find((agent) => agent.id === failure.agentId)?.name ?? failure.agentId, message: failure.message })}
              </p>
            ))}
          </div>
        ) : null}
        {visible.length === 0 ? (
          <p className="px-4 py-8 text-center text-caption text-muted-foreground">
            {query
              ? selectedOnly
                ? t(($) => $.mcp.market.agents_selected_no_matches)
                : t(($) => $.mcp.market.agents_no_matches)
              : t(($) => $.mcp.market.agents_selected_empty)}
          </p>
        ) : visible.map((agent) => {
          const assigned = succeeded.has(agent.id);
          const checked = selected.has(agent.id) || assigned;
          const rowDisabled = disabled || assigned;
          const failure = failureById.get(agent.id);
          const description = typeof agent.description === "string" ? agent.description : "";
          const rowId = `${id}-${agent.id}`;
          const describedBy = [description ? `${rowId}-description` : "", assigned ? `${rowId}-assigned` : "", failure ? `${rowId}-failure` : ""].filter(Boolean).join(" ");
          // Anchor the full sr-only description inside this scrollable row.
          return (
            <Label
              key={agent.id}
              htmlFor={rowId}
              data-selected={checked || undefined}
              data-disabled={rowDisabled || undefined}
              className="relative min-h-14 cursor-pointer gap-3 border-b px-3 py-2.5 font-normal last:border-b-0 hover:bg-muted/50 data-[selected=true]:bg-accent data-[selected=true]:hover:bg-accent data-[disabled=true]:cursor-default has-focus-visible:ring-2 has-focus-visible:ring-inset has-focus-visible:ring-foreground"
            >
              <Checkbox
                id={rowId}
                checked={checked}
                disabled={rowDisabled}
                aria-labelledby={`${rowId}-name`}
                aria-describedby={describedBy || undefined}
                className="focus-visible:border-foreground focus-visible:ring-foreground pointer-coarse:after:-inset-3.5"
                onCheckedChange={(value) => onSelectionChange([agent.id], value)}
              />
              <span aria-hidden="true">
                <ActorAvatar
                  name={agent.name}
                  initials={agent.name.slice(0, 2).toUpperCase()}
                  isAgent
                  size="lg"
                  avatarUrl={typeof agent.avatar_url === "string" && agent.avatar_url ? resolvePublicFileUrl(agent.avatar_url) : null}
                />
              </span>
              <span className="min-w-0 flex-1 space-y-1">
                <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                  <span id={`${rowId}-name`} className={cn("min-w-0 flex-1 text-body leading-5 [overflow-wrap:anywhere]", checked && "font-medium")}>
                    {agent.name}
                  </span>
                  {assigned ? (
                    <span id={`${rowId}-assigned`} className="flex items-center gap-1 text-caption text-muted-foreground">
                      <Check className="size-3" aria-hidden="true" />
                      {t(($) => $.mcp.market.assigned)}
                    </span>
                  ) : null}
                </span>
                {description ? (
                  <>
                    <span aria-hidden="true" className="line-clamp-2 text-caption leading-5 text-muted-foreground [overflow-wrap:anywhere]">
                      {descriptionPreview(description)}
                    </span>
                    <span id={`${rowId}-description`} className="sr-only">{description}</span>
                  </>
                ) : null}
                {failure ? (
                  <span id={`${rowId}-failure`} role="alert" className="block text-caption leading-5 text-destructive [overflow-wrap:anywhere]">{failure.message}</span>
                ) : null}
              </span>
            </Label>
          );
        })}
      </div>
    </div>
  );
}
