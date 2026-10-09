// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { api } from "../api";
import type { Iteration } from "../api/iteration-schemas";
import type { IterationEvent } from "./activity";
import {
  iterationScopePhase,
  projectIterationScopeActivity,
  selectIterationScopeIssues,
  selectIterationScopeMetric,
  type IterationScopeMetric,
} from "./scope";

type Detail = Awaited<ReturnType<typeof api.getIteration>>;
type HistoricalIssue = NonNullable<Detail["snapshot"]>["scope"][number];
const workspaceId = "10000000-0000-4000-8000-000000000001";
const iterationId = "20000000-0000-4000-8000-000000000001";
const originalId = "30000000-0000-4000-8000-000000000001";
const addedId = "30000000-0000-4000-8000-000000000002";
const otherId = "30000000-0000-4000-8000-000000000003";
const time = "2026-10-09T08:00:00Z";

function iteration(fields: Partial<Iteration> = {}): Iteration {
  return {
    id: iterationId, workspace_id: workspaceId, name: "Scope example", description: null,
    coordinator_user_id: null, mode: "manual", status: "active", timezone: "Asia/Shanghai",
    start_date: "2026-10-09", end_date: "2026-10-15", revision: 1, scope_revision: 1,
    started_at: time, logical_ended_at: null, processed_at: null,
    ...fields,
  };
}

function statistics(fields: Partial<Detail["statistics"]> = {}): Detail["statistics"] {
  return {
    original: 0, current: 0, cancelled: 0, effective: 0, completed: 0,
    original_completed: 0, remaining: 0, added_unique: 0, removed_events: 0,
    reentry_events: 0, cancel_events: 0, reopen_events: 0, started: 0,
    initial_effective: 0, net_effective_change: 0, net_effective_change_ratio: null,
    effective_ratio: null, original_ratio: null, calculated_at: time, chart: [],
    ...fields,
  };
}

function facts(id: string, category: string, fields: Record<string, unknown> = {}) {
  return { issue_id: id, title: "Saved task", status_key: category, status_category: category, ...fields };
}

function event(sequence: number, fields: Partial<IterationEvent> = {}): IterationEvent {
  return {
    id: `50000000-0000-4000-8000-${String(sequence).padStart(12, "0")}`,
    iteration_id: iterationId, issue_id: originalId,
    operation_id: `60000000-0000-4000-8000-${String(sequence).padStart(12, "0")}`,
    sequence, kind: "issue_changed", actor: { type: "system" },
    before_facts: facts(originalId, "todo"), after_facts: facts(originalId, "todo"),
    occurred_at: time, sampled_at: time, reason: null,
    ...fields,
  };
}

function start(sequence = 1) {
  return event(sequence, { kind: "start", issue_id: null, before_facts: null, after_facts: { status: "active" } });
}

function baseline(sequence = 2, id = originalId, category = "todo") {
  return event(sequence, { kind: "baseline", issue_id: id, before_facts: null, after_facts: facts(id, category) });
}

function membership(sequence: number, kind: "join" | "leave" | "delete" | "reenter", id = addedId, category = "todo") {
  return event(sequence, {
    kind, issue_id: id,
    before_facts: kind === "join" || kind === "reenter" ? null : facts(id, category),
    after_facts: kind === "leave" || kind === "delete" ? null : facts(id, category),
  });
}

function historical(issue_id = originalId, status_category = "todo"): HistoricalIssue {
  return {
    issue_id, identifier: "OLD-1", title: "Historical title", status_key: status_category, status_category,
    project_id: null, project_name: null, assignee_type: null, assignee_id: null, assignee_name: null,
    was_completed_at_start: status_category === "done", rollover_count: 0,
  };
}

function snapshot(fields: Partial<NonNullable<Detail["snapshot"]>> = {}): NonNullable<Detail["snapshot"]> {
  return {
    schema_version: 1, workspace_id: workspaceId, iteration_id: iterationId,
    operation_id: "60000000-0000-4000-8000-000000000090", end_type: "completed",
    reason: "Delivery finished", logical_ended_at: time, processed_at: time,
    original: [], scope: [], events: [start()], statistics: statistics(), destinations: [],
    ...fields,
  };
}

function project(events: readonly IterationEvent[] | undefined, fields: Partial<Parameters<typeof projectIterationScopeActivity>[0]> = {}) {
  return projectIterationScopeActivity({ iteration: iteration(), statistics: statistics(), snapshot: null, events, ...fields });
}

const activityMetrics: IterationScopeMetric[] = [
  "added_unique", "removed_events", "reentry_events", "cancel_events", "reopen_events", "net_effective_change",
];

describe("iteration scope phase", () => {
  it.each([
    ["planned", null, "planned"],
    ["active", time, "active"],
    ["cancelled", null, "cancelledBeforeStart"],
    ["planned", time, "unknown"],
    ["active", null, "unknown"],
    ["completed", time, "unknown"],
    ["cancelled", time, "unknown"],
    ["future-status", time, "unknown"],
  ])("uses actual start and required closure evidence for %s / %s", (status, started_at, expected) => {
    expect(iterationScopePhase({ status, started_at }, null)).toBe(expected);
  });

  it.each([
    ["completed", "completed"],
    ["cancelled", "cancelledAfterStart"],
    ["future-ending", "unknown"],
  ])("prefers the saved %s ending to stale active metadata", (end_type, expected) => {
    expect(iterationScopePhase(iteration(), snapshot({ end_type }))).toBe(expected);
  });

  it("does not turn a snapshot without actual start evidence into an executed period", () => {
    expect(iterationScopePhase(iteration({ status: "cancelled", started_at: null }), snapshot())).toBe("unknown");
  });
});

describe("iteration scope statistical window", () => {
  it("keeps a two-task plan and pre-start deletion out of commitment metrics", () => {
    const counts = statistics({ current: 2, effective: 2, remaining: 2, net_effective_change: 2 });
    const plan = project([
      { ...membership(1, "join"), kind: "planned_activity" },
      membership(2, "delete"),
    ], { iteration: iteration({ status: "planned", started_at: null }), statistics: counts });
    expect(plan.phase).toBe("planned");
    expect(plan.statistics).toBe(counts);
    expect(plan.events.every(item => item.impact.kind === "planning")).toBe(true);
    expect(plan.startSequence).toBeNull();
    expect(selectIterationScopeMetric("current", plan)).toEqual({ kind: "scope", scope: "current", filter: "all" });
    for (const metric of ["initial_effective", ...activityMetrics] as const)
      expect(selectIterationScopeMetric(metric, plan)).toEqual({ kind: "unavailable" });
  });

  it("uses sequence despite identical clocks, and accepts baseline operations distinct from start", () => {
    const initial = baseline(4);
    const counts = statistics({ original: 1, initial_effective: 1, current: 2, effective: 2, added_unique: 1 });
    const entries = [membership(1, "delete"), start(3), initial, membership(5, "join")];
    const result = project(entries, { statistics: counts });
    expect(result.startSequence).toBe(3);
    const bySequence = new Map(result.events.map(item => [item.entry.event.sequence, item]));
    expect(bySequence.get(1)?.impact).toEqual({ kind: "planning" });
    expect(bySequence.get(3)?.impact).toEqual({ kind: "lifecycle" });
    expect(bySequence.get(4)?.impact).toEqual({ kind: "baseline" });
    expect(bySequence.get(5)?.impact).toEqual({ kind: "effective", delta: 1 });
    expect(result.evidenceComplete).toBe(true);
    expect(selectIterationScopeMetric("added_unique", result)).toMatchObject({ kind: "tasks", eventIds: [entries[3]!.id], complete: true });
    expect(entries.map(item => item.sequence)).toEqual([1, 3, 4, 5]);
  });

  it("allows a genuinely empty commitment and a first post-start join after planning participation", () => {
    const counts = statistics({ current: 1, effective: 1, added_unique: 1, net_effective_change: 1 });
    const entries = [
      { ...membership(1, "join"), kind: "planned_activity" },
      { ...membership(2, "leave"), kind: "planned_activity" },
      start(3), membership(4, "join"),
    ];
    const result = project(entries, { statistics: counts });
    expect(result.phase).toBe("active");
    expect(result.evidenceComplete).toBe(true);
    expect(result.statistics.original).toBe(0);
    expect(result.statistics.net_effective_change_ratio).toBeNull();
    expect(selectIterationScopeMetric("added_unique", result)).toMatchObject({ kind: "tasks", eventIds: [entries[3]!.id], complete: true });
    expect(selectIterationScopeMetric("reentry_events", result)).toEqual({ kind: "events", eventIds: [], complete: true });
  });

  it("does not count planned activity or metadata merely because its sequence follows start", () => {
    const result = project([
      start(), { ...membership(2, "join"), kind: "planned_activity" },
      event(3, { kind: "edit", issue_id: null, before_facts: { name: "Old" }, after_facts: { name: "New" } }),
    ]);
    expect(selectIterationScopeMetric("added_unique", result)).toEqual({ kind: "tasks", issues: [], eventIds: [], complete: true });
    expect(selectIterationScopeMetric("net_effective_change", result)).toEqual({ kind: "events", eventIds: [], complete: true });
    expect(result.events.find(item => item.entry.event.sequence === 2)?.impact).toEqual({ kind: "planning" });
  });

  it.each([
    undefined,
    [],
    [membership(2, "join")],
    [start(), start(2), membership(3, "join")],
    [start(1), event(2, { kind: "start" }), membership(3, "join")],
  ])("keeps missing or ambiguous start evidence unavailable: %j", events => {
    const result = project(events);
    expect(result.evidenceComplete).toBe(false);
    expect(result.startSequence).toBeNull();
    for (const metric of activityMetrics) expect(selectIterationScopeMetric(metric, result)).toEqual({ kind: "unavailable" });
    expect(result.events.every(item => item.impact.kind !== "effective")).toBe(true);
  });

  it("uses only frozen events/statistics before closeout membership releases", () => {
    const end = event(4, { kind: "end", issue_id: null, before_facts: null, after_facts: { status: "completed" } });
    const frozenCounts = statistics({ original: 1, current: 1, initial_effective: 1, effective: 1 });
    const frozen = snapshot({
      events: [start(), baseline(), end], statistics: frozenCounts,
      original: [historical()], scope: [historical()],
      destinations: [{ issue_id: originalId, target_iteration_id: null, rollover_count_before: 0, rollover_count_after: 0 }],
    });
    const live = [start(), baseline(), end, membership(5, "leave", originalId)];
    const result = project(live, { snapshot: frozen, statistics: statistics({ removed_events: 99 }) });
    expect(result.phase).toBe("completed");
    expect(result.statistics).toBe(frozenCounts);
    expect(result.events.map(item => item.entry.event.id)).not.toContain(live[3]!.id);
    expect(selectIterationScopeMetric("removed_events", result)).toEqual({ kind: "events", eventIds: [], complete: true });
    expect(result.events.find(item => item.entry.event.sequence === 2)?.entry.issue?.title).toBe("Saved task");
  });

  it("does not count closure releases when activity has advanced past active detail metadata", () => {
    const end = event(4, { kind: "end", issue_id: null });
    const release = membership(5, "leave", originalId);
    const result = project([start(), baseline(), end, release], {
      statistics: statistics({ original: 1, initial_effective: 1, current: 1, effective: 1 }),
    });
    expect(result.events.find(item => item.entry.event.id === release.id)?.impact).toEqual({ kind: "lifecycle" });
    expect(selectIterationScopeMetric("removed_events", result)).toEqual({ kind: "events", eventIds: [], complete: false });
  });

  it("does not display a snapshot belonging to another iteration", () => {
    const counts = statistics();
    const result = project([start()], { statistics: counts, snapshot: snapshot({ iteration_id: otherId }) });
    expect(result.phase).toBe("unknown");
    expect(result.statistics).toBe(counts);
    expect(result.events).toEqual([]);
    expect(result.evidenceComplete).toBe(false);
  });
});

describe("effective impact from stored facts", () => {
  it.each([
    ["cancel", "todo", "cancelled", -1, ["cancel_events", "net_effective_change"]],
    ["cancel", "done", "cancelled", -1, ["cancel_events", "net_effective_change"]],
    ["reopen", "cancelled", "todo", 1, ["reopen_events", "net_effective_change"]],
    ["reopen", "cancelled", "done", 1, ["net_effective_change"]],
    ["reopen", "done", "todo", 0, ["reopen_events"]],
    ["status", "todo", "done", 0, []],
    ["issue_changed", "todo", "cancelled", -1, ["cancel_events", "net_effective_change"]],
    ["status", "done", "blocked", 0, ["reopen_events"]],
    ["execution_started", "todo", "in_progress", 0, []],
    ["issue_changed", "todo", "todo", 0, []],
  ])("explains %s / %s → %s without relying on the raw action name", (kind, before, after, delta, metrics) => {
    const change = event(3, { kind, before_facts: facts(originalId, before), after_facts: facts(originalId, after) });
    const result = project([start(), baseline(2, originalId, before), change]);
    const projected = result.events.find(item => item.entry.event.id === change.id)!;
    expect(projected.impact).toEqual({ kind: "effective", delta });
    expect([...projected.metrics].sort()).toEqual([...metrics].sort());
    expect(projected.entry.event).toBe(change);
    expect(result.evidenceComplete).toBe(true);
  });

  it.each([
    ["join", "todo", 1, "added_unique"],
    ["reenter", "in_review", 1, "reentry_events"],
    ["join", "cancelled", 0, "added_unique"],
    ["reenter", "cancelled", 0, "reentry_events"],
    ["leave", "todo", -1, "removed_events"],
    ["delete", "done", -1, "removed_events"],
    ["leave", "cancelled", 0, "removed_events"],
    ["delete", "cancelled", 0, "removed_events"],
  ] as const)("separates %s membership in %s from effective scope", (kind, category, delta, metric) => {
    const change = membership(3, kind, addedId, category);
    const result = project([start(), change], { statistics: statistics({ [metric]: 1 }) });
    const projected = result.events.find(item => item.entry.event.id === change.id)!;
    expect(projected.impact).toEqual({ kind: "effective", delta });
    expect(projected.metrics).toContain(metric);
    expect(projected.metrics.includes("net_effective_change")).toBe(delta !== 0);
    expect(selectIterationScopeMetric(metric, result)).toMatchObject({ eventIds: [change.id], complete: true });
  });

  it.each([
    { kind: "rollover" },
    { kind: "future-membership" },
    { kind: "deleted" },
    { before_facts: undefined },
    { after_facts: undefined },
    { before_facts: null },
    { after_facts: [] },
    { after_facts: "malformed" },
    { after_facts: facts(originalId, "future-category") },
    { after_facts: { issue_id: originalId, status_key: "todo" } },
    { after_facts: { status_category: "todo" } },
    { after_facts: facts(otherId, "todo") },
    { before_facts: facts(otherId, "todo") },
    { issue_id: null },
    { kind: "join", before_facts: undefined },
    { kind: "leave", after_facts: undefined },
  ])("preserves unknown/inconsistent evidence instead of manufacturing a delta: %j", fields => {
    const change = event(3, fields);
    const result = project([start(), baseline(), change]);
    const projected = result.events.find(item => item.entry.event.id === change.id)!;
    expect(projected.impact).toEqual({ kind: "unknown" });
    expect(projected.metrics).toEqual([]);
    expect(projected.entry.event).toBe(change);
    expect(result.evidenceComplete).toBe(false);
    expect(selectIterationScopeMetric("net_effective_change", result)).toEqual({ kind: "events", eventIds: [], complete: false });
  });
});

describe("truthful metric evidence", () => {
  it("selects cancellation and reopening occurrences from transitions, including ordinary status records", () => {
    const changes = [
      event(3, { kind: "cancel", before_facts: facts(originalId, "todo"), after_facts: facts(originalId, "cancelled") }),
      event(4, { kind: "reopen", before_facts: facts(originalId, "cancelled"), after_facts: facts(originalId, "done") }),
      event(5, { kind: "reopen", before_facts: facts(originalId, "done"), after_facts: facts(originalId, "todo") }),
      event(6, { kind: "issue_changed", before_facts: facts(originalId, "todo"), after_facts: facts(originalId, "cancelled") }),
      event(7, { kind: "status", before_facts: facts(originalId, "cancelled"), after_facts: facts(originalId, "in_review") }),
    ];
    const result = project([start(), baseline(), ...changes], {
      statistics: statistics({ original: 1, initial_effective: 1, current: 1, effective: 1, cancel_events: 2, reopen_events: 2 }),
    });
    expect(selectIterationScopeMetric("cancel_events", result)).toEqual({ kind: "events", eventIds: [changes[3]!.id, changes[0]!.id], complete: true });
    expect(selectIterationScopeMetric("reopen_events", result)).toEqual({ kind: "events", eventIds: [changes[4]!.id, changes[2]!.id], complete: true });
  });

  it("keeps additions after removal and counts every reentry separately", () => {
    const entries = [start(), baseline(), membership(3, "leave", originalId), membership(4, "reenter", originalId),
      membership(5, "join"), membership(6, "leave"), membership(7, "reenter"),
      membership(8, "leave"), membership(9, "reenter"), membership(10, "delete")];
    const counts = statistics({ original: 1, current: 1, initial_effective: 1, effective: 1, added_unique: 1, removed_events: 4, reentry_events: 3 });
    const result = project(entries, { statistics: counts });
    const added = selectIterationScopeMetric("added_unique", result);
    expect(added).toMatchObject({ kind: "tasks", issues: [{ issue_id: addedId, title: "Saved task" }], eventIds: [entries[4]!.id], complete: true });
    if (added.kind === "tasks") expect(added.issues[0]?.entry.event).toBe(entries[4]);
    const removed = selectIterationScopeMetric("removed_events", result);
    expect(removed).toMatchObject({ kind: "events", complete: true });
    if (removed.kind === "events") expect([...removed.eventIds].sort()).toEqual([3, 6, 8, 10].map(n => event(n).id).sort());
    const returned = selectIterationScopeMetric("reentry_events", result);
    expect(returned).toMatchObject({ kind: "events", complete: true });
    if (returned.kind === "events") expect([...returned.eventIds].sort()).toEqual([4, 7, 9].map(n => event(n).id).sort());
  });

  it("retains first-addition labels instead of substituting later task facts", () => {
    const first = membership(3, "join");
    first.after_facts = facts(addedId, "todo", { title: "First saved title", identifier: "OLD-42" });
    const result = project([start(), first,
      event(4, { issue_id: addedId, before_facts: first.after_facts, after_facts: facts(addedId, "todo", { title: "Later title" }) }),
      membership(5, "leave"), membership(6, "reenter")],
    { statistics: statistics({ added_unique: 1, removed_events: 1, reentry_events: 1 }) });
    expect(selectIterationScopeMetric("added_unique", result)).toMatchObject({
      kind: "tasks", issues: [{ issue_id: addedId, title: "First saved title", identifier: "OLD-42" }], eventIds: [first.id], complete: true,
    });
    const withoutTitle = membership(7, "join", otherId);
    withoutTitle.after_facts = { issue_id: otherId, status_category: "todo" };
    const unnamed = selectIterationScopeMetric("added_unique", project([start(), withoutTitle], { statistics: statistics({ added_unique: 1 }) }));
    expect(unnamed).toMatchObject({ kind: "tasks", issues: [{ issue_id: otherId }], complete: true });
    if (unnamed.kind === "tasks") {
      expect(unnamed.issues[0]?.title).toBeUndefined();
      expect(unnamed.issues[0]?.identifier).toBeUndefined();
    }
  });

  it("keeps the first join once but does not call repeated canonical joins complete evidence", () => {
    const first = membership(3, "join");
    const duplicate = membership(7, "join");
    duplicate.after_facts = facts(addedId, "todo", { title: "Later duplicate" });
    const result = project([start(), duplicate, first], { statistics: statistics({ added_unique: 1 }) });
    expect(selectIterationScopeMetric("added_unique", result)).toMatchObject({
      kind: "tasks", issues: [{ issue_id: addedId, title: "Saved task" }], eventIds: [first.id], complete: false,
    });
  });

  it("counts records in a bulk operation and shows both contributors to net zero", () => {
    const joined = membership(3, "join");
    const left = { ...membership(4, "leave", originalId), operation_id: joined.operation_id };
    const result = project([start(), baseline(), joined, left], {
      statistics: statistics({ original: 1, initial_effective: 1, effective: 1, current: 1, added_unique: 1, removed_events: 1, net_effective_change: 0 }),
    });
    const net = selectIterationScopeMetric("net_effective_change", result);
    expect(net).toMatchObject({ kind: "events", complete: true });
    if (net.kind === "events") expect([...net.eventIds].sort()).toEqual([joined.id, left.id].sort());
    expect(result.statistics.net_effective_change).toBe(0);
    expect(result.events.map(item => item.entry.event)).toEqual(expect.arrayContaining([joined, left]));
  });

  it.each([0, 3])("marks signed net evidence incomplete when its proven delta disagrees with server net %s", serverNet => {
    const joined = membership(3, "join");
    const counts = statistics({ added_unique: 1, net_effective_change: serverNet });
    const result = project([start(), joined], { statistics: counts });
    expect(result.evidenceComplete).toBe(true);
    expect(selectIterationScopeMetric("net_effective_change", result)).toEqual({ kind: "events", eventIds: [joined.id], complete: false });
    expect(result.statistics).toBe(counts);
  });

  it("checks signed net evidence against its negative delta rather than its record count", () => {
    const left = membership(3, "leave", originalId);
    const result = project([start(), baseline(), left], {
      statistics: statistics({ original: 1, initial_effective: 1, removed_events: 1, net_effective_change: -1 }),
    });
    expect(selectIterationScopeMetric("net_effective_change", result)).toEqual({ kind: "events", eventIds: [left.id], complete: true });
  });

  it.each(["added_unique", "removed_events", "reentry_events", "cancel_events", "reopen_events"] as const)("does not claim exact %s details when the count disagrees", metric => {
    const counts = statistics({ [metric]: 9 });
    const result = project([start(), membership(3, "join")], { statistics: counts });
    expect(selectIterationScopeMetric(metric, result)).toMatchObject({ complete: false });
    expect(result.statistics).toBe(counts);
  });

  it("returns known supporting records with an incomplete marker when another change is unknown", () => {
    const joined = membership(3, "join");
    const result = project([start(), joined, event(4, { kind: "future-event" })], { statistics: statistics({ added_unique: 1 }) });
    expect(selectIterationScopeMetric("added_unique", result)).toMatchObject({ kind: "tasks", eventIds: [joined.id], complete: false });
  });

  it.each([
    (joined: IterationEvent) => ({ ...joined, sequence: 4 }),
    (joined: IterationEvent) => ({ ...joined, id: event(4).id }),
    (joined: IterationEvent) => ({ ...joined, id: event(4).id, sequence: 4, iteration_id: otherId }),
  ])("rejects ambiguous collection identity/sequence before asserting a complete window", invalid => {
    const joined = membership(3, "join");
    const result = project([start(), joined, invalid(joined)], { statistics: statistics({ added_unique: 1 }) });
    expect(result.evidenceComplete).toBe(false);
    expect(selectIterationScopeMetric("added_unique", result)).toMatchObject({ kind: "unavailable" });
  });

  it("offers independent authoritative task sources while activity is still loading", () => {
    const result = project(undefined);
    for (const [metric, scope, filter] of [
      ["original", "original", "all"], ["current", "current", "all"],
      ["initial_effective", "original", "effective"], ["effective", "current", "effective"],
      ["cancelled", "current", "cancelled"],
    ] as const) {
      expect(selectIterationScopeMetric(metric, result)).toEqual({ kind: "scope", scope, filter });
    }
    expect(selectIterationScopeMetric("added_unique", result)).toEqual({ kind: "unavailable" });
  });
});

describe("authoritative original/current issue selections", () => {
  it("includes done in effective scope and keeps original frozen categories", () => {
    const original = [historical(originalId, "todo"), historical(addedId, "done"), historical(otherId, "cancelled")];
    expect(selectIterationScopeIssues(original, "all", 3)).toEqual({ issues: original, complete: true });
    expect(selectIterationScopeIssues(original, "effective", 2)).toEqual({ issues: original.slice(0, 2), complete: true });
    expect(selectIterationScopeIssues(original, "cancelled", 1)).toEqual({ issues: [original[2]], complete: true });
    expect(selectIterationScopeIssues([], "effective", 0)).toEqual({ issues: [], complete: true });
    expect(original[0]?.status_category).toBe("todo");
  });

  it("requires known categories for category subsets without guessing from status keys", () => {
    const known = historical();
    const unknown = { ...historical(addedId), status_category: "future-category", status_key: "todo" };
    expect(selectIterationScopeIssues([known, unknown], "all", 2)).toEqual({ issues: [known, unknown], complete: true });
    expect(selectIterationScopeIssues([known, unknown], "effective", 1)).toEqual({ issues: [known], complete: false });
    expect(selectIterationScopeIssues([known, unknown], "cancelled", 0)).toEqual({ issues: [], complete: false });
  });

  it("deduplicates issue identities but reports duplicates and mismatched cardinality", () => {
    const item = historical();
    expect(selectIterationScopeIssues([item, { ...item, title: "Conflicting duplicate" }], "all", 1)).toEqual({ issues: [item], complete: false });
    expect(selectIterationScopeIssues([item], "effective", 2)).toEqual({ issues: [item], complete: false });
    expect(selectIterationScopeIssues([], "all", 1)).toEqual({ issues: [], complete: false });
  });
});
