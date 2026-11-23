import { KNOWLEDGE_MAX_VISIBLE_MS, type CollaborationKnowledgeProjection } from "./knowledge-schema";

export interface KnowledgeProjectionBoundary {
  readonly generation: number;
  readonly groupGeneration: number;
  readonly group: string;
  readonly deadline: number;
  readonly projection_checked_at: string;
  readonly valid_until: string;
  readonly next_expiry_at: string | null;
}
interface KnowledgeReadToken {
  generation: number; groupGeneration: number; group: string; startedAt: number;
  parent?: KnowledgeProjectionBoundary;
}
export const knowledgeProjectionExpired = () => new DOMException("Knowledge authorization projection expired", "AbortError");

/** Metadata only. Monotonic request time consumes latency; wall-clock skew cannot extend visibility. */
export class KnowledgeProjectionPolicy {
  private generation = 0;
  private groups = new Map<string, { generation: number; boundary?: KnowledgeProjectionBoundary }>();
  private lastChecks = new Map<string, { checkedAt: number; deadline: number }>();
  constructor(private readonly now: () => number = () => performance.now()) {}

  invalidate(group?: string): void {
    if (group === undefined) { this.generation++; this.groups.clear(); }
    else { const entry = this.groups.get(group); this.groups.set(group, { generation: (entry?.generation ?? 0) + 1 }); }
  }

  begin(group: string, parent?: KnowledgeProjectionBoundary): KnowledgeReadToken {
    if (parent && !this.isValid(parent, group)) throw knowledgeProjectionExpired();
    if (!parent) this.invalidate(group);
    return { generation: this.generation, groupGeneration: this.groups.get(group)!.generation, group, startedAt: this.now(), parent };
  }

  isCurrent(token: Pick<KnowledgeReadToken, "generation" | "groupGeneration" | "group">): boolean {
    return token.generation === this.generation && token.groupGeneration === this.groups.get(token.group)?.generation;
  }

  isValid(boundary: KnowledgeProjectionBoundary, group = boundary.group): boolean {
    return boundary.group === group && this.groups.get(group)?.boundary === boundary &&
      this.isCurrent(boundary) && this.now() < boundary.deadline;
  }

  accept(token: KnowledgeReadToken, value: CollaborationKnowledgeProjection): KnowledgeProjectionBoundary {
    const now = this.now();
    if (!this.isCurrent(token) || !Number.isFinite(now) || now < token.startedAt) throw knowledgeProjectionExpired();
    const checkedAt = Date.parse(value.projection_checked_at);
    const previous = this.lastChecks.get(token.group);
    if (previous && checkedAt < previous.checkedAt) throw knowledgeProjectionExpired();
    const duration = Math.min(KNOWLEDGE_MAX_VISIBLE_MS, Date.parse(value.valid_until) - checkedAt,
      value.next_expiry_at === null ? Infinity : Date.parse(value.next_expiry_at) - checkedAt);
    let deadline = token.startedAt + duration;
    // A replay of the same server check is not a new authorization, even on the first page.
    if (previous?.checkedAt === checkedAt) deadline = Math.min(deadline, previous.deadline);
    if (token.parent) {
      if (!this.isValid(token.parent) || value.projection_checked_at !== token.parent.projection_checked_at ||
        value.valid_until !== token.parent.valid_until || value.next_expiry_at !== token.parent.next_expiry_at) throw knowledgeProjectionExpired();
      deadline = Math.min(deadline, token.parent.deadline);
    }
    if (!Number.isFinite(deadline) || deadline <= now) throw knowledgeProjectionExpired();
    // Every page shares the exact first-page boundary, rather than renewing it.
    const boundary = token.parent ?? Object.freeze({ generation: token.generation, groupGeneration: token.groupGeneration,
      group: token.group, deadline, projection_checked_at: value.projection_checked_at, valid_until: value.valid_until, next_expiry_at: value.next_expiry_at });
    this.groups.get(token.group)!.boundary = boundary;
    this.lastChecks.set(token.group, { checkedAt, deadline });
    return boundary;
  }
}
