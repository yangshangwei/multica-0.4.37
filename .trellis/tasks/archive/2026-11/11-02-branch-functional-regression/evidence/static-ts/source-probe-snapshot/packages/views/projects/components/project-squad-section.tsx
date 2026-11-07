"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { ArrowRight, Check, Loader2, MoreHorizontal, Plus, Users, X } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import { useWorkspacePaths } from "@multica/core/paths";
import { canAssignAgentToIssue, useCurrentMember } from "@multica/core/permissions";
import { getProjectExecutionSquads, getProjectSquadReadiness, projectLocalDaemonIds, projectResourcesOptions, projectSquadSelection, replaceProjectSquadSelection } from "@multica/core/projects";
import { useConfigureProjectSquads } from "@multica/core/projects/mutations";
import { runtimeDisplayLabel, runtimeListOptions } from "@multica/core/runtimes";
import type { ConfigureProjectSquadRequest, Project, ProjectExecutionSquad } from "@multica/core/types";
import { agentListOptions, memberListOptions, squadListOptions, squadMemberStatusOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@multica/ui/components/ui/dropdown-menu";
import { Sheet, SheetClose, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@multica/ui/components/ui/sheet";
import { cn } from "@multica/ui/lib/utils";
import { templateLanguageFor, useSquadTemplates } from "../../agents/create/use-role-templates";
import { useLocale, useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { ProjectSquadPicker } from "./project-squad-picker";

export function ProjectSquadSection({ project }: { project: Project }) {
  const { t } = useT("projects");
  const { t: tModals } = useT("modals");
  const wsId = useWorkspaceId();
  const configure = useConfigureProjectSquads(wsId);
  const { configs, rows, emptyRow, localDaemonIds, configurationBlocked } = useProjectSquadState(wsId, project);
  const pending = useRef(false);
  const issueToCreate = useRef<{ projectId: string; workspaceId: string; squadId: string } | null>(null);
  const [managerState, setManagerState] = useState<"open" | "closing" | "closed">("closed");
  const [adding, setAdding] = useState(false);
  const availableCount = rows.filter((row) => row.available).length;
  const checkingCount = rows.filter((row) => row.loading && !row.queryError).length;
  const attentionCount = rows.length - availableCount - checkingCount;
  const defaultRow = rows[0];
  const defaultSummary = defaultRow ? t(($) => $.execution_squad.default_summary, { name: defaultRow.name }) : null;
  const defaultWarning = !defaultRow || defaultRow.available ? null
    : defaultRow.queryError ? t(($) => $.execution_squad.check_failed)
      : defaultRow.loading ? t(($) => $.execution_squad.default_checking)
        : t(($) => $.execution_squad.default_unavailable);

  const save = async (squads: ConfigureProjectSquadRequest[]) => {
    if (pending.current || configure.isPending || configurationBlocked) return false;
    pending.current = true;
    try {
      await configure.mutateAsync({ id: project.id, squads });
      setAdding(false);
      return true;
    } finally {
      pending.current = false;
    }
  };

  const createIssue = (row: ProjectSquadState) => {
    if (!row.available || !row.config?.squad_id || pending.current || configure.isPending) return;
    issueToCreate.current = { projectId: project.id, workspaceId: wsId, squadId: row.config.squad_id };
    setManagerState("closing");
  };

  useEffect(() => {
    if (managerState !== "closed") return;
    const requested = issueToCreate.current;
    issueToCreate.current = null;
    if (!requested || requested.projectId !== project.id || requested.workspaceId !== wsId || pending.current || configure.isPending) return;
    if (!rows.some((row) => row.config?.squad_id === requested.squadId && row.available)) return;
    // The close callback schedules unmounting; this effect runs after that commit.
    useModalStore.getState().open("create-issue", {
      project_id: project.id, assignee_type: "squad", assignee_id: requested.squadId, status: "todo",
    });
  }, [managerState, project.id, wsId, configure.isPending, rows]);

  return (
    <section aria-label={t(($) => $.execution_squad.label_plural)} className="shrink-0 border-b px-4 py-3 sm:px-6">
      <Sheet open={managerState === "open"} onOpenChange={(open) => {
        if (open) issueToCreate.current = null;
        setManagerState(open ? "open" : "closing");
      }} onOpenChangeComplete={(open) => {
        if (open) return;
        setAdding(false);
        setManagerState("closed");
      }}>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <Users className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              <h2 className="text-body font-medium">{t(($) => $.execution_squad.summary_label)}</h2>
              <span className="text-caption tabular-nums text-muted-foreground">{t(($) => $.execution_squad.selected_count, { count: configs.length })}</span>
              <span role="status" className="flex flex-wrap gap-x-2 gap-y-1 text-caption tabular-nums text-muted-foreground">
                {availableCount > 0 && <span>{t(($) => $.execution_squad.available_count, { count: availableCount })}</span>}
                {attentionCount > 0 && <span className="text-destructive">{t(($) => $.execution_squad.attention_count, { count: attentionCount })}</span>}
                {checkingCount > 0 && <span>{t(($) => $.execution_squad.checking_count, { count: checkingCount })}</span>}
              </span>
            </div>
            <p className="truncate text-caption text-muted-foreground" title={defaultSummary ?? undefined}>
              {defaultSummary ?? t(($) => $.execution_squad.no_squads)}
            </p>
          </div>
          <SheetTrigger render={<Button size="sm" variant="ghost" />}>
            {t(($) => $.execution_squad.manage)}
          </SheetTrigger>
        </div>
        {defaultWarning && <p role="status" className="mt-2 text-caption text-muted-foreground">{defaultWarning}</p>}

        <SheetContent showCloseButton={false} className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-lg">
          <SheetHeader className="shrink-0 gap-2 p-5 pr-12">
            <SheetTitle>{t(($) => $.execution_squad.manage)}</SheetTitle>
            <SheetDescription>{t(($) => $.execution_squad.manage_description)}</SheetDescription>
          </SheetHeader>
          <SheetClose render={<Button size="icon-sm" variant="ghost" className="absolute right-3 top-3" aria-label={tModals(($) => $.common.close)} />}>
            <X className="size-4" aria-hidden="true" />
          </SheetClose>
          <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-5 pb-5">
            {configs.length > 0 && !adding && <Button size="sm" variant="outline" disabled={configure.isPending} onClick={() => setAdding(true)}>
              <Plus className="size-3.5" aria-hidden="true" />{t(($) => $.execution_squad.add)}
            </Button>}
            {rows.map((row, index) => <ProjectSquadRow
              key={row.config?.squad_id ?? (row.config?.template_key ? `${row.config.template_key}:${row.config.runtime_id ?? ""}` : `invalid-${index}`)}
              state={row} pending={configure.isPending} configurationBlocked={configurationBlocked} localDaemonIds={localDaemonIds} isDefault={index === 0}
              onSave={(request) => save(replaceProjectSquadSelection(configs, index, request))}
              onSetDefault={() => save([configs[index]!, ...configs.filter((_, position) => position !== index)].map(projectSquadSelection))}
              onCreateIssue={() => createIssue(row)}
            />)}
            {(configs.length === 0 || adding) && <ProjectSquadRow
              key="new" state={emptyRow} pending={configure.isPending} configurationBlocked={configurationBlocked} localDaemonIds={localDaemonIds} startEditing
              onSave={(request) => save(replaceProjectSquadSelection(configs, configs.length, request))} onCancel={() => setAdding(false)}
            />}
          </div>
        </SheetContent>
      </Sheet>
    </section>
  );
}

type ProjectSquadState = ReturnType<typeof useProjectSquadState>["rows"][number];

function ProjectSquadRow({ state, pending, configurationBlocked: prerequisitesBlocked, localDaemonIds, onSave, onSetDefault, onCreateIssue, startEditing = false, onCancel, isDefault = false }: {
  state: ProjectSquadState;
  pending: boolean;
  configurationBlocked: boolean;
  localDaemonIds: string[];
  onSave: (request: ConfigureProjectSquadRequest) => Promise<boolean>;
  onSetDefault?: () => Promise<boolean>;
  onCreateIssue?: () => void;
  startEditing?: boolean;
  onCancel?: () => void;
  isDefault?: boolean;
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const removeDescriptionId = useId();
  const [choice, setChoice] = useState<ConfigureProjectSquadRequest | null>(startEditing ? {} : null);
  const [error, setError] = useState<string | null>(null);
  const [showRuntimeDetails, setShowRuntimeDetails] = useState(false);
  const { config, squad, name, readiness, loading, queryError, paused, available, actualRuntimes } = state;
  const configurationBlocked = pending || prerequisitesBlocked;
  const retainedChoice = config ? projectSquadSelection(config) : {};
  const hasRetainedChoice = !!(retainedChoice.template_key?.trim() || retainedChoice.squad_id?.trim());

  const statusLabels = {
    none: t(($) => $.execution_squad.status_none),
    needs_runtime: t(($) => $.execution_squad.status_needs_runtime),
    failed: t(($) => $.execution_squad.status_failed),
    unavailable: t(($) => $.execution_squad.status_unavailable),
    offline: t(($) => $.execution_squad.status_offline),
    wrong_machine: t(($) => $.execution_squad.status_wrong_machine),
    ready: t(($) => $.execution_squad.status_ready),
  };
  const setupErrors: Record<string, string> = {
    agent_name_conflict: t(($) => $.execution_squad.errors.agent_name_conflict),
    agent_access_denied: t(($) => $.execution_squad.errors.agent_access_denied),
    agent_unavailable: t(($) => $.execution_squad.errors.agent_unavailable),
    runtime_unavailable: t(($) => $.execution_squad.errors.runtime_unavailable),
    runtime_mismatch: t(($) => $.execution_squad.errors.runtime_mismatch),
    squad_unavailable: t(($) => $.execution_squad.errors.squad_unavailable),
  };
  const hints = {
    none: t(($) => $.execution_squad.none_hint),
    needs_runtime: t(($) => $.execution_squad.needs_runtime_hint),
    failed: setupErrors[config?.error_code ?? ""] ?? t(($) => $.execution_squad.errors.preparation_failed),
    unavailable: t(($) => $.execution_squad.unavailable_hint),
    offline: t(($) => $.execution_squad.offline_hint),
    wrong_machine: t(($) => $.execution_squad.wrong_machine_hint),
    ready: t(($) => $.execution_squad.ready_hint),
  };

  const update = async (write: () => Promise<boolean>) => {
    if (configurationBlocked) return;
    setError(null);
    try {
      if (await write()) setChoice(null);
    } catch (caught) {
      setError(caught instanceof Error && caught.message ? caught.message : t(($) => $.execution_squad.save_failed));
    }
  };
  const save = (request: ConfigureProjectSquadRequest) => update(() => onSave({
    ...request,
    ...(request.template_key ? { language: templateLanguageFor(locale) } : {}),
  }));
  const change = () => { setChoice(retainedChoice); setError(null); };

  return (
    <div role="group" aria-label={name} className="space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-2">
          {squad ? <AppLink href={paths.squadDetail(squad.id)} className="block break-words rounded-sm text-body font-medium underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring">{name}</AppLink>
            : <p className="break-words text-body font-medium">{name}</p>}
          {squad?.description?.trim() && <p className="whitespace-pre-line break-words text-caption leading-relaxed text-muted-foreground">{squad.description}</p>}
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span role="status" className={cn("text-caption", available ? "text-success" : "text-muted-foreground")}>
              {pending ? t(($) => $.execution_squad.saving)
                : queryError ? t(($) => $.execution_squad.check_failed)
                  : loading ? t(($) => $.execution_squad.checking) : statusLabels[readiness]}
            </span>
            {isDefault && <span className="rounded bg-muted px-1.5 py-0.5 text-caption text-muted-foreground">{t(($) => $.execution_squad.new_issue_default)}</span>}
          </div>
          {!loading && !queryError && readiness !== "ready" && <p className="text-caption leading-relaxed text-muted-foreground">{hints[readiness]}</p>}
        </div>
        {choice === null && config && <DropdownMenu>
          <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" disabled={pending} aria-label={t(($) => $.execution_squad.actions, { name })} />}>
            <MoreHorizontal className="size-4" aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="max-w-72">
            <DropdownMenuItem disabled={!available || pending} onClick={onCreateIssue}>
              <ArrowRight className="size-3.5" aria-hidden="true" />{t(($) => $.execution_squad.new_issue)}
            </DropdownMenuItem>
            <DropdownMenuItem disabled={isDefault || configurationBlocked || !hasRetainedChoice} onClick={() => { if (onSetDefault) void update(onSetDefault); }}>
              <Check className="size-3.5" aria-hidden="true" />{t(($) => $.execution_squad.set_default)}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem disabled={pending} onClick={change}>{t(($) => $.execution_squad.change)}</DropdownMenuItem>
            <DropdownMenuItem disabled={actualRuntimes.length === 0} onClick={() => setShowRuntimeDetails((shown) => !shown)}>{t(($) => $.execution_squad.runtime_details)}</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" disabled={configurationBlocked} aria-describedby={removeDescriptionId} onClick={() => void save({})}>
              {t(($) => $.execution_squad.remove, { name })}
            </DropdownMenuItem>
            <p id={removeDescriptionId} className="px-1.5 py-1 text-caption leading-relaxed text-muted-foreground">{t(($) => $.execution_squad.remove_description)}</p>
          </DropdownMenuContent>
        </DropdownMenu>}
      </div>

      <div className="flex flex-wrap gap-2">
        {(queryError || paused) && <Button size="sm" variant="outline" disabled={state.fetching} onClick={state.retry}>{t(($) => $.execution_squad.retry_availability)}</Button>}
        {choice === null && readiness === "none" && <Button size="sm" variant="outline" disabled={pending} onClick={change}>{t(($) => $.execution_squad.choose)}</Button>}
        {choice === null && readiness === "needs_runtime" && <Button size="sm" variant="outline" disabled={pending} onClick={change}>{t(($) => $.execution_squad.choose_runtime)}</Button>}
        {(readiness === "needs_runtime" || readiness === "offline" || (readiness === "failed" && config?.error_code === "runtime_unavailable")) && <Button size="sm" variant="outline" onClick={() => navigation.push(paths.runtimes())}>{t(($) => $.execution_squad.connect)}</Button>}
        {choice === null && readiness === "failed" && hasRetainedChoice && <Button size="sm" variant="outline" disabled={configurationBlocked} onClick={() => void save(retainedChoice)}>{t(($) => $.execution_squad.retry)}</Button>}
      </div>

      {showRuntimeDetails && <div className="space-y-1">
        {actualRuntimes.map((runtime) => <p key={runtime.id} className="break-words text-caption text-muted-foreground">{t(($) => $.execution_squad.actual_runtime, { runtime: runtimeDisplayLabel(runtime) })}</p>)}
      </div>}

      {choice !== null && <div className="mt-4 max-w-xl space-y-3">
        <ProjectSquadPicker value={choice} onChange={setChoice} localDaemonIds={localDaemonIds} disabled={configurationBlocked} />
        <p className="text-caption text-muted-foreground">{t(($) => $.execution_squad.future_only)}</p>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={() => void save(choice)} disabled={configurationBlocked}>
            {pending && <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />}
            {t(($) => $.execution_squad.save)}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => { setChoice(null); onCancel?.(); }} disabled={pending}>{t(($) => $.execution_squad.cancel)}</Button>
        </div>
      </div>}
      {error && <p role="alert" className="mt-2 text-caption text-destructive">{error}</p>}
    </div>
  );
}

function useProjectSquadState(wsId: string, project: Project) {
  const { t } = useT("projects");
  const configs = getProjectExecutionSquads(project);
  const { userId, role } = useCurrentMember(wsId);
  // Paused initial requests have isLoading=false and cannot authorize dispatch.
  const membershipQuery = useQuery(memberListOptions(wsId));
  const agentsQuery = useQuery(agentListOptions(wsId));
  const squadsQuery = useQuery(squadListOptions(wsId));
  const runtimesQuery = useQuery(runtimeListOptions(wsId));
  const resourcesQuery = useQuery(projectResourcesOptions(wsId, project.id));
  const { data: templates = [] } = useSquadTemplates();
  const squadIds = [...new Set(configs.flatMap((config) => config.squad_id ? [config.squad_id] : []))];
  const rosterQueries = useQueries({ queries: squadIds.map((id) => squadMemberStatusOptions(wsId, id)) });
  const rosters = new Map(squadIds.map((id, index) => [id, rosterQueries[index]!]));
  const agents = new Map((agentsQuery.data ?? []).filter((agent) => agent.workspace_id === wsId).map((agent) => [agent.id, agent]));
  const runtimes = new Map((runtimesQuery.data ?? []).filter((runtime) => runtime.workspace_id === wsId).map((runtime) => [runtime.id, runtime]));
  const localDaemonIds = projectLocalDaemonIds(resourcesQuery.data ?? []);
  const configurationQueries = [membershipQuery, agentsQuery, squadsQuery, runtimesQuery, resourcesQuery];
  // A deleted roster blocks dispatch, but must not lock replacement or removal.
  const configurationBlocked = configurationQueries.some((query) => query.isPending || query.isError || query.fetchStatus === "paused");

  const describe = (config?: ProjectExecutionSquad) => {
    const squad = squadsQuery.data?.find((item) => item.id === config?.squad_id && item.workspace_id === wsId);
    const leader = squad?.leader_id ? agents.get(squad.leader_id) : undefined;
    // Saved runtime_id records a requested binding, not the actual execution machine.
    const runtime = leader?.runtime_id ? runtimes.get(leader.runtime_id) : undefined;
    const rosterQuery = config?.squad_id ? rosters.get(config.squad_id) : undefined;
    const roster = rosterQuery?.data?.members;
    const canInvoke = !!leader && canAssignAgentToIssue(leader, { userId, role }).allowed &&
      roster?.every((member) => {
        if (member.member_type !== "agent") return true;
        const agent = agents.get(member.member_id);
        return !!agent && canAssignAgentToIssue(agent, { userId, role }).allowed;
      }) === true;
    const readiness = getProjectSquadReadiness(config, {
      squad, leader, runtime, localDaemonIds, canInvoke,
      members: roster, agents: agentsQuery.data, runtimes: runtimesQuery.data,
    });
    const requiredQueries = [...configurationQueries, ...(rosterQuery ? [rosterQuery] : [])];
    const paused = requiredQueries.some((query) => query.fetchStatus === "paused");
    const loading = paused || requiredQueries.some((query) => query.isPending);
    const queryError = requiredQueries.some((query) => query.isError);
    const actualRuntimes = [...new Set(roster?.flatMap((member) => {
      const agent = member.member_type === "agent" ? agents.get(member.member_id) : undefined;
      const actualRuntime = agent?.runtime_id ? runtimes.get(agent.runtime_id) : undefined;
      return actualRuntime ? [actualRuntime] : [];
    }) ?? (runtime ? [runtime] : []))];
    return {
      config, squad, readiness, loading, queryError, paused, actualRuntimes,
      name: squad?.name ?? templates.find((template) => template.key === config?.template_key)?.title ?? t(($) => $.execution_squad.label),
      available: readiness === "ready" && !loading && !queryError,
      fetching: requiredQueries.some((query) => query.isFetching),
      retry: () => { void Promise.allSettled(requiredQueries.map((query) => query.refetch())); },
    };
  };

  return { configs, rows: configs.map(describe), emptyRow: describe(), localDaemonIds, configurationBlocked };
}
