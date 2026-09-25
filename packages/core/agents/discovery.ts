import type { Agent, AgentRoleTemplate, Squad, SquadMember, SquadTemplate } from "../types";

export type AgentRoleKind = "coordinator" | "specialist" | "other";

export interface AgentRoleMetadata {
  templateKey: string;
  title: string;
  kind: Exclude<AgentRoleKind, "other">;
}

export interface AgentSquadMembership {
  squadId: string;
  name: string;
  isLeader: boolean;
}

export function buildAgentRoleMetadata(
  roles: readonly AgentRoleTemplate[],
  squads: readonly SquadTemplate[],
): ReadonlyMap<string, AgentRoleMetadata> {
  const metadata = new Map<string, AgentRoleMetadata>();
  for (const role of roles) {
    if (!role.key) continue;
    metadata.set(role.key, {
      templateKey: role.key,
      title: role.title || role.name,
      kind: "specialist",
    });
  }
  for (const squad of squads) {
    for (const member of squad.members ?? []) {
      if (!member.template_key || metadata.has(member.template_key)) continue;
      metadata.set(member.template_key, {
        templateKey: member.template_key,
        title: member.title || member.name,
        kind: "specialist",
      });
    }
    const leader = squad.leader;
    if (!leader?.template_key) continue;
    metadata.set(leader.template_key, {
      templateKey: leader.template_key,
      title: leader.title || leader.name,
      kind: "coordinator",
    });
  }
  return metadata;
}

export function resolveAgentRole(
  agent: Pick<Agent, "template_key">,
  metadata: ReadonlyMap<string, AgentRoleMetadata>,
): AgentRoleMetadata | null {
  // A template records provenance, not the current agent's editable capability
  // or autonomy policy. Names and permission levels cannot establish a role.
  return agent.template_key ? metadata.get(agent.template_key) ?? null : null;
}

export function buildAgentSquadMemberships(
  squads: readonly Squad[],
  resolvedMembers?: ReadonlyMap<string, readonly SquadMember[]>,
): {
  byAgent: ReadonlyMap<string, readonly AgentSquadMembership[]>;
  incompleteSquadIds: ReadonlySet<string>;
} {
  const byAgent = new Map<string, AgentSquadMembership[]>();
  const incompleteSquadIds = new Set<string>();
  for (const squad of squads) {
    if (squad.archived_at) continue;
    const members = resolvedMembers?.get(squad.id);
    const agentIds = new Set(squad.agent_member_ids ?? members
      ?.filter((member) => member.squad_id === squad.id && member.member_type === "agent")
      .map((member) => member.member_id) ?? []);
    if (squad.agent_member_ids === undefined && members === undefined) {
      incompleteSquadIds.add(squad.id);
    }
    // Legacy list previews are truncated. Only the explicit leader is certain
    // until a full roster has been fetched for a selected squad.
    if (squad.leader_id) agentIds.add(squad.leader_id);
    for (const agentId of agentIds) {
      const memberships = byAgent.get(agentId) ?? [];
      memberships.push({
        squadId: squad.id,
        name: squad.name,
        isLeader: squad.leader_id === agentId,
      });
      byAgent.set(agentId, memberships);
    }
  }
  return { byAgent, incompleteSquadIds };
}
