"use client";

import { useEffect, useMemo, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { Users } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { canAssignAgentToIssue, useCurrentMember } from "@multica/core/permissions";
import { DEFAULT_PROJECT_SQUAD_TEMPLATE_KEY, eligibleProjectRuntimes, selectProjectRuntime } from "@multica/core/projects";
import { runtimeDisplayLabel, runtimeListOptions } from "@multica/core/runtimes";
import type { ConfigureProjectSquadRequest } from "@multica/core/types";
import { agentListOptions, memberListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { RuntimePicker } from "../../agents/components/inspector/runtime-picker";
import { useSquadTemplates } from "../../agents/create/use-role-templates";
import { useT } from "../../i18n";

export function ProjectSquadPicker({ value, onChange, localDaemonId, localDaemonIds, disabled = false }: {
  value: ConfigureProjectSquadRequest;
  onChange: (value: ConfigureProjectSquadRequest) => void;
  localDaemonId?: string | null;
  localDaemonIds?: readonly string[];
  disabled?: boolean;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const { userId, role, isLoading: memberLoading } = useCurrentMember(wsId);
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: agents = [], isLoading: agentsLoading } = useQuery(agentListOptions(wsId));
  const { data: squads = [] } = useQuery(squadListOptions(wsId));
  const { data: runtimes = [], isLoading: runtimesLoading, isError: runtimesError } = useQuery(runtimeListOptions(wsId));
  const { data: templates = [], isLoading: templatesLoading, isError: templatesError } = useSquadTemplates();
  const requireManualRuntime = useRef(false);

  const eligibleRuntimes = useMemo(() => eligibleProjectRuntimes(runtimes, {
    workspaceId: wsId, userId, daemonId: localDaemonId, daemonIds: localDaemonIds,
  }), [runtimes, wsId, userId, localDaemonId, localDaemonIds]);
  const mika = agents.find((agent) => agent.system_key === "mika" && !agent.archived_at &&
    agent.workspace_id === wsId && canAssignAgentToIssue(agent, { userId, role }).allowed);
  const suggestedRuntime = selectProjectRuntime(runtimes, {
    workspaceId: wsId, userId, daemonId: localDaemonId, daemonIds: localDaemonIds,
    preferredRuntimeId: mika?.runtime_id,
  });

  useEffect(() => {
    if (disabled || !value.template_key || runtimesLoading || runtimesError || agentsLoading || memberLoading) return;
    if (value.runtime_id && !eligibleRuntimes.some((runtime) => runtime.id === value.runtime_id)) {
      // Keep the template, but never silently move a retained choice to a
      // different machine after a permission or local-directory change.
      requireManualRuntime.current = true;
      onChange({ template_key: value.template_key });
    } else if (!value.runtime_id && suggestedRuntime && !requireManualRuntime.current) {
      onChange({ ...value, runtime_id: suggestedRuntime.id });
    }
  }, [value, eligibleRuntimes, suggestedRuntime, runtimesLoading, runtimesError, agentsLoading, memberLoading, disabled, onChange]);

  const availableSquads = squads.filter((squad) => {
    const leader = agents.find((agent) => agent.id === squad.leader_id && agent.workspace_id === wsId);
    return squad.workspace_id === wsId && !squad.archived_at && leader && !leader.archived_at &&
      canAssignAgentToIssue(leader, { userId, role }).allowed;
  });
  const template = templates.find((item) => item.key === value.template_key);
  const squad = availableSquads.find((item) => item.id === value.squad_id);
  const squadLeader = squad ? agents.find((agent) => agent.id === squad.leader_id) : null;
  const squadRuntime = runtimes.find((runtime) => runtime.id === squadLeader?.runtime_id);
  const localIds = localDaemonId ? [localDaemonId] : localDaemonIds ?? [];
  const squadWrongMachine = squad && localIds.length > 0 &&
    (!squadRuntime?.daemon_id || !localIds.includes(squadRuntime.daemon_id));
  const selection = value.template_key ? `template:${value.template_key}` : value.squad_id ? `squad:${value.squad_id}` : "none";
  const fallbackTitle = value.template_key === DEFAULT_PROJECT_SQUAD_TEMPLATE_KEY
    ? t(($) => $.execution_squad.feature_delivery)
    : t(($) => $.execution_squad.unavailable_choice);
  const items = [
    ...templates.map((item) => ({ value: `template:${item.key}`, label: item.title })),
    ...availableSquads.map((item) => ({ value: `squad:${item.id}`, label: item.name })),
    { value: "none", label: t(($) => $.execution_squad.none) },
  ];
  if (!items.some((item) => item.value === selection)) {
    items.push({ value: selection, label: fallbackTitle });
  }

  return (
    <div className="min-w-0 space-y-2">
      <div className="flex items-center gap-2 text-caption font-medium">
        <Users className="size-4 text-muted-foreground" aria-hidden="true" />
        <span>{t(($) => $.execution_squad.label)}</span>
      </div>
      <Select
        value={selection}
        items={items}
        disabled={disabled}
        onValueChange={(next) => {
          if (!next) return;
          requireManualRuntime.current = false;
          if (next.startsWith("template:")) {
            onChange({ template_key: next.slice(9), ...(value.runtime_id ? { runtime_id: value.runtime_id } : {}) });
          } else if (next.startsWith("squad:")) {
            onChange({ squad_id: next.slice(6) });
          } else {
            onChange({});
          }
        }}
      >
        <SelectTrigger aria-label={t(($) => $.execution_squad.label)} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent align="start" alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectLabel>{t(($) => $.execution_squad.templates)}</SelectLabel>
            {templates.map((item) => (
              <SelectItem key={item.key} value={`template:${item.key}`} className="data-selected:font-semibold">
                {item.title}
              </SelectItem>
            ))}
          </SelectGroup>
          {availableSquads.length > 0 && <SelectGroup>
            <SelectLabel>{t(($) => $.execution_squad.existing)}</SelectLabel>
            {availableSquads.map((item) => <SelectItem key={item.id} value={`squad:${item.id}`} className="data-selected:font-semibold">{item.name}</SelectItem>)}
          </SelectGroup>}
          <SelectGroup><SelectItem value="none">{t(($) => $.execution_squad.none)}</SelectItem></SelectGroup>
        </SelectContent>
      </Select>

      {template && <p className="text-caption leading-relaxed text-muted-foreground">{template.description}</p>}
      {templatesError && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.execution_squad.catalog_failed)}</p>}
      {!templatesLoading && value.template_key && !template && !templatesError && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.execution_squad.unavailable_hint)}</p>}
      {value.squad_id && !squad && !agentsLoading && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.execution_squad.unavailable_hint)}</p>}
      {squad && <p className="text-caption text-muted-foreground">{squadRuntime
        ? t(($) => $.execution_squad.existing_runtime, { runtime: runtimeDisplayLabel(squadRuntime) })
        : t(($) => $.execution_squad.existing_runtime_hint)}</p>}
      {squadWrongMachine && <p role="status" className="text-caption text-warning">{t(($) => $.execution_squad.wrong_machine_hint)}</p>}

      {value.template_key && <div className="space-y-2 pt-1">
        <RuntimePicker
          value={value.runtime_id ?? ""}
          runtimes={eligibleRuntimes}
          members={members}
          currentUserId={userId}
          canEdit={!disabled}
          variant="field"
          onChange={(runtimeId) => onChange({ template_key: value.template_key, runtime_id: runtimeId })}
        />
        <p className="text-caption leading-relaxed text-muted-foreground">{runtimesError
          ? t(($) => $.execution_squad.runtime_load_failed)
          : value.runtime_id ? t(($) => $.execution_squad.prepare_hint)
            : t(($) => $.execution_squad.choose_runtime_hint)}</p>
        {localIds.length > 0 && <p className="text-caption text-muted-foreground">{t(($) => $.execution_squad.local_runtime_hint)}</p>}
      </div>}
    </div>
  );
}
