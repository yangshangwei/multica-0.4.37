"use client";

import { useEffect, useId, useRef, useState } from "react";
import { ChevronDown, Users, X } from "lucide-react";
import type { ConfigureProjectSquadRequest } from "@multica/core/types";
import { DEFAULT_PROJECT_SQUAD_TEMPLATE_KEY } from "@multica/core/projects";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { cn } from "@multica/ui/lib/utils";
import { RuntimePicker } from "../../agents/components/inspector/runtime-picker";
import { useT } from "../../i18n";
import { useProjectSquadOptions } from "./project-squad-picker";

const choiceKey = (choice: ConfigureProjectSquadRequest) =>
  choice.template_key ? `template:${choice.template_key}` : `squad:${choice.squad_id}`;

export function ProjectSquadsPicker({ value, onChange, localDaemonId, localDaemonIds, disabled = false }: {
  value: ConfigureProjectSquadRequest[];
  onChange: (value: ConfigureProjectSquadRequest[]) => void;
  localDaemonId?: string | null;
  localDaemonIds?: readonly string[];
  disabled?: boolean;
}) {
  const { t } = useT("projects");
  const helpId = useId();
  const [open, setOpen] = useState(false);
  const requireManualRuntime = useRef(new Set<string>());
  const deferRuntime = useRef(false);
  const { userId, members, templates, eligibleRuntimes, suggestedRuntime, availableSquads,
    agents, runtimes, runtimesLoading, runtimesError, agentsLoading, memberLoading, templatesLoading, templatesError,
  } = useProjectSquadOptions(localDaemonId, localDaemonIds);

  useEffect(() => {
    if (disabled || runtimesLoading || runtimesError || agentsLoading || memberLoading) return;
    let changed = false;
    const next = value.map((choice) => {
      if (!choice.template_key) return choice;
      const key = choiceKey(choice);
      if (choice.runtime_id && !eligibleRuntimes.some((runtime) => runtime.id === choice.runtime_id)) {
        requireManualRuntime.current.add(key);
        changed = true;
        return { template_key: choice.template_key };
      }
      if (!choice.runtime_id && suggestedRuntime && !deferRuntime.current && !requireManualRuntime.current.has(key)) {
        changed = true;
        return { ...choice, runtime_id: suggestedRuntime.id };
      }
      return choice;
    });
    if (changed) onChange(next);
  }, [value, onChange, eligibleRuntimes, suggestedRuntime, disabled, runtimesLoading, runtimesError, agentsLoading, memberLoading]);

  const localIds = localDaemonId ? [localDaemonId] : localDaemonIds ?? [];
  const wrongMachine = (squadId: string) => {
    if (localIds.length === 0 || runtimesLoading || runtimesError) return false;
    const squad = availableSquads.find((item) => item.id === squadId);
    const leader = agents.find((agent) => agent.id === squad?.leader_id);
    const runtime = runtimes.find((item) => item.id === leader?.runtime_id);
    return !runtime?.daemon_id || !localIds.includes(runtime.daemon_id);
  };
  const options = [
    ...templates.map((template) => ({
      key: `template:${template.key}`, title: template.title, description: template.description,
      choice: { template_key: template.key } as ConfigureProjectSquadRequest,
      group: "templates" as const,
      unavailable: false,
    })),
    ...availableSquads.map((squad) => ({
      key: `squad:${squad.id}`, title: squad.name, description: squad.description,
      choice: { squad_id: squad.id } as ConfigureProjectSquadRequest,
      group: "existing" as const,
      unavailable: wrongMachine(squad.id),
    })),
  ];
  const selectableOptions = options.filter((option) => !option.unavailable);
  const titleFor = (choice: ConfigureProjectSquadRequest) => options.find((option) => option.key === choiceKey(choice))?.title
    ?? (choice.template_key === DEFAULT_PROJECT_SQUAD_TEMPLATE_KEY
      ? t(($) => $.execution_squad.feature_delivery) : t(($) => $.execution_squad.unavailable_choice));
  const selectedKeys = new Set(value.map(choiceKey));
  const templateChoices = value.filter((choice) => choice.template_key);
  const sharedRuntime = templateChoices[0]?.runtime_id;
  const runtimeId = templateChoices.every((choice) => choice.runtime_id === sharedRuntime) ? sharedRuntime ?? "" : "";
  const unavailable = !templatesLoading && !templatesError && !agentsLoading && value.some(
    (choice) => !options.some((option) => option.key === choiceKey(choice)),
  );
  const remove = (key: string) => onChange(value.filter((choice) => choiceKey(choice) !== key));
  const add = (choice: ConfigureProjectSquadRequest) => {
    requireManualRuntime.current.delete(choiceKey(choice));
    return choice.template_key && runtimeId ? { ...choice, runtime_id: runtimeId } : choice;
  };

  return (
    <div className="min-w-0 space-y-3" role="group" aria-label={t(($) => $.execution_squad.label_plural)}>
      <div className="space-y-1">
        <div className="flex items-center gap-2 text-body font-medium">
          <Users className="size-4 text-muted-foreground" aria-hidden="true" />
          <span>{t(($) => $.execution_squad.label_plural)}</span>
          <span className="text-caption font-normal text-muted-foreground">{t(($) => $.execution_squad.optional)}</span>
        </div>
        <p id={helpId} className="text-caption leading-relaxed text-muted-foreground">{t(($) => $.execution_squad.purpose)}</p>
      </div>

      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger render={<Button
          type="button" variant="outline" disabled={disabled}
          aria-label={t(($) => $.execution_squad.choose_multiple)} aria-describedby={helpId}
          className="h-auto min-h-10 w-full justify-between gap-3 py-2 text-body"
        >
          <span className="min-w-0 truncate">{value.length === 0 ? t(($) => $.execution_squad.choose)
            : value.length === 1 ? titleFor(value[0]!) : t(($) => $.execution_squad.selected_count, { count: value.length })}</span>
          <span className="flex shrink-0 items-center gap-2 text-caption text-muted-foreground">
            {t(($) => $.execution_squad.multiple)}<ChevronDown className="size-3.5" aria-hidden="true" />
          </span>
        </Button>} />
        <PopoverContent align="start" className="w-[var(--anchor-width)] min-w-64 max-w-[calc(100vw-2rem)] p-0">
          <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
            <span className="text-caption text-muted-foreground">{t(($) => $.execution_squad.selected_count, { count: value.length })}</span>
            <div className="flex gap-1">
              <Button type="button" size="sm" variant="ghost" disabled={disabled || selectableOptions.length === 0 || selectableOptions.every((option) => selectedKeys.has(option.key))}
                onClick={() => onChange([...value, ...selectableOptions.filter((option) => !selectedKeys.has(option.key)).map((option) => add(option.choice))])}>
                {t(($) => $.execution_squad.select_all)}
              </Button>
              <Button type="button" size="sm" variant="ghost" disabled={disabled || value.length === 0} onClick={() => onChange([])}>
                {t(($) => $.execution_squad.clear)}
              </Button>
            </div>
          </div>
          <div className="max-h-64 overflow-y-auto overscroll-contain p-1">
            {(["templates", "existing"] as const).map((group) => {
              const choices = options.filter((option) => option.group === group);
              if (!choices.length) return null;
              return <div key={group} role="group" aria-label={t(($) => $.execution_squad[group])}>
                <p className="px-2 pb-1 pt-2 text-caption text-muted-foreground">{t(($) => $.execution_squad[group])}</p>
                {choices.map((option) => <label key={option.key} className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-md px-2 py-2 hover:bg-accent/60",
                  selectedKeys.has(option.key) && "bg-accent/40",
                  option.unavailable && "cursor-not-allowed",
                )}>
                  <Checkbox checked={selectedKeys.has(option.key)} disabled={disabled || (option.unavailable && !selectedKeys.has(option.key))} className="mt-0.5"
                    onCheckedChange={(checked) => checked ? onChange([...value, add(option.choice)]) : remove(option.key)} />
                  <span className="min-w-0 flex-1">
                    <span className={cn("block break-words text-body", selectedKeys.has(option.key) && "font-medium")}>{option.title}</span>
                    {option.description && <span className="mt-0.5 block text-caption leading-relaxed text-muted-foreground">{option.description}</span>}
                    {option.unavailable && <span className="mt-0.5 block text-caption text-warning">{t(($) => $.execution_squad.wrong_machine_hint)}</span>}
                  </span>
                </label>)}
              </div>;
            })}
            {options.length === 0 && <p className="px-2 py-3 text-caption text-muted-foreground">{templatesLoading
              ? t(($) => $.execution_squad.checking) : t(($) => $.execution_squad.catalog_failed)}</p>}
          </div>
        </PopoverContent>
      </Popover>

      {value.length > 1 && <div className="flex flex-wrap gap-2">
        {value.map((choice, index) => <span key={choiceKey(choice)} className="inline-flex max-w-full items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-caption">
          <span className="truncate">{titleFor(choice)}</span>
          {index === 0 && <span className="shrink-0 text-muted-foreground">{t(($) => $.execution_squad.default_badge)}</span>}
          <button type="button" disabled={disabled} onClick={() => remove(choiceKey(choice))}
            aria-label={t(($) => $.execution_squad.remove, { name: titleFor(choice) })}
            className="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
            <X className="size-3" aria-hidden="true" />
          </button>
        </span>)}
      </div>}
      <p className="text-caption leading-relaxed text-muted-foreground">{value.length > 0
        ? t(($) => $.execution_squad.selection_hint) : t(($) => $.execution_squad.skip_hint)}</p>
      {templatesError && <p role="status" className="text-caption text-warning">{t(($) => $.execution_squad.catalog_failed)}</p>}
      {unavailable && <p role="status" className="text-caption text-warning">{t(($) => $.execution_squad.unavailable_hint)}</p>}
      {value.some((choice) => choice.squad_id && wrongMachine(choice.squad_id)) && <p role="status" className="text-caption text-warning">{t(($) => $.execution_squad.wrong_machine_hint)}</p>}

      {templateChoices.length > 0 && <div className="space-y-1.5">
        {eligibleRuntimes.length > 0 && <div className="flex items-end gap-2">
        <div className="min-w-0 flex-1 space-y-1.5">
        <p className="text-caption font-medium">{t(($) => $.execution_squad.runtime_label)}</p>
        <RuntimePicker value={runtimeId} runtimes={eligibleRuntimes} members={members} currentUserId={userId}
          canEdit={!disabled} variant="field" showLabel={false}
          onChange={(next) => {
            requireManualRuntime.current.clear();
            deferRuntime.current = false;
            onChange(value.map((choice) => choice.template_key ? { ...choice, runtime_id: next } : choice));
          }} />
        </div>
        {templateChoices.some((choice) => choice.runtime_id) && <Button type="button" size="sm" variant="ghost" disabled={disabled}
          onClick={() => {
            deferRuntime.current = true;
            onChange(value.map((choice) => choice.template_key ? { template_key: choice.template_key } : choice));
          }}>{t(($) => $.execution_squad.connect_later)}</Button>}
        </div>}
        <p className="text-caption leading-relaxed text-muted-foreground">{runtimesError
          ? t(($) => $.execution_squad.runtime_load_failed)
          : runtimeId ? t(($) => $.execution_squad.prepare_simple) : t(($) => $.execution_squad.runtime_optional)}</p>
        {(localDaemonId || localDaemonIds?.length) && <p className="text-caption text-muted-foreground">{t(($) => $.execution_squad.local_runtime_hint)}</p>}
      </div>}
    </div>
  );
}
