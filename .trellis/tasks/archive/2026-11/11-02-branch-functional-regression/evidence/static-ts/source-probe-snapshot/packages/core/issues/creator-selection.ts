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
  /** Remembered fallbacks (default, last accepted, first agent) skip an actor
   *  this rejects. Callers and the draft are explicit picks and stay honored,
   *  so the panel can say why they cannot create instead of swapping them. */
  canCreate?: (actor: QuickCreateActorRef) => boolean;
}

/** Unknown high-priority identities must resolve before a fallback can win. */
export function resolveQuickCreateCreator(input: CreatorSelectionInput): QuickCreateActorRef | null {
  const candidates = [...input.callers, input.draftActor, input.defaultActor, input.lastActor];
  const usable = (actor: QuickCreateActorRef) => input.canCreate?.(actor) !== false;
  for (const [index, candidate] of candidates.entries()) {
    if (index === input.callers.length + 1 && input.preferencesReady === false) return null;
    if (!candidate || (candidate.type !== "agent" && candidate.type !== "squad") || typeof candidate.id !== "string" || !candidate.id.trim()) continue;
    const known = candidate.type === "agent" ? input.agentsKnown : input.agentsKnown && input.squadsKnown;
    if (!known) return null;
    const available = candidate.type === "agent" ? input.agents : input.squads;
    const explicit = index <= input.callers.length;
    if (available.some((actor) => actor.id === candidate.id) && (explicit || usable(candidate))) return candidate;
  }
  const first = input.agents.find((agent) => usable({ type: "agent", id: agent.id }));
  return input.agentsKnown && first ? { type: "agent", id: first.id } : null;
}
