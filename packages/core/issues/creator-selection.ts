import type { QuickCreateActorRef } from "./stores/quick-create-store";

export interface CreatorSelectionInput {
  callers: readonly QuickCreateActorRef[];
  draftActor: QuickCreateActorRef | null;
  defaultActor: QuickCreateActorRef | null;
  lastActor: QuickCreateActorRef | null;
  agents: readonly { id: string }[];
  squads: readonly { id: string }[];
  preferencesReady?: boolean;
  agentsKnown: boolean;
  squadsKnown: boolean;
}

/** Unknown high-priority identities must resolve before a fallback can win. */
export function resolveQuickCreateCreator(input: CreatorSelectionInput): QuickCreateActorRef | null {
  const candidates = [...input.callers, input.draftActor, input.defaultActor, input.lastActor];
  for (const [index, candidate] of candidates.entries()) {
    if (index === input.callers.length + 1 && input.preferencesReady === false) return null;
    if (!candidate || (candidate.type !== "agent" && candidate.type !== "squad") || typeof candidate.id !== "string" || !candidate.id.trim()) continue;
    const known = candidate.type === "agent" ? input.agentsKnown : input.agentsKnown && input.squadsKnown;
    if (!known) return null;
    const available = candidate.type === "agent" ? input.agents : input.squads;
    if (available.some((actor) => actor.id === candidate.id)) return candidate;
  }
  return input.agentsKnown && input.agents[0] ? { type: "agent", id: input.agents[0].id } : null;
}
