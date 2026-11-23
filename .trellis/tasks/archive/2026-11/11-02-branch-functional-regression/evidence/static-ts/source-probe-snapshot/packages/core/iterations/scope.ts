import type { api } from "../api";
import type { Iteration } from "../api/iteration-schemas";
import { isIssueStatusCategory } from "../issue-statuses/queries";
import type { IssueStatusCategory } from "../types";
import { projectIterationEvent, type IterationActivityEntry, type IterationEvent } from "./activity";

type IterationDetail = Awaited<ReturnType<typeof api.getIteration>>;
type IterationStatistics = IterationDetail["statistics"];
type IterationSnapshot = NonNullable<IterationDetail["snapshot"]>;
type HistoricalIterationIssue = IterationSnapshot["scope"][number];

export type IterationScopePhase = "planned" | "active" | "completed"
  | "cancelledBeforeStart" | "cancelledAfterStart" | "unknown";
export type IterationScopeImpact =
  | { kind: "effective"; delta: -1 | 0 | 1 }
  | { kind: "planning" | "baseline" | "lifecycle" | "unknown" };
export type IterationScopeMetric = "original" | "current" | "initial_effective"
  | "effective" | "cancelled" | "added_unique" | "removed_events"
  | "reentry_events" | "cancel_events" | "reopen_events" | "net_effective_change";
export type IterationScopeTask = {
  issue_id: string;
  identifier?: string;
  title?: string;
  entry: IterationActivityEntry;
};
export type IterationScopeProjection = {
  phase: IterationScopePhase;
  startSequence: number | null;
  statistics: IterationStatistics;
  events: readonly {
    entry: IterationActivityEntry;
    impact: IterationScopeImpact;
    metrics: readonly IterationScopeMetric[];
  }[];
  // Completeness of this activity window, not a revision shared with live statistics.
  evidenceComplete: boolean;
};
export type IterationScopeMetricSelection =
  | { kind: "scope"; scope: "original" | "current"; filter: "all" | "effective" | "cancelled" }
  | { kind: "tasks"; issues: readonly IterationScopeTask[]; eventIds: readonly string[]; complete: boolean }
  | { kind: "events"; eventIds: readonly string[]; complete: boolean }
  | { kind: "unavailable" };

export function iterationScopePhase(
  iteration: Pick<Iteration, "status" | "started_at">,
  snapshot: IterationSnapshot | null,
): IterationScopePhase {
  if (snapshot) {
    if (iteration.started_at === null) return "unknown";
    switch (snapshot.end_type) {
      case "completed": return "completed";
      case "cancelled": return "cancelledAfterStart";
      default: return "unknown";
    }
  }
  switch (iteration.status) {
    case "planned": return iteration.started_at === null ? "planned" : "unknown";
    case "active": return iteration.started_at !== null ? "active" : "unknown";
    case "cancelled": return iteration.started_at === null ? "cancelledBeforeStart" : "unknown";
    default: return "unknown";
  }
}

const hasStarted = (phase: IterationScopePhase) =>
  phase === "active" || phase === "completed" || phase === "cancelledAfterStart";

type ScopeEntry = IterationScopeProjection["events"][number];
type ContextImpact = Exclude<IterationScopeImpact["kind"], "effective">;
const contextualEntry = (entry: IterationActivityEntry, kind: ContextImpact): ScopeEntry =>
  ({ entry, impact: { kind }, metrics: [] });

function storedCategory(entry: IterationActivityEntry, side: "before" | "after"): IssueStatusCategory | undefined {
  const raw = side === "before" ? entry.event.before_facts : entry.event.after_facts;
  const category = entry[side].status?.category;
  // The activity decoder owns every display field. Only attribution is checked
  // here because a readable title/category does not prove which issue it describes.
  return entry.issue && raw !== null && typeof raw === "object" && !Array.isArray(raw)
    && "issue_id" in raw && raw.issue_id === entry.issue.id
    && typeof category === "string" && isIssueStatusCategory(category) ? category : undefined;
}

function quantifiedEntry(
  entry: IterationActivityEntry,
  before: IssueStatusCategory | null,
  after: IssueStatusCategory | null,
  membershipMetric?: IterationScopeMetric,
): ScopeEntry {
  const wasEffective = before !== null && before !== "cancelled";
  const isEffective = after !== null && after !== "cancelled";
  const delta = wasEffective === isEffective ? 0 : isEffective ? 1 : -1;
  const metrics: IterationScopeMetric[] = membershipMetric ? [membershipMetric] : [];
  if (before !== null && after !== null) {
    if (before !== "cancelled" && after === "cancelled") metrics.push("cancel_events");
    if ((before === "done" || before === "cancelled") && after !== "done" && after !== "cancelled")
      metrics.push("reopen_events");
  }
  if (delta !== 0) metrics.push("net_effective_change");
  return { entry, impact: { kind: "effective", delta }, metrics };
}

function projectScopeEntry(
  entry: IterationActivityEntry,
  phase: IterationScopePhase,
  startSequence: number | null,
  endSequence: number | null,
): ScopeEntry {
  const { event } = entry;
  // These are the server history window's exclusions. In particular, a
  // baseline follows start in sequence but is not a post-start addition.
  switch (event.kind) {
    case "planned_activity": return contextualEntry(entry, "planning");
    case "start": case "end": case "completed": case "cancelled": case "cancel_planned":
    case "create": case "created": case "edit": case "edited": case "date_edit":
      return contextualEntry(entry, event.issue_id === null ? "lifecycle" : "unknown");
    case "baseline":
      return contextualEntry(entry, startSequence !== null && event.sequence > startSequence
        && event.before_facts === null && storedCategory(entry, "after") !== undefined ? "baseline" : "unknown");
    case "join": case "reenter": case "leave": case "delete":
    case "issue_changed": case "status": case "cancel": case "reopen": case "execution_started":
      break;
    default:
      // Readable legacy aliases and rollover records are not canonical counted
      // events. Their original records remain available through the activity entry.
      return contextualEntry(entry, "unknown");
  }
  if (phase === "planned" || phase === "cancelledBeforeStart") return contextualEntry(entry, "planning");
  if (startSequence === null) return contextualEntry(entry, "unknown");
  if (event.sequence <= startSequence) return contextualEntry(entry, "planning");
  if (endSequence !== null && event.sequence > endSequence) return contextualEntry(entry, "lifecycle");

  const before = storedCategory(entry, "before");
  const after = storedCategory(entry, "after");
  switch (event.kind) {
    case "join": case "reenter":
      if (event.before_facts === null && after !== undefined)
        return quantifiedEntry(entry, null, after, event.kind === "join" ? "added_unique" : "reentry_events");
      break;
    case "leave": case "delete":
      if (before !== undefined && event.after_facts === null)
        return quantifiedEntry(entry, before, null, "removed_events");
      break;
    default:
      if (before !== undefined && after !== undefined) return quantifiedEntry(entry, before, after);
  }
  return contextualEntry(entry, "unknown");
}

/** Explain individual stored transitions; never replay or replace service statistics. */
export function projectIterationScopeActivity(input: {
  iteration: Iteration;
  statistics: IterationStatistics;
  snapshot: IterationSnapshot | null;
  events: readonly IterationEvent[] | undefined;
}): IterationScopeProjection {
  const { iteration, snapshot } = input;
  const snapshotMatches = !snapshot || (snapshot.iteration_id === iteration.id && snapshot.workspace_id === iteration.workspace_id);
  const phase = snapshotMatches ? iterationScopePhase(iteration, snapshot) : "unknown";
  const source = snapshotMatches ? snapshot?.events ?? input.events : undefined;
  const events = [...(source ?? [])].sort((a, b) => b.sequence - a.sequence);
  const ids = new Set<string>();
  const sequences = new Set<number>();
  const collectionValid = events.every(event => {
    if (event.iteration_id !== iteration.id || ids.has(event.id) || sequences.has(event.sequence)) return false;
    ids.add(event.id);
    sequences.add(event.sequence);
    return true;
  });
  const starts = events.filter(event => event.kind === "start");
  const startSequence = collectionValid && hasStarted(phase) && starts.length === 1 && starts[0]!.issue_id === null
    ? starts[0]!.sequence : null;
  const endings = events.filter(event => startSequence !== null && event.sequence > startSequence
    && event.issue_id === null && ["end", "completed", "cancelled", "cancel_planned"].includes(event.kind));
  const endSequence = endings.at(-1)?.sequence ?? null;
  const projected = events.map(event => projectScopeEntry(projectIterationEvent(event), phase, startSequence, endSequence));
  const windowValid = hasStarted(phase)
    ? startSequence !== null && endings.length <= 1
      && !(phase === "active" && endSequence !== null)
      && !(snapshot && endSequence !== null && events.some(event => event.sequence > endSequence))
    : phase !== "unknown" && starts.length === 0;
  return {
    phase, startSequence,
    statistics: snapshotMatches && snapshot ? snapshot.statistics : input.statistics,
    events: projected,
    evidenceComplete: source !== undefined && collectionValid && windowValid
      && projected.every(item => item.impact.kind !== "unknown"
        || (startSequence !== null && item.entry.event.sequence < startSequence)),
  };
}

/** Activity counts are sanity checks, not proof of a shared live response revision. */
export function selectIterationScopeMetric(
  metric: IterationScopeMetric,
  projection: IterationScopeProjection,
): IterationScopeMetricSelection {
  if (projection.phase === "unknown") return { kind: "unavailable" };
  switch (metric) {
    case "current": return { kind: "scope", scope: "current", filter: "all" };
    case "effective": return { kind: "scope", scope: "current", filter: "effective" };
    case "cancelled": return { kind: "scope", scope: "current", filter: "cancelled" };
    case "original": case "initial_effective":
      return hasStarted(projection.phase)
        ? { kind: "scope", scope: "original", filter: metric === "original" ? "all" : "effective" }
        : { kind: "unavailable" };
    case "added_unique": case "removed_events": case "reentry_events":
    case "cancel_events": case "reopen_events": case "net_effective_change":
      break;
    default: return { kind: "unavailable" };
  }
  if (!hasStarted(projection.phase) || projection.startSequence === null) return { kind: "unavailable" };
  const selected = projection.events.filter(item => item.metrics.includes(metric));
  const entries = selected.map(item => item.entry);
  if (metric === "added_unique") {
    const additions = new Map<string, IterationScopeTask>();
    // Take the first canonical join's saved labels even if the issue was later
    // renamed, removed, or returned. Reentries never become new additions.
    for (const entry of [...entries].sort((a, b) => a.event.sequence - b.event.sequence)) {
      if (entry.issue && !additions.has(entry.issue.id)) additions.set(entry.issue.id, {
        issue_id: entry.issue.id,
        ...(entry.issue.title !== undefined ? { title: entry.issue.title } : {}),
        ...(entry.issue.identifier !== undefined ? { identifier: entry.issue.identifier } : {}),
        entry,
      });
    }
    const issues = [...additions.values()].reverse();
    return {
      kind: "tasks", issues, eventIds: issues.map(issue => issue.entry.event.id),
      complete: projection.evidenceComplete && issues.length === entries.length
        && issues.length === projection.statistics.added_unique,
    };
  }
  const eventIds = entries.map(entry => entry.event.id);
  // A signed difference cannot be checked against record count. Compare only
  // proven per-event deltas, retaining the authoritative statistic on mismatch.
  const evidenceValue = metric === "net_effective_change"
    ? selected.reduce((total, item) => total + (item.impact.kind === "effective" ? item.impact.delta : 0), 0)
    : eventIds.length;
  return {
    kind: "events", eventIds,
    complete: projection.evidenceComplete && evidenceValue === projection.statistics[metric],
  };
}

/** Call with one complete original/current collection; the caller checks its scope revision. */
export function selectIterationScopeIssues(
  items: readonly HistoricalIterationIssue[],
  filter: "all" | "effective" | "cancelled",
  expectedCount: number,
): { issues: readonly HistoricalIterationIssue[]; complete: boolean } {
  const issues: HistoricalIterationIssue[] = [];
  const ids = new Set<string>();
  let complete = true;
  for (const item of items) {
    if (ids.has(item.issue_id)) {
      complete = false;
      continue;
    }
    ids.add(item.issue_id);
    if (filter !== "all" && !isIssueStatusCategory(item.status_category)) {
      complete = false;
      continue;
    }
    if (filter === "all" || (filter === "cancelled") === (item.status_category === "cancelled")) issues.push(item);
  }
  return { issues, complete: complete && issues.length === expectedCount };
}
