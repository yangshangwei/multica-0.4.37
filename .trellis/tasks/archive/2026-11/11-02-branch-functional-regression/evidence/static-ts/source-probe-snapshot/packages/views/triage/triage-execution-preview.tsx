"use client";

import { useQuery } from "@tanstack/react-query";
import {
  canAssignAgentToIssue,
  useCurrentMember,
} from "@multica/core/permissions";
import { isAgentRuntimeBound } from "@multica/core/agents";
import {
  agentListOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";
import {
  runtimeDisplayLabel,
  runtimeListOptions,
} from "@multica/core/runtimes";
import {
  projectDetailOptions,
  projectResourcesOptions,
} from "@multica/core/projects";
import type { ProjectResource, TriageFields } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { localDirectoryLabel } from "../projects/components/local-directory-label";
import { useT } from "../i18n";
import { TRIAGE_CONTROL } from "./triage-ui";

type ExecutionReadiness =
  | "ready"
  | "checking"
  | "check_failed"
  | "choose_executor"
  | "executor_unavailable"
  | "invoke_denied"
  | "runtime_unavailable"
  | "project_unavailable"
  | "resource_unavailable";

function validResource(
  resource: ProjectResource,
  wsId: string,
  projectId: string,
) {
  if (resource.workspace_id !== wsId || resource.project_id !== projectId)
    return false;
  const ref: Record<string, unknown> = { ...resource.resource_ref };
  switch (resource.resource_type) {
    case "github_repo":
      return typeof ref.url === "string" && ref.url.trim().length > 0;
    case "local_directory":
      return (
        typeof ref.daemon_id === "string" &&
        ref.daemon_id.trim().length > 0 &&
        typeof ref.local_path === "string" &&
        ref.local_path.trim().length > 0 &&
        (ref.execution_mode === undefined ||
          ref.execution_mode === "in_place" ||
          ref.execution_mode === "worktree")
      );
    default:
      return false;
  }
}

/** Read-only prerequisite composition. No trigger, mutation or preparation endpoint runs. */
export function useTriageExecutionPrerequisites(
  wsId: string,
  fields: TriageFields,
  enabled: boolean,
) {
  const membership = useCurrentMember(wsId);
  const agents = useQuery({
    ...agentListOptions(wsId),
    enabled,
    staleTime: 15_000,
  });
  const squads = useQuery({
    ...squadListOptions(wsId),
    enabled: enabled && fields.assignee_type === "squad",
    staleTime: 15_000,
  });
  const runtimes = useQuery({
    ...runtimeListOptions(wsId),
    enabled,
    staleTime: 15_000,
    refetchInterval: enabled ? 15_000 : false,
  });
  const projectId = fields.project_id ?? "";
  const project = useQuery({
    ...projectDetailOptions(wsId, projectId),
    enabled: enabled && !!projectId,
    staleTime: 15_000,
  });
  const resources = useQuery({
    ...projectResourcesOptions(wsId, projectId),
    enabled: enabled && !!projectId,
    staleTime: 15_000,
  });
  const squad =
    fields.assignee_type === "squad"
      ? squads.data?.find((entry) => entry.id === fields.assignee_id)
      : undefined;
  const agentId =
    fields.assignee_type === "squad"
      ? squad?.leader_id
      : fields.assignee_type === "agent"
        ? fields.assignee_id
        : null;
  const agent = agents.data?.find((entry) => entry.id === agentId);
  const runtime = runtimes.data?.find(
    (entry) => entry.id === agent?.runtime_id,
  );
  const activeQueries = [
    agents,
    runtimes,
    ...(fields.assignee_type === "squad" ? [squads] : []),
    ...(projectId ? [project, resources] : []),
  ];
  let readiness: ExecutionReadiness = "ready";
  if (
    !fields.assignee_id ||
    (fields.assignee_type !== "agent" && fields.assignee_type !== "squad")
  )
    readiness = "choose_executor";
  else if (activeQueries.some((query) => query.isError))
    readiness = "check_failed";
  else if (
    membership.isLoading ||
    activeQueries.some(
      (query) =>
        query.data === undefined || query.isPending || query.isPlaceholderData,
    )
  )
    readiness = "checking";
  else if (
    !membership.userId ||
    !membership.role ||
    !agent ||
    agent.workspace_id !== wsId ||
    agent.archived_at ||
    (fields.assignee_type === "squad" &&
      (!squad || squad.workspace_id !== wsId || squad.archived_at))
  )
    readiness = "executor_unavailable";
  else if (
    !canAssignAgentToIssue(agent, {
      userId: membership.userId,
      role: membership.role,
    }).allowed
  )
    readiness = "invoke_denied";
  else if (
    !isAgentRuntimeBound(agent) ||
    (runtime
      ? runtime.workspace_id !== wsId ||
        runtime.status !== "online" ||
        !runtime.owner_id
      : agent.runtime_availability !== "online")
  )
    readiness = "runtime_unavailable";
  else if (
    projectId &&
    (!project.data ||
      project.data.id !== projectId ||
      project.data.workspace_id !== wsId)
  )
    readiness = "project_unavailable";
  else if (
    projectId &&
    (resources.data ?? []).some(
      (resource) => !validResource(resource, wsId, projectId),
    )
  )
    readiness = "resource_unavailable";
  return {
    canExecute: enabled && readiness === "ready",
    readiness,
    agent,
    squad,
    runtime,
    project: projectId ? project.data : undefined,
    resources: projectId ? (resources.data ?? []) : [],
    hasProject: !!projectId,
    resourcesKnown: !projectId || Array.isArray(resources.data),
    retry: () => {
      for (const query of activeQueries) void query.refetch();
    },
    isFetching: activeQueries.some((query) => query.isFetching),
  };
}

export function TriageExecutionSummary({
  prerequisites,
}: {
  prerequisites: ReturnType<typeof useTriageExecutionPrerequisites>;
}) {
  const { t } = useT("triage");
  const { agent, squad, runtime, project, resources, readiness } =
    prerequisites;
  return (
    <section
      className="space-y-3 rounded-md border border-surface-border p-3"
      aria-label={t(($) => $.execution_summary)}
    >
      <h3 className="text-body font-medium">{t(($) => $.execution_summary)}</h3>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-caption">
        <dt className="text-muted-foreground">
          {t(($) => $.execution_executor)}
        </dt>
        <dd className="break-words">
          {agent?.name ?? t(($) => $.unassigned)}
          {squad && (
            <span className="block text-muted-foreground">
              {t(($) => $.execution_squad_leader, { name: squad.name })}
            </span>
          )}
        </dd>
        <dt className="text-muted-foreground">
          {t(($) => $.execution_runtime)}
        </dt>
        <dd className="break-words">
          {runtime
            ? runtimeDisplayLabel(runtime)
            : agent?.runtime_availability === "online"
              ? t(($) => $.execution_managed_runtime)
              : t(($) => $.unknown)}
        </dd>
        <dt className="text-muted-foreground">{t(($) => $.project)}</dt>
        <dd className="break-words">
          {project?.title ??
            (prerequisites.hasProject ? t(($) => $.unknown) : t(($) => $.none))}
        </dd>
      </dl>
      <div className="space-y-2">
        <h4 className="text-caption font-medium">
          {t(($) => $.execution_resources)}
        </h4>
        {!prerequisites.resourcesKnown ? (
          <p className="text-caption text-muted-foreground">
            {t(
              ($) =>
                $.execution_readiness[
                  readiness === "check_failed" ? "check_failed" : "checking"
                ],
            )}
          </p>
        ) : resources.length === 0 ? (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.execution_no_project_resources)}
          </p>
        ) : (
          <ul className="space-y-2">
            {resources.map((resource) => {
              const ref: Record<string, unknown> = { ...resource.resource_ref };
              if (
                resource.resource_type === "github_repo" &&
                typeof ref.url === "string"
              )
                return (
                  <li className="min-w-0 text-caption" key={resource.id}>
                    <span className="font-medium">
                      {resource.label || t(($) => $.execution_repository)}
                    </span>
                    <span className="block break-all text-muted-foreground">
                      {ref.url}
                      {typeof ref.ref === "string" && ` · ${ref.ref}`}
                    </span>
                  </li>
                );
              if (
                resource.resource_type === "local_directory" &&
                typeof ref.local_path === "string" &&
                typeof ref.daemon_id === "string"
              )
                return (
                  <li className="min-w-0 text-caption" key={resource.id}>
                    <span className="font-medium">
                      {localDirectoryLabel({
                        label: resource.label,
                        resource_ref: {
                          local_path: ref.local_path,
                          daemon_id: ref.daemon_id,
                          label:
                            typeof ref.label === "string"
                              ? ref.label
                              : undefined,
                        },
                      })}
                    </span>
                    <span className="block break-all text-muted-foreground">
                      {ref.local_path}
                    </span>
                    <span className="block text-muted-foreground">
                      {ref.execution_mode === "worktree"
                        ? t(($) => $.execution_worktree)
                        : t(($) => $.execution_in_place)}{" "}
                      ·{" "}
                      {runtime?.daemon_id === ref.daemon_id
                        ? t(($) => $.execution_current_machine)
                        : t(($) => $.execution_configured_machine)}
                    </span>
                  </li>
                );
              return (
                <li key={resource.id} className="text-caption text-destructive">
                  {resource.label ?? t(($) => $.execution_resource_unavailable)}
                </li>
              );
            })}
          </ul>
        )}
      </div>
      <p
        role="status"
        className={`text-caption ${readiness === "ready" || readiness === "checking" ? "text-muted-foreground" : "text-destructive"}`}
      >
        {t(($) => $.execution_readiness[readiness])}
      </p>
      {(readiness === "check_failed" ||
        readiness === "runtime_unavailable" ||
        readiness === "resource_unavailable") && (
        <Button
          variant="outline"
          className={TRIAGE_CONTROL}
          onClick={prerequisites.retry}
          disabled={prerequisites.isFetching}
        >
          {t(($) => $.retry)}
        </Button>
      )}
    </section>
  );
}
