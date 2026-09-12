"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, Loader2, Users } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import { useWorkspacePaths } from "@multica/core/paths";
import { canAssignAgentToIssue, useCurrentMember } from "@multica/core/permissions";
import { getProjectSquadReadiness, projectLocalDaemonIds, projectResourcesOptions } from "@multica/core/projects";
import { useConfigureProjectSquad } from "@multica/core/projects/mutations";
import { runtimeDisplayLabel, runtimeListOptions } from "@multica/core/runtimes";
import type { ConfigureProjectSquadRequest, Project } from "@multica/core/types";
import { agentListOptions, memberListOptions, squadListOptions, squadMemberStatusOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { templateLanguageFor, useSquadTemplates } from "../../agents/create/use-role-templates";
import { useLocale, useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { ProjectSquadPicker } from "./project-squad-picker";

export function ProjectSquadSection({ project }: { project: Project }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const { userId, role } = useCurrentMember(wsId);
  // Read the query status as well as the canonical membership context: a
  // paused initial request has isLoading=false, but is not ready to authorize.
  const membershipQuery = useQuery(memberListOptions(wsId));
  const agentsQuery = useQuery(agentListOptions(wsId));
  const squadsQuery = useQuery(squadListOptions(wsId));
  const runtimesQuery = useQuery(runtimeListOptions(wsId));
  const resourcesQuery = useQuery(projectResourcesOptions(wsId, project.id));
  const { data: templates = [] } = useSquadTemplates();
  const configure = useConfigureProjectSquad(wsId);
  const [choice, setChoice] = useState<ConfigureProjectSquadRequest | null>(null);
  const [error, setError] = useState<string | null>(null);
  const config = project.execution_squad;
  const rosterQuery = useQuery(squadMemberStatusOptions(wsId, config?.squad_id ?? ""));
  const squad = squadsQuery.data?.find((item) => item.id === config?.squad_id && item.workspace_id === wsId);
  const leader = agentsQuery.data?.find((item) => item.id === squad?.leader_id && item.workspace_id === wsId);
  // A reused leader can be bound elsewhere. The requested runtime in the
  // saved configuration never proves where this squad will actually execute.
  const runtime = runtimesQuery.data?.find((item) => item.id === leader?.runtime_id && item.workspace_id === wsId);
  const roster = rosterQuery.data?.members;
  const canInvoke = !!leader && canAssignAgentToIssue(leader, { userId, role }).allowed &&
    roster?.every((member) => {
      if (member.member_type !== "agent") return true;
      const agent = agentsQuery.data?.find((item) => item.id === member.member_id && item.workspace_id === wsId);
      return !!agent && canAssignAgentToIssue(agent, { userId, role }).allowed;
    }) === true;
  const localDaemonIds = projectLocalDaemonIds(resourcesQuery.data ?? []);
  const readiness = getProjectSquadReadiness(config, {
    squad, leader, runtime, localDaemonIds, canInvoke,
    members: roster, agents: agentsQuery.data, runtimes: runtimesQuery.data,
  });
  const configurationQueries = [membershipQuery, agentsQuery, squadsQuery, runtimesQuery, resourcesQuery];
  const requiredQueries = [...configurationQueries,
    ...(config?.squad_id ? [rosterQuery] : [])];
  const paused = requiredQueries.some((query) => query.fetchStatus === "paused");
  const loading = paused || requiredQueries.some((query) => query.isPending);
  const queryError = requiredQueries.some((query) => query.isError);
  // Replacement uses current workspace choices and resources. The saved
  // squad's roster may already be deleted, so its failure cannot lock recovery.
  const configurationBlocked = configure.isPending || configurationQueries.some(
    (query) => query.isPending || query.isError || query.fetchStatus === "paused",
  );
  const canDispatch = readiness === "ready" && !loading && !queryError && !configure.isPending;
  const template = templates.find((item) => item.key === config?.template_key);
  const name = squad?.name ?? template?.title ?? t(($) => $.execution_squad.label);
  const retainedChoice: ConfigureProjectSquadRequest = config?.template_key
    ? { template_key: config.template_key, ...(config.runtime_id ? { runtime_id: config.runtime_id } : {}) }
    : config?.squad_id ? { squad_id: config.squad_id } : {};
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

  const save = async (request: ConfigureProjectSquadRequest) => {
    if (configurationBlocked) return;
    setError(null);
    try {
      await configure.mutateAsync({
        id: project.id,
        ...request,
        ...(request.template_key ? { language: templateLanguageFor(locale) } : {}),
      });
      setChoice(null);
    } catch (caught) {
      setError(caught instanceof Error && caught.message ? caught.message : t(($) => $.execution_squad.save_failed));
    }
  };

  return (
    <section aria-label={t(($) => $.execution_squad.label)} className="shrink-0 border-b px-4 py-3 sm:px-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <Users className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="break-words text-body font-medium">{name}</span>
            <span role="status" className={cn("text-caption", canDispatch ? "text-success" : "text-muted-foreground")}>
              {configure.isPending ? t(($) => $.execution_squad.saving)
                : queryError ? t(($) => $.execution_squad.check_failed)
                  : loading ? t(($) => $.execution_squad.checking) : statusLabels[readiness]}
            </span>
          </div>
          {!loading && !queryError && <p className="max-w-prose text-caption leading-relaxed text-muted-foreground">{hints[readiness]}</p>}
          {runtime && <p className="break-words text-caption text-muted-foreground">{t(($) => $.execution_squad.actual_runtime, { runtime: runtimeDisplayLabel(runtime) })}</p>}
        </div>
        {(queryError || paused) && <Button size="sm" variant="outline" disabled={requiredQueries.some((query) => query.isFetching)} onClick={() => {
          void Promise.allSettled(requiredQueries.map((query) => query.refetch()));
        }}>{t(($) => $.execution_squad.retry_availability)}</Button>}
        {choice === null && <div className="flex flex-wrap items-center gap-1.5">
          <Button size="sm" variant="ghost" disabled={configure.isPending} onClick={() => { setChoice(retainedChoice); setError(null); }}>
            {readiness === "none" ? t(($) => $.execution_squad.choose) : readiness === "needs_runtime" ? t(($) => $.execution_squad.choose_runtime) : t(($) => $.execution_squad.change)}
          </Button>
          {(readiness === "needs_runtime" || readiness === "offline" || (readiness === "failed" && config?.error_code === "runtime_unavailable")) && <Button size="sm" variant="outline" onClick={() => navigation.push(paths.runtimes())}>{t(($) => $.execution_squad.connect)}</Button>}
          {readiness === "failed" && hasRetainedChoice && <Button size="sm" variant="outline" disabled={configurationBlocked} onClick={() => void save(retainedChoice)}>{t(($) => $.execution_squad.retry)}</Button>}
          {config?.squad_id && <Button
            size="sm"
            disabled={!canDispatch}
            onClick={() => {
              if (!canDispatch || !config.squad_id) return;
              useModalStore.getState().open("create-issue", {
                project_id: project.id, assignee_type: "squad", assignee_id: config.squad_id, status: "todo",
              });
            }}
          >
            {t(($) => $.execution_squad.dispatch)}<ArrowRight className="size-3.5" aria-hidden="true" />
          </Button>}
        </div>}
      </div>

      {choice !== null && <div className="mt-4 max-w-xl space-y-3">
        <ProjectSquadPicker value={choice} onChange={setChoice} localDaemonIds={localDaemonIds} disabled={configurationBlocked} />
        <p className="text-caption text-muted-foreground">{t(($) => $.execution_squad.future_only)}</p>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={() => void save(choice)} disabled={configurationBlocked}>
            {configure.isPending && <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />}
            {t(($) => $.execution_squad.save)}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setChoice(null)} disabled={configure.isPending}>{t(($) => $.execution_squad.cancel)}</Button>
          {squad && <Button size="sm" variant="ghost" onClick={() => navigation.push(paths.squadDetail(squad.id))}>{t(($) => $.execution_squad.open_squad)}</Button>}
        </div>
      </div>}
      {error && <p role="alert" className="mt-2 text-caption text-destructive">{error}</p>}
    </section>
  );
}
