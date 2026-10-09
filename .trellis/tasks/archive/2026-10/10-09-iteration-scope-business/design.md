# Iteration scope business presentation design

## Objective and change boundary

Correct the screenshot's planned-period `0 → 2 / +2 / added 0` interpretation and connect authoritative scope counters to understandable, actionable evidence. The current gap lives in the shared iteration page/panel and pure activity presentation, not the Go statistics reducer.

Implement in `packages/core/iterations` (pure projections/selectors) and `packages/views/iterations` (phase-aware presentation, inline details and navigation), with shared English/Chinese strings and focused E2E coverage. No new route, wire schema, dependency, database or backend behavior. Preserve the incumbent restrained app styling and semantic tokens.

## Architecture and data flow

1. Existing protected iteration detail query supplies iteration metadata, authoritative statistics and optional frozen snapshot.
2. Pass iteration metadata into the scope panel. A core phase helper uses actual `started_at`, known lifecycle status and a valid snapshot, never counts or dates.
3. Existing `iterationActivityOptions` supplies the complete authorized live event array. Frozen panels use only `snapshot.events` and `snapshot.statistics`.
4. A new pure `scope.ts` reuses `projectIterationEvent` to add period-window/impact information and select supporting records. It must not reproduce `ScopeState` or compute replacement statistics.
5. Task-set metric clicks lazily reuse `iterationGroupedIssuesOptions` for the requested original/current collection. Validate its `scope_revision` against the current detail and its selected cardinality against the metric. Frozen task sets use `snapshot.original`/`snapshot.scope` directly.
6. A small inline scope-detail component renders selected tasks or records. Query owns server data; selected metric, category, search and optional task filters remain panel-local view state. Main Tasks tab state is untouched.

## Shared core contract

Use these named exports from `packages/core/iterations/index.ts` (types can use local aliases inferred from existing schemas):

```ts
type IterationScopePhase = "planned" | "active" | "completed"
  | "cancelledBeforeStart" | "cancelledAfterStart" | "unknown";
type IterationScopeImpact =
  | { kind: "effective"; delta: -1 | 0 | 1 }
  | { kind: "planning" | "baseline" | "lifecycle" | "unknown" };
type IterationScopeMetric = "original" | "current" | "initial_effective"
  | "effective" | "cancelled" | "added_unique" | "removed_events"
  | "reentry_events" | "cancel_events" | "reopen_events" | "net_effective_change";

iterationScopePhase(iteration, snapshot): IterationScopePhase;
projectIterationScopeActivity({ iteration, statistics, snapshot, events }): {
  phase: IterationScopePhase;
  startSequence: number | null;
  statistics: IterationStatistics;
  events: readonly { entry: IterationActivityEntry;
    impact: IterationScopeImpact; metrics: readonly IterationScopeMetric[] }[];
  evidenceComplete: boolean;
};
selectIterationScopeMetric(metric, projection):
  | { kind: "scope"; scope: "original" | "current";
      filter: "all" | "effective" | "cancelled" }
  | { kind: "tasks"; issues: readonly IterationScopeTask[];
      eventIds: readonly string[]; complete: boolean }
  | { kind: "events"; eventIds: readonly string[]; complete: boolean }
  | { kind: "unavailable" };
```

`IterationScopeTask` carries a real issue ID plus optional frozen identifier/title and the supporting decoded entry, so a missing title is never fabricated. The implementer documents its exact shape before the UI consumer imports it. A small exported pure scope-array selector may share known-category/cardinality checks; no view should decode raw event JSON or reconstruct the statistics reducer. Scope selections can be available before activity loads because their authoritative query source is independent.

## Phase and metric semantics

| Phase | Summary and activity wording |
| --- | --- |
| Planned, no actual start | Planned tasks (`current`), no commitment yet, Planning adjustments; hide initial/net/after-start counters. |
| Cancelled before start, no snapshot | Plan cancelled before starting; explain that no commitment was formed. Retain the planning audit. |
| Active with real start | Scope at start / Current effective scope / Net effective change. A real empty baseline remains 0. |
| Completed or cancelled after start, valid snapshot | Scope at start / Effective scope at closure / Net change, using frozen values; distinguish cancelled. |
| Unknown or inconsistent facts | Neutral unavailable explanation and inspectable records, without claiming a planned/active/frozen phase. |

A valid returned snapshot takes precedence over stale live catalogue metadata. Unknown/malformed end types or impossible start/snapshot combinations must not invent a phase. Cancellation-before-start has no snapshot in the current contract.

Metrics retain exact units. `added_unique` selects first canonical post-start joins, deduplicated by issue; reentry never adds a new distinct task. `removed_events` selects post-start leave/delete occurrences. Cancellation/reopen selection follows known status transitions, not raw event names alone. Net change opens contributing scope activity with explanatory copy, never an invented signed task set. `started` remains a noninteractive detailed count because the historical DTO lacks sticky participation execution evidence.

## Per-event impact and evidence

- Partition by the single actual start marker's sequence, not timestamps. Baseline formation, planned activity, lifecycle anchors and pre-start deletion do not claim post-start effective deltas.
- For a recognized membership/status change, known cancelled membership contributes 0; known noncancelled membership contributes 1; explicit absence contributes 0. Missing/malformed status facts yield unknown, not zero.
- `done → todo` has delta 0 and counts as reopening. `cancelled → done` has delta +1 and does not count as reopening. Removing a cancelled task has delta 0 but is a removal occurrence.
- Retain operation grouping and saved-timezone/sequence ordering. Filtering children retains their operation context without counting nonmatching anchor records as matches.
- Compare metric selection cardinalities with the authoritative metric. Explain partial/unsupported evidence and preserve the number; do not repair history or invent missing entries.
- Live activities and detail statistics do not carry one common collection revision. Present activity selections as supporting records, not a promise of a same-version current task set. Current/original complete task lists have a revision and must match it before claiming an exact list.

## UI composition and interaction

- Preserve existing header, tabs, typography, whitespace and native disclosures. Keep Planning adjustments reachable when a plan becomes empty; the Tasks tab remains its default entry and keeps the existing empty-state actions.
- Primary counters that have trustworthy details are semantic buttons with visible focus, localized accessible names and a selected treatment that remains visible on hover. No equal-sized dashboard cards or new modal.
- Clicking a metric clears incompatible local refinements and shows an inline titled detail section. Provide an explicit clear/back-to-activity action. Later search/category refinements show a matching count, never alter the summary above.
- Activity retains Scope/All selection and adds category plus task title/identifier search. Count matching records separately from displayed operation groups. Task details may use known project/assignee facts for narrowing; unknown is distinct from unassigned.
- Keep raw event identity, operation ID, sequence, kind, sampled time and original facts under a renamed Technical audit disclosure, default closed. Business rows foreground action, frozen task identity, reason/movement and known impact.
- For minimal event task identities, provide `AppLink` to the real issue ID with copy explaining that it opens the current task. For full historical task rows, reuse `IterationCurrentComparison`. Never fetch every live task to decorate historical rows.
- English and Chinese share keys. Long titles/reasons wrap; controls retain 44px coarse-pointer sizing. Preserve filtered empty/error/retry states and mounted tab filters.

## Read consistency and access recovery

Use existing protected query helpers and Query invalidation. A failed first read is not an empty result. A temporary background error can retain the last authorized complete result, with retry. A stale task-list revision requests a fresh detail/list and exposes the mismatch rather than an exact-looking stale list. Cancellation after workspace revocation must remove counters, records and any opened detail, even if the final error is `CancelledError`. Iteration 404 does not revoke sibling iterations. Frozen arrays never backfill from live joins.

## Ownership

- Core implementer: `packages/core/iterations/scope.ts`, `scope.test.ts`, and the export block in `index.ts` only.
- UI implementer: iteration page/panel/event rendering, an inline scope-detail component if needed, relevant shared view tests, and `locales/en/projects.json` / `locales/zh-Hans/projects.json`. No API or core changes.
- Browser-test implementer: one focused E2E spec, or a shared focused scenario and thin Web/native wrappers if that provides real parity; no product files or shared fixture rewrites without agreement.
- Main: task artifacts, integration, specifications, provenance, visual inspection, final verification and local commit. Preserve pre-existing `apps/web/next-env.d.ts` contents.

## Alternatives and rollback

Rejected a backend statistics change: the existing counts deliberately distinguish pre-start planning and post-start changes. Rejected a second client statistics reducer: it duplicates the canonical rules and can drift. Rejected automatic main-task-tab filter rewrites: they discard the user's browsing context. Rejected eager per-task reads and new metric API parameters: existing complete queries already cover supported task sets.

Rollback is limited to shared presentation/selectors, tests and locale additions; it needs no data migration and does not alter stored facts or permissions. The feature adds no production writes.
