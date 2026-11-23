import { isRuntimeUsableForUser } from "../runtimes/access";
import { isAgentRuntimeBound } from "../agents/runtime-binding";
import type { Agent, AgentRuntime, ConfigureProjectSquadRequest, Project, ProjectExecutionSquad, ProjectResource, Squad } from "../types";

export const DEFAULT_PROJECT_SQUAD_TEMPLATE_KEY = "feature-delivery";

export function getProjectExecutionSquads(project: Pick<Project, "execution_squad" | "execution_squads">): ProjectExecutionSquad[] {
  return project.execution_squads ??
    (project.execution_squad && project.execution_squad.state !== "none" ? [project.execution_squad] : []);
}

export function projectSquadSelection(config: ProjectExecutionSquad): ConfigureProjectSquadRequest {
  if (config.template_key) {
    return { template_key: config.template_key, ...(config.runtime_id ? { runtime_id: config.runtime_id } : {}) };
  }
  return config.squad_id ? { squad_id: config.squad_id } : {};
}

export function replaceProjectSquadSelection(
  configs: readonly ProjectExecutionSquad[],
  index: number,
  request: ConfigureProjectSquadRequest,
): ConfigureProjectSquadRequest[] {
  const selections = configs.map(projectSquadSelection);
  const existingIndex = configs.findIndex((config, position) => position !== index &&
    (request.template_key ? config.template_key === request.template_key
      : !!request.squad_id && config.squad_id === request.squad_id));
  if (existingIndex !== -1) {
    // Keep a configured template's provenance when its instance is picked.
    if (request.template_key) selections[existingIndex] = request;
    selections.splice(index, 1);
  } else if (request.template_key || request.squad_id) {
    selections.splice(index, 1, request);
  } else {
    selections.splice(index, 1);
  }
  return selections;
}

export interface ProjectRuntimeSelectionOptions {
  workspaceId: string;
  userId?: string | null;
  daemonId?: string | null;
  daemonIds?: readonly string[];
  preferredRuntimeId?: string | null;
}

export function eligibleProjectRuntimes(
  runtimes: readonly AgentRuntime[],
  options: ProjectRuntimeSelectionOptions,
): AgentRuntime[] {
  // Automatic provisioning must wait for the authenticated identity even
  // though the generic runtime picker can show options while auth loads.
  if (!options.userId || !options.workspaceId) return [];
  const machines = options.daemonId ? [options.daemonId] : options.daemonIds;
  return runtimes.filter((runtime) =>
    runtime.workspace_id === options.workspaceId &&
    isRuntimeUsableForUser(runtime, options.userId ?? null) &&
    (!machines?.length || (runtime.daemon_id != null && machines.includes(runtime.daemon_id))),
  );
}

export function selectProjectRuntime(
  runtimes: readonly AgentRuntime[],
  options: ProjectRuntimeSelectionOptions,
): AgentRuntime | null {
  const online = eligibleProjectRuntimes(runtimes, options).filter((runtime) => runtime.status === "online");
  const preferred = online.find((runtime) => runtime.id === options.preferredRuntimeId);
  return preferred ?? (online.length === 1 ? online[0]! : null);
}

export function projectLocalDaemonIds(resources: readonly ProjectResource[]): string[] {
  const ids = new Set<string>();
  for (const resource of resources) {
    const ref = resource.resource_ref;
    if (resource.resource_type === "local_directory" && ref && typeof ref === "object" &&
      "daemon_id" in ref && typeof ref.daemon_id === "string" && ref.daemon_id.trim()) {
      ids.add(ref.daemon_id);
    }
  }
  return [...ids];
}

export type ProjectSquadReadiness =
  | "none" | "needs_runtime" | "failed" | "unavailable" | "offline" | "wrong_machine" | "ready";

export function getProjectSquadReadiness(
  config: ProjectExecutionSquad | null | undefined,
  targets: {
    squad?: Squad | null;
    leader?: Agent | null;
    runtime?: AgentRuntime | null;
    canInvoke: boolean;
    localDaemonIds?: readonly string[];
    members?: readonly { member_type: "agent" | "member"; member_id: string; status?: string | null }[];
    agents?: readonly Agent[];
    runtimes?: readonly AgentRuntime[];
  },
): ProjectSquadReadiness {
  if (!config || config.state === "none") return "none";
  if (config.state === "needs_runtime") return "needs_runtime";
  if (config.state === "failed") return "failed";
  if (config.state !== "configured") return "unavailable";
  const { squad, leader, runtime, canInvoke, localDaemonIds, members, agents, runtimes } = targets;
  if (!squad || !leader || !runtime || canInvoke !== true ||
    !config.squad_id || squad.id !== config.squad_id || squad.archived_at ||
    leader.id !== squad.leader_id || leader.archived_at ||
    leader.workspace_id !== squad.workspace_id || runtime.workspace_id !== squad.workspace_id ||
    !isAgentRuntimeBound(leader) || runtime.id !== leader.runtime_id) return "unavailable";
  if (!members?.some((member) => member.member_type === "agent" && member.member_id === leader.id)) {
    return "unavailable";
  }
  let offline = false;
  for (const member of members) {
    if (member.member_type !== "agent") continue;
    const agent = member.member_id === leader.id ? leader : agents?.find((item) => item.id === member.member_id);
    if (!agent || agent.archived_at || agent.workspace_id !== squad.workspace_id || !isAgentRuntimeBound(agent)) {
      return "unavailable";
    }
    const actualRuntime = agent.id === leader.id ? runtime : runtimes?.find((item) => item.id === agent.runtime_id);
    if (!actualRuntime || actualRuntime.workspace_id !== squad.workspace_id) return "unavailable";
    if (localDaemonIds?.length && (!actualRuntime.daemon_id || !localDaemonIds.includes(actualRuntime.daemon_id))) {
      return "wrong_machine";
    }
    if (member.status === "archived" ||
      (member.status != null && !["working", "idle", "offline", "unstable"].includes(member.status))) return "unavailable";
    if (actualRuntime.status !== "online" || member.status === "offline" || member.status === "unstable") offline = true;
  }
  return offline ? "offline" : "ready";
}
