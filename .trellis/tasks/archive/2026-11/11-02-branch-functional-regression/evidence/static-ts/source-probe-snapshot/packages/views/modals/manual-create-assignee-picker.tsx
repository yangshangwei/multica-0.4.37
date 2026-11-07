"use client";

import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import type { IssueAssigneeType, UpdateIssueRequest } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { isAgentRuntimeBound } from "@multica/core/agents";
import { canAssignAgentToIssue } from "@multica/core/permissions";
import { useActorName } from "@multica/core/workspace/hooks";
import { agentListOptions, memberListOptions, squadListOptions } from "@multica/core/workspace/queries";
import {
  captureQuickCreateScope,
  isQuickCreateStoreReady,
  useQuickCreateStore,
  type QuickCreateActorRef,
} from "@multica/core/issues/stores/quick-create-store";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n";
import { QuickCreateActorPicker } from "./quick-create-actor-picker";

const EMPTY_ACTORS: QuickCreateActorRef[] = [];

export function ManualCreateAssigneePicker({
  wsId, assigneeType, assigneeId, onUpdate, triggerRender, open, onOpenChange,
}: {
  wsId: string;
  assigneeType: IssueAssigneeType | null;
  assigneeId: string | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  triggerRender?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useT("issues");
  const { getActorName } = useActorName();
  const userId = useAuthStore((state) => state.user?.id);
  const authStatus = useAuthStore((state) => state.status);
  const membersQuery = useQuery(memberListOptions(wsId));
  const agentsQuery = useQuery(agentListOptions(wsId));
  const squadsQuery = useQuery(squadListOptions(wsId));
  const members = membersQuery.data ?? [];
  const agents = (agentsQuery.data ?? []).filter((agent) => !agent.archived_at);
  const squads = (squadsQuery.data ?? []).filter((squad) => !squad.archived_at);
  const role = members.find((member) => member.user_id === userId)?.role ?? null;

  const preferencesReady = useQuickCreateStore((state) => isQuickCreateStoreReady(state, wsId, userId));
  const resetGeneration = useQuickCreateStore((state) => state.resetGeneration);
  const favorites = useQuickCreateStore((state) => preferencesReady ? state.favoriteActors : EMPTY_ACTORS);
  const recent = useQuickCreateStore((state) => preferencesReady ? state.recentActors : EMPTY_ACTORS);
  const toggleFavorite = useQuickCreateStore((state) => state.toggleFavoriteActor);

  useEffect(() => {
    if (authStatus === "authenticated" && userId && captureQuickCreateScope(wsId, userId)
      && !isQuickCreateStoreReady(useQuickCreateStore.getState(), wsId, userId)) {
      void useQuickCreateStore.persist.rehydrate();
    }
  }, [wsId, userId, authStatus, resetGeneration]);

  const disabledReasons = new Map<string, string>();
  const runnableAgentIds = new Set<string>();
  for (const agent of agents) {
    const decision = canAssignAgentToIssue(agent, { userId: userId ?? null, role });
    const runtimeBound = isAgentRuntimeBound(agent);
    if (runtimeBound) runnableAgentIds.add(agent.id);
    if (!decision.allowed) disabledReasons.set(`agent:${agent.id}`, decision.message);
    else if (!runtimeBound) disabledReasons.set(`agent:${agent.id}`, t(($) => $.pickers.assignee.agent_runtime_required));
  }
  for (const squad of squads) {
    if (!runnableAgentIds.has(squad.leader_id)) {
      disabledReasons.set(`squad:${squad.id}`, t(($) => $.pickers.assignee.squad_runtime_required));
    }
  }

  const actor: QuickCreateActorRef | null = assigneeId && (assigneeType === "agent" || assigneeType === "squad")
    ? { type: assigneeType, id: assigneeId } : null;

  return <QuickCreateActorPicker
    actor={actor}
    visibleAgents={agents}
    visibleSquads={squads}
    favoriteActors={favorites}
    recentActors={recent}
    preferencesReady={preferencesReady}
    onToggleFavorite={(ref) => {
      const scope = captureQuickCreateScope(wsId, userId);
      if (scope) toggleFavorite(ref, scope);
    }}
    onPick={(ref) => {
      if (!disabledReasons.has(`${ref.type}:${ref.id}`)) {
        onUpdate({ assignee_type: ref.type, assignee_id: ref.id });
      }
    }}
    triggerRender={triggerRender}
    trigger={assigneeType && assigneeId ? <>
      <ActorAvatar actorType={assigneeType} actorId={assigneeId} size="sm" enableHoverCard showStatusDot />
      <span className="truncate">{getActorName(assigneeType, assigneeId)}</span>
    </> : <span className="text-muted-foreground">{t(($) => $.pickers.assignee.trigger_unassigned)}</span>}
    open={open}
    onOpenChange={onOpenChange}
    agentState={{
      pending: agentsQuery.isPending || membersQuery.isPending,
      error: agentsQuery.isError || membersQuery.isError,
      hasData: agentsQuery.data !== undefined && membersQuery.data !== undefined,
      onRetry: () => { void agentsQuery.refetch(); void membersQuery.refetch(); },
    }}
    squadState={{
      pending: squadsQuery.isPending,
      error: squadsQuery.isError,
      hasData: squadsQuery.data !== undefined,
      onRetry: () => { void squadsQuery.refetch(); },
    }}
    assignment={{
      members,
      selectedMemberId: assigneeType === "member" ? assigneeId : null,
      onPickMember: (id) => onUpdate({ assignee_type: "member", assignee_id: id }),
      onClear: () => onUpdate({ assignee_type: null, assignee_id: null }),
      disabledReasons,
      privateAgentIds: new Set(agents.filter((agent) => agent.visibility === "private").map((agent) => agent.id)),
      memberState: {
        pending: membersQuery.isPending,
        error: membersQuery.isError,
        hasData: membersQuery.data !== undefined,
        onRetry: () => { void membersQuery.refetch(); },
      },
    }}
  />;
}
