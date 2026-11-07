"use client";

import React, { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { chatPinnedAgentsOptions, chatSessionsOptions } from "@multica/core/chat/queries";
import { usePinChatAgent, useUnpinChatAgent } from "@multica/core/chat/mutations";
import { squadListOptions } from "@multica/core/workspace/queries";
import { QuickCreateActorPicker } from "../../modals/quick-create-actor-picker";
import { Plus } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import type { Agent } from "@multica/core/types";
import { isAgentRuntimeBound } from "@multica/core/agents";
import { toast } from "sonner";
import { useT } from "../../i18n";

/** Chat owns eligibility and server pins; discovery is shared with creation. */
export function AgentPicker({
  agents, currentAgentId, onSelect, trigger, triggerRender,
  side = "bottom", align = "start",
}: {
  agents: Agent[];
  userId: string | undefined;
  currentAgentId?: string;
  onSelect: (agent: Agent) => void;
  trigger: React.ReactNode;
  triggerRender: React.ReactElement;
  side?: "top" | "bottom";
  align?: "start" | "center" | "end";
}) {
  const { t } = useT("chat");
  const wsId = useWorkspaceId();
  const pinsQuery = useQuery(chatPinnedAgentsOptions(wsId));
  const sessionsQuery = useQuery(chatSessionsOptions(wsId));
  const squadsQuery = useQuery(squadListOptions(wsId));
  const pin = usePinChatAgent();
  const unpin = useUnpinChatAgent();
  const pinned = pinsQuery.data ?? [];
  const favoriteActors = pinned.map((item) => ({ type: "agent" as const, id: item.agent_id }));
  const recentActors = useMemo(() => (sessionsQuery.data ?? [])
    .filter((session) => session.status !== "archived")
    .sort((a, b) => new Date(b.last_message?.created_at ?? b.updated_at).getTime()
      - new Date(a.last_message?.created_at ?? a.updated_at).getTime())
    .map((session) => ({ type: "agent" as const, id: session.agent_id })), [sessionsQuery.data]);
  const disabledReasons = new Map(agents.filter((agent) => !isAgentRuntimeBound(agent))
    .map((agent) => [`agent:${agent.id}`, t(($) => $.window.agent_needs_runtime_hint)]));

  return <QuickCreateActorPicker
    actor={currentAgentId ? { type: "agent", id: currentAgentId } : null}
    visibleAgents={agents}
    visibleSquads={(squadsQuery.data ?? []).filter((squad) => !squad.archived_at)}
    favoriteActors={favoriteActors}
    recentActors={recentActors}
    preferencesReady={pinsQuery.data !== undefined}
    onToggleFavorite={(ref) => {
      if (ref.type !== "agent" || pin.isPending || unpin.isPending) return;
      const isPinned = pinned.some((item) => item.agent_id === ref.id);
      if (!isPinned && (pinned.length >= 5 || disabledReasons.has(`agent:${ref.id}`))) return;
      const mutation = isPinned ? unpin : pin;
      mutation.mutate(ref.id, { onError: () => toast.error(t(($) => $.window.pin_failed)) });
    }}
    onPick={(ref) => {
      const agent = ref.type === "agent" ? agents.find((candidate) => candidate.id === ref.id) : undefined;
      if (agent && isAgentRuntimeBound(agent)) onSelect(agent);
    }}
    squadState={{
      pending: squadsQuery.isPending,
      error: squadsQuery.isError,
      hasData: squadsQuery.data !== undefined,
      onRetry: () => { void squadsQuery.refetch(); },
    }}
    chat={{ hint: t(($) => $.window.picker_hint), disabledReasons, canPin: pinned.length < 5, favoritesPending: pin.isPending || unpin.isPending }}
    side={side}
    align={align}
    trigger={trigger}
    triggerRender={triggerRender}
  />;
}

/**
 * "New chat" ⊕ button. Per the Chat V2 design, starting a new chat is where the
 * agent is chosen — so this opens an AgentPicker and reports the pick via
 * `onStart`. No agent is pre-checked: a new chat has no "current" agent yet.
 * Shortcuts: with a single available agent it starts immediately (no point
 * showing a one-item menu); with none it still fires `onStart(null)` so the
 * surface shows its no-agent empty state.
 */
export function NewChatButton({
  agents,
  userId,
  onStart,
  side = "bottom",
}: {
  agents: Agent[];
  userId: string | undefined;
  onStart: (agent: Agent | null) => void;
  side?: "top" | "bottom";
}) {
  const { t } = useT("chat");
  const label = t(($) => $.window.new_chat_tooltip);

  if (agents.length <= 1) {
    const only = agents[0] ?? null;
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              className="rounded-full text-muted-foreground"
              aria-label={label}
              onClick={() => {
                if (only && !isAgentRuntimeBound(only)) {
                  toast.error(t(($) => $.input.runtime_required_toast));
                  return;
                }
                onStart(only);
              }}
            />
          }
        >
          <Plus />
        </TooltipTrigger>
        <TooltipContent side={side === "top" ? "top" : "bottom"}>{label}</TooltipContent>
      </Tooltip>
    );
  }

  return (
    <AgentPicker
      agents={agents}
      userId={userId}
      onSelect={(agent) => onStart(agent)}
      side={side}
      align="start"
      triggerRender={
        <Button
          variant="ghost"
          size="icon-sm"
          className="rounded-full text-muted-foreground"
          aria-label={label}
        />
      }
      trigger={<Plus />}
    />
  );
}
