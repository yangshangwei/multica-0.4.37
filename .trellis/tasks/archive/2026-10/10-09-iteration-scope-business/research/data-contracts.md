# Research: Iteration scope data contracts and truthful drilldowns

- Query: Map lifecycle, statistics, complete activity, frozen/current task sources and the smallest pure-core projection that explains scope changes without changing backend counts.
- Scope: internal; `packages/core/iterations`, iteration API schemas, server history/statistics/lifecycle writers and their canonical tests.
- Date: 2026-10-10 (Asia/Shanghai; task created 2026-10-09).
- Method: Read-only source/spec/test inspection. No application code, specs or task metadata changed; no tests executed by this research agent.

## Findings

### Decision summary

Keep the server's `Statistics` object authoritative. Add a small pure presentation/selection helper beside `activity.ts`, reusing `projectIterationEvent`; do not port `stats.go` or replay a second complete statistics model in TypeScript.

1. Classify the period from actual start evidence and snapshot presence, never from `initial_effective > 0`, planned dates or `status !== planned` alone.
2. Explain each recognized event from its own stored before/after membership and status facts, in the sequence-defined period window. Planning, baseline formation, closure and unknown facts have explicit nonnumeric states.
3. Select distinct post-start additions for `added_unique`; select occurrences for removal/reentry/cancel/reopen metrics. Select contributing activity for net change, never an invented “+N tasks” set.
4. Use existing original/current issue sources for task-set metrics. Fetch complete live membership only when a requested drilldown needs it; use snapshot arrays directly when frozen.
5. Missing facts, incompatible kinds, partial history or mismatched cardinalities must not become an exact empty/matching list. Preserve visible server counters and explain unavailable or partial supporting evidence.

### Files found

| File | Purpose |
| --- | --- |
| `packages/core/api/iteration-schemas.ts` | Validated iteration/status/start, statistics, historical issue, event and snapshot API contracts. |
| `packages/core/iterations/activity.ts` | Complete activity query; stored-fact decoding; semantic kinds; operation/day grouping. |
| `packages/core/iterations/index.ts` | Detail, ordinary issue-page, complete grouped-issue and old single-page event query options. |
| `packages/core/iterations/access.ts` | Session/authorization epochs and protected query eviction. |
| `server/internal/iteration/history.go` | Live/frozen source choice, baseline/original facts and sequence cutoff for statistical changes. |
| `server/internal/iteration/stats.go` | Authoritative counts, denominators and scope semantics. |
| `server/internal/iteration/record.go` | Stored issue facts and ordinary status/cancel/reopen event naming. |
| `server/internal/iteration/record_delete.go` | Deletion preserves historical facts and releases membership. |
| `server/internal/iteration/membership.go` | Canonical join/reenter/leave classification; source/target facts. |
| `server/internal/iteration/membership_batch.go` | Bulk source-leaves-before-target-joins with one operation identity. |
| `server/internal/service/iteration_lifecycle_apply.go` | Start marker precedes baseline, planned cancellation, closure order. |
| `server/internal/service/iteration_closure.go` | Snapshot before releases, complete closure destinations and original capture. |
| `server/internal/iteration/history_snapshot.go` | Immutable snapshot ownership, identity/completeness/statistics validation. |
| `server/internal/handler/iteration_history.go` | Authorized reads, current/original filters, revision-bound paging. |
| `server/pkg/db/queries/iteration_history.sql` | Complete ascending event stream and tenant-scoped display references. |
| `server/internal/iteration/stats_test.go` | Canonical counts, no-ops, reentry, terminal transitions, deletion and empty denominators. |
| `server/internal/handler/iteration_history_test.go` | Persisted history, same-clock sequencing, frozen reads, paging and unstarted cancellation. |
| `packages/core/iterations/activity.test.ts` | Complete traversal, stale restart, authorization, historical unknown/null and reopen boundaries. |

### Lifecycle and authoritative source

`IterationSchema` requires nullable `started_at`, `logical_ended_at` and `processed_at`; status/mode are open strings, preserving future values (`packages/core/api/iteration-schemas.ts:34`). `IterationDetailSchema` returns iteration + statistics + nullable snapshot, with workspace/iteration identity checks (`:143`).

| Evidence | Business phase / presentation | Statistics and issue source |
| --- | --- | --- |
| `planned`, `started_at === null`, no snapshot | Plan, no initial commitment yet | Current live plan; show `current` as planned task count. No post-start/net wording. |
| `active`, nonnull start, no snapshot | Running | Detail statistics, live current/original issues and complete activity. |
| Valid snapshot, normally `completed` | Completed/frozen | `snapshot.statistics`, `snapshot.scope`, `snapshot.original`, `snapshot.events`. |
| Valid cancelled snapshot / `cancelled` with start | Cancelled after starting | Same frozen facts, cancelled label. |
| `cancelled`, null start, no snapshot | Plan cancelled before starting | No commitment and no end snapshot. Do not present an “ended effective scope” as if execution occurred. |
| Unknown/inconsistent metadata, or closed-started resource missing its required snapshot | Unknown/unavailable historical state | Read-only; no fabricated running/planned/frozen interpretation. |

- Backend `LoadHistory` uses a stored snapshot for `completed` and for `cancelled && StartedAt.Valid`, and checks header/time consistency (`server/internal/iteration/history.go:71`). Active start evidence must be internally consistent; unstarted originals are rejected (`:99`, `:223`).
- A returned snapshot takes display precedence over stale catalogue metadata; this is an existing core spec rule (`.trellis/spec/core/frontend/iteration-operations.md:158`). Prefer the snapshot's known `end_type` for completed versus cancelled presentation.
- An empty baseline is a valid started period. Original and initial effective count 0 do not imply a plan. Canonical tests: `server/internal/service/iteration_lifecycle_test.go:304`; `server/internal/iteration/stats_test.go:62`.
- Unstarted cancellation has no snapshot and releases all memberships: `server/internal/service/iteration_lifecycle_apply.go:89`, service test `iteration_lifecycle_test.go:208`, handler test `iteration_history_test.go:321`.
- The screenshot discrepancy is expected from the current backend shape: before start, history builds current membership directly, while original/cumulative change counters stay empty; `statistics()` still returns effective minus initial effective (`history.go:152`, `stats.go:175`). Correct the display semantics; do not alter that numeric contract.

### Metric semantics, units and matching data

All counts below come from `server/internal/iteration/stats.go:28`, `:79`, `:116`, `:154`. `current` means as-of the projection, including cancelled items; a closed projection means scope immediately before closure releases.

| Metric | Exact meaning | Correct drilldown |
| --- | --- | --- |
| `original` | Unique tasks captured at actual start, including retained done/cancelled tasks | Original source, every item; preserve start-time facts. |
| `initial_effective` | Original tasks whose start category is not cancelled | Original source filtered to known noncancelled categories. |
| `current` | Current/as-of-closure members, including cancelled | Current source / snapshot scope, every item. For planned UI this is the plan task count. |
| `cancelled` | Current/as-of-closure members with cancelled category | Current source filtered to cancelled. Distinct tasks, not cancellation occurrences. |
| `effective` | Current members excluding cancelled; done still counts | Current source filtered to known noncancelled categories. |
| `completed` | Current effective members in done | Current source done. Tasks completed after leaving do not contribute. |
| `original_completed` | Current done tasks that also belong to the original set | Intersection of original identity and current done; start-time completion alone is not the numerator. |
| `remaining` | Effective minus completed | Current known nonterminal, noncancelled members. |
| `added_unique` | First post-start participation of each task that was not in the original set | One row per distinct canonical post-start `join` issue, with its first-addition event evidence. Keep it even if later removed/deleted. Do not include `reenter`. |
| `removed_events` | Number of actual present-to-absent transitions after start | Exact post-start `leave` and `delete` records, including removal of an already cancelled task (effective delta 0). Occurrences, not unique tasks. |
| `reentry_events` | Absent-to-present transitions for a task seen at start or during earlier post-start participation | Exact post-start `reenter` records. Repeated returns of one issue count repeatedly. |
| `cancel_events` | Within membership, known noncancelled → cancelled transitions | Exact matching status transitions; `done → cancelled` also counts. |
| `reopen_events` | Within membership, done or cancelled → a nonterminal category | Exact matching status transitions; includes ordinary done → todo (effective delta 0), excludes cancelled → done. |
| `started` | Current effective members whose current participation has execution/start evidence, including status in_progress/in_review/done; evidence is sticky only until leave | Do not infer from status alone or task title. Existing historical issue DTO does not expose `has_started`; omit this drilldown in the narrow delivery. |
| `net_effective_change` | Effective minus initial effective | Select post-start scope-impact activity. It is a signed difference, not a task membership set. A net 0 can still have many real additions/removals. |

- `added_unique` is not “currently present late tasks”: an added issue can leave and still count. Start originals reentering never become added-unique tasks. Planned join → planned leave → actual start → first active join counts as an addition, not reentry. Writer `PrepareMembershipChange` uses `p.InOriginal || HasIterationActiveJoin` (`membership.go:71`), then emits `join`/`reenter` (`:139`). Regression: `iteration_lifecycle_test.go:561`.
- Raw event names alone are insufficient for cancel/reopen: `RecordIssueChange` emits `reopen` for cancelled → done (`record.go:179`), but `ScopeState.apply` excludes terminal-to-terminal transitions from `reopen_events` (`stats.go:132`). This transition restores effective scope +1. Done → todo counts a reopen with effective delta 0. See `stats_test.go:153`.
- Canonical reducers ignore redundant absent leaves and do not count repeated present-to-present joins as new membership. Real writers suppress no-ops (`membership.go:87`). Therefore record selectors may rely on the canonical writer kinds, but must compare selected cardinality with the server metric and degrade if inconsistent; do not add a new replay reducer to repair corrupted/legacy streams.
- Completion ratios are `completed/effective` and `original_completed/original`; net ratio uses initial effective. Zero denominator is null/Not applicable, never a false 0% (`stats.go:146`).

### Statistical window, event sequence and closure

- Locate the one `start` event with null issue ID. Its `sequence` is the boundary. `historyChanges` rejects missing/duplicate start markers for a started period (`history.go:259`). It ignores every event with sequence <= start and also ignores baseline, planned_activity and lifecycle/metadata events after start (`:281`).
- Start writes its marker before baseline events (`iteration_lifecycle_apply.go:79`); original capture writes one baseline per retained task (`iteration_closure.go:205`). Baseline formation is not post-start addition. Do not require baseline and start to have identical operation IDs as a new compatibility rule: real writers use one operation, but validated historical fixtures can carry different operation IDs.
- Event ordering is sequence-based, not timestamp-based. A planned delete and start/baseline can share the same clamped timestamp; date/time comparisons cannot partition them. Regression: `server/internal/handler/iteration_history_test.go:111`.
- Before actual start, even a raw `delete` event is a planning adjustment. Ordinary planned changes and membership use `planned_activity`; deletion deliberately retains raw `delete` (`record_delete.go:31`). Suppress post-start numeric impact for both.
- Closure appends `end`/`cancelled`, loads history and captures snapshot before membership releases (`iteration_closure.go:123`, `:152`). Apply then writes release/move records (`iteration_lifecycle_apply.go:53`, `:59`). `snapshot.events` includes the anchor and excludes those later source leaves. Do not mix `/events` results into the frozen panel.
- Closure destination rows cover the complete frozen scope and identify genuine rollover by a nonnull other target and a +1 rollover counter for a nonterminal issue (`history_snapshot.go:243`). Null destinations on done/cancelled tasks do not mean unfinished removal. Closure choices are separate from period removal-event counts.
- Bulk moves use one operation ID and source leaves before target joins (`membership_batch.go:14`). Operation grouping must not erase individual records or turn occurrence metrics into operation counts. Do not collapse a source and target across iteration identity.
- There is no current server writer emitting raw `rollover` for this flow. Source movement uses `leave`, planned target uses `planned_activity`, and handoff target gets a start/baseline. A generic raw `rollover` kind retained for UI compatibility is not by itself proof of ±1 effective impact or of a metric occurrence.

### Per-event effective impact without replaying statistics

Extend/reuse core `projectIterationEvent` rather than decoding raw JSON in views. Its current boundary distinguishes missing/malformed (`undefined`) from explicit clearing (`null`), validates reference strings, and derives readable saved titles (`activity.ts:118`, `:203`). It presently does not expose fact `issue_id` or `has_started`; add only the minimal typed fields the helper actually needs, or perform identity validation inside the same core decoder.

For a recognized canonical event in a proven post-start window, require stored facts to have the expected shape and a known category from backlog/todo/in_progress/in_review/blocked/done/cancelled. Validate fact issue identity against `event.issue_id`; an explicit mismatch is never a usable transition. Missing legacy fact identity/status may remain readable, but should not become a quantified impact unless the accepted compatibility contract proves attribution.

| Stored transition | Effective delta | Additional meaning |
| --- | --- | --- |
| Canonical join/reenter: before null, after noncancelled member | +1 | Added unique or reentry are separate selectors. |
| Canonical join/reenter into cancelled (legacy facts) | 0 | Current writer disallows cancelled admission, but do not turn saved cancelled into effective. |
| Canonical leave/delete: before noncancelled member, after null | -1 | Removal occurrence. |
| Canonical leave/delete: before cancelled member, after null | 0 | Removal occurrence, no additional effective shrink. |
| Existing member noncancelled → cancelled | -1 | Cancellation occurrence. |
| Existing member cancelled → known noncancelled, including done | +1 | Effective restoration; only nonterminal destination counts as reopen. |
| Existing member done → nonterminal noncancelled | 0 | Reopen occurrence, completion changes. |
| Existing member noncancelled → noncancelled | 0 | Completion/execution/metadata may change, effective scope does not. |
| Planning event or pre-start sequence | Not applicable to commitment | Render planning wording, not +0 post-start scope. |
| Baseline | Commitment established | No addition badge. |
| Start/end/cancel-iteration anchor | Lifecycle context | No task delta. |
| Unknown kind, malformed/null transition sides, missing/unknown category or conflicting identity | Unknown | Inspectable record; never silently coerce to 0, +1 or -1. |

The precise cancellation/reopen predicates use facts even when a recognized event has kind `status` or `issue_changed`; normal current writers emit specialized kinds, but server statistics derives counts from states. Keep any event-category filter separately named from server metric selectors: “restored effective scope” and `reopen_events` are not interchangeable.

### Complete collection and provenance safeguards

Existing live activity is already complete: `iterationActivityOptions(wsId, id)` (`activity.ts:8`) reads all limit=100 ascending pages before publishing one `Event[]`. It uses one protected authorization epoch/AbortSignal, validates workspace/iteration and individual event identity, rejects duplicate IDs/sequences/cursors, and restarts once on `cursor_stale`. Keep that query and its key; the old `iterationEventsOptions` is a single page and cannot support exact selections.

The server cursor binds workspace, iteration, iteration revision, scope revision and query filter (`handler/iteration_history.go:117`, `:148`). Issue cursors additionally bind a digest of original/current display projection because priority/labels can change without scope events (`:353`). Event endpoints accept only cursor/limit/after_sequence and return ascending records; do not implement category/search by requesting a filtered partial event stream.

Important limitations and mitigations:

- A successful complete traversal proves its own collection, not that a separately cached detail/statistics response is from the same revision. `/events` has no top-level scope revision or last sequence; `iterationActivityOptions` returns only `Event[]` (`iteration-schemas.ts:178`). `calculated_at` and equal timestamps cannot prove coherent revisions.
- Cardinality comparison is a necessary sanity check for an exact selector, not proof of cross-request revision identity. Equal-count membership replacements can pass it. For event selections, label evidence as activity records and preserve the server number; if the UI requires a strict same-revision guarantee for live activity/counters, a frontend-only read choreography can capture detail before/after traversal and accept it only when revision/scope_revision are unchanged, retaining authorization cancellation. This is extra scope; do not hide the limitation behind a second reducer.
- Original/current complete issue results already expose `scope_revision`; compare it with the detail iteration when claiming an exact current task-set metric. Counts alone are insufficient. A frozen snapshot provides all these inputs together and does not have this cross-query gap.
- Unknown post-start kinds or incomplete status facts can affect evidence completeness. Keep records inspectable, mark affected metric selection unavailable/incomplete, and offer All activity. Do not change a missing collection into `[]` and call it a successful empty history.
- Transport errors, stale refresh, unavailable support and a genuine zero result are different states. Keep a prior authorized complete result on temporary errors. On 401/403 or explicit workspace-access-denied 404, existing `protectIterationRead` clears protected data; check `hasIterationReadAccess` too because cancellation can mask the original error (`access.ts:20`, spec `iteration-operations.md:246`). An iteration-not-found 404 only clears that entity's history (`activity.ts:55`).

### Existing original/current task-list APIs

- `api.getIterationIssues(wsId, id, params)` uses `IterationIssuesSchema`: items, total, scope_revision, next_cursor, optional filter metadata (`iteration-schemas.ts:167`, `client.ts:4515`).
- `scope=original` selects start-time originals; omitted/`scope=current` selects current scope. For a started closed period both are from the snapshot, even though actual live issue pointers have been released (`handler/iteration_history.go:362`, `history.go:89`).
- Current source is captured from live issues while active/planned; original source stays historical, including deleted originals (`history.go:120`, `:132`). Historical labels/priority are nullable/optional; absence is unknown, not an empty collection (`iteration-schemas.ts:93`).
- Existing filters include positive status category/key, search over title+identifier, project, typed assignee, priority and label (`handler/iteration_history.go:181`, `:222`). There is no `effective=true`, `exclude_cancelled`, arbitrary ID-set, `added_unique`, or event-metric filter. Do not invent unsupported query parameters.
- For a complete effective subset, lazily reuse `iterationGroupedIssuesOptions(wsId, id, { scope: "current" })`; for initial effective use `{ scope: "original" }`. This helper traverses all pages, checks stable scope_revision/total, duplicate task/cursor and final items.length===total, returning the whole set (`core/iterations/index.ts:90`). It does not automatically restart stale cursors; a retry must start from the first page. Do not call it just to populate filter options.
- Filter a successful complete array in the pure helper and check selected cardinality against the server count. A single visible page is not evidence for a full task-set metric. For ordinary all-current/all-original lists, paginated UI can truthfully show server total without pretending the visible page is complete.
- In frozen mode filter `snapshot.original`/`snapshot.scope` directly; avoid extra requests and all current issue joins. Historical records do not prove that a task is still accessible today. Keep saved titles/status in the drilldown and navigate using the authoritative issue ID through the existing platform adapter; let current task access/deletion handling remain separate.

### Proposed minimal helper interface

Recommended module: `packages/core/iterations/scope.ts`, exported by the existing iterations index. Keep `activity.ts` as the one raw-fact decoder. These are suggested interface boundaries for the design, not additional requirements to build a generic projection framework.

```ts
type IterationScopePhase =
  | "planned" | "active" | "completed"
  | "cancelledBeforeStart" | "cancelledAfterStart" | "unknown";

type IterationScopeImpact =
  | { kind: "effective"; delta: -1 | 0 | 1 }
  | { kind: "planning" | "baseline" | "lifecycle" | "unknown" };

type IterationScopeMetric =
  | "original" | "current" | "initial_effective" | "effective" | "cancelled"
  | "added_unique" | "removed_events" | "reentry_events"
  | "cancel_events" | "reopen_events" | "net_effective_change";

function iterationScopePhase(
  iteration: Pick<Iteration, "status" | "started_at">,
  snapshot: IterationSnapshot | null,
): IterationScopePhase;

type IterationScopeProjection = {
  phase: IterationScopePhase;
  startSequence: number | null;
  statistics: IterationStatistics; // The chosen authoritative input, not a reducer result.
  events: readonly {
    entry: IterationActivityEntry;
    impact: IterationScopeImpact;
    metrics: readonly IterationScopeMetric[];
  }[];
  evidenceComplete: boolean;
};

function projectIterationScopeActivity(input: {
  iteration: Iteration;
  statistics: IterationStatistics; // Only compare/select; never recompute it.
  snapshot: IterationSnapshot | null;
  events: readonly IterationEvent[] | undefined; // Successful complete live query only.
}): IterationScopeProjection;

// Select supporting evidence, preserving original event IDs and the source of
// saved task labels. Never produce a new Statistics object.
function selectIterationScopeMetric(
  metric: IterationScopeMetric,
  projection: IterationScopeProjection,
):
  | { kind: "scope"; scope: "original" | "current";
      filter: "all" | "effective" | "cancelled" }
  | { kind: "tasks"; issues: readonly IterationActivityIssue[];
      eventIds: readonly string[]; complete: boolean }
  | { kind: "events"; eventIds: readonly string[]; complete: boolean }
  | { kind: "unavailable" };
```

`IterationSnapshot`, `IterationStatistics` and `IterationActivityIssue` above are local aliases of existing inferred schema/entry types, not new wire contracts. The view can pass the chosen scope array into a simple pure predicate; do not require every metric click to populate all possible task sets. `added_unique` returns one issue per first canonical join, with a saved label and evidence event; occurrence selections keep multiple event IDs for the same issue. A net-change selection returns activity, and must not require its number of records to equal the signed net statistic. For other exact event/unique-task selections compare the selected count with the corresponding server field; return incomplete/unavailable on discrepancy.

If minimizing exports further, phase and metric selection can be methods/results of one pure projection. Preserve the substantive boundaries: no fetching in the helper, no views JSON decoding, no replayed replacement statistics, and explicit provenance/unknown states. Baseline lists can use original API/snapshot directly instead of implementing a separate baseline reconstruction algorithm.

### Meaningful regression cases

Keep pure matrices in a Node Vitest suite beside the helper; mounted views should test wiring/accessibility and selected business examples, not duplicate the whole matrix.

1. Plan with 2 tasks: plan count 2; no initial 0, net +2, post-start-added 0. Planned raw delete has planning semantics.
2. Active with start marker and zero baseline: still running; later first join is +1/addition. Null ratios stay Not applicable.
3. Cancel before start: no snapshot and no commitment; cancel after start: frozen snapshot and cancelled ending label.
4. Snapshot supplied while stale metadata says active: all metrics/activity/task sets use snapshot and never fetch live activity.
5. Same timestamp for planning delete, start, baseline and later change: cutoff follows sequence; baseline never contributes additions.
6. Task added then removed/deleted: still in added_unique drilldown; removal has one occurrence; current list excludes it.
7. Original leaves/reenters; added task leaves/reenters twice: original never becomes added_unique, added task appears once there, every return remains an occurrence.
8. Planned join/leave before start followed by first active join: added unique, not reentry.
9. todo → cancelled: delta -1/cancel occurrence; cancelled → todo: +1/reopen occurrence; done → todo: 0/reopen occurrence; cancelled → done: +1/no reopen occurrence; done → cancelled: -1/cancel occurrence.
10. Cancelled task leaves: removal occurrence with delta 0; ordinary done transition or execution start: effective delta 0.
11. Unknown category, malformed side, absent vs null, mismatched stored issue ID, unknown raw kind: preserve record/unknown state, no invented numeric impact or complete selection.
12. Missing/duplicate start marker or partial history: post-start metric evidence unavailable, never an authoritative empty result.
13. One bulk operation affecting several issues: occurrence counts and deduplicated task counts remain distinct; original records retained after grouping/search.
14. Source snapshot with closeout carried/removed tasks and terminal-null destinations: period removals do not include post-snapshot release records; closeout carry evidence remains frozen.
15. Net 0 with +1 join/-1 leave: metric selects both contributors, not an empty task set. Changing UI filter/search never changes top statistics.
16. Live membership list scope_revision differs from detail, or selected cardinality disagrees with the counter: show mismatch/refresh state; do not replace server count.
17. Complete all-pages current/original drilldown includes task beyond page 100, detects duplicate/stale pages and distinguishes failure from zero matches.
18. Frozen task renamed/deleted/moved after closure: saved labels/status and metric membership remain unchanged; current-link failure does not rewrite historical fact.

Reuse existing query/authorization evidence rather than writing duplicate tests: `activity.test.ts:61` (multi-page/stale), `:101` (duplicate rejection), `:122` (retained complete data), `:132` (entity deletion), `:159` and `:175` (session/revocation), `:194` (cancel/reopen and unknown semantics). Server canonical counts: `stats_test.go:19`, `:41`, `:62`, `:146`, `:153`. Frozen immutable/malformed payloads: `history_snapshot_test.go:36`, `:52`, `:100`, `:207`. No Go behavior changes are proposed.

## Related specs and external references

- `.trellis/workflow.md` — persisted research and role isolation; read before inspection.
- `.trellis/spec/core/frontend/iteration-operations.md:184` — complete activity, frozen sources, scope/ratio units, stored unknown/null and authorization.
- `.trellis/spec/core/frontend/iteration-operations.md:309` — current/original filter metadata and complete grouping semantics.
- `.trellis/spec/server/iterations.md:172` — lifecycle/history writer boundaries, start marker and immutable snapshot contracts.
- `.trellis/spec/core/frontend/type-safety.md` — API parsing with explicit null/unknown behavior.
- `CLAUDE.md` — pure core ownership, server state in React Query, one canonical test layer, no new dependencies.
- `docs/plans/2026-10-04-work-management-prds/iterations-prd.md:122` — actual-start original commitment; `:150` planning changes excluded from post-start scope; `:170` frozen ending; `:174` planned versus active cancellation.
- External references: none consulted; this is an existing internal contract task using current TanStack Query/Zod APIs. No dependency/API version migration is needed. The existing iteration wire/snapshot schema is version 1 (`iteration-schemas.ts:20`, `:111`).

## Caveats / Not Found

- `/events` does not expose collection revision, total or shared detail snapshot identity. A complete standalone traversal plus equal counts cannot by itself prove cross-request atomicity. Be precise about this limitation in the final interface/UX; do not invent revision tokens.
- Event fact schemas intentionally accept unknown JSON. Current server records ordinary `IssueFacts` without identifier/project/assignee display names (`record.go:19`); baseline and historical issue records contain full saved display identity. Missing labels must stay unknown; do not hydrate today's issue into an old event.
- Current source can be obtained directly from the existing issue endpoint; there is no complete live `scope` array in a detail response. Avoid building a second membership state reducer simply to reconstruct it.
- `started` task membership cannot be reliably inferred from the public historical task DTO alone, because current-participation `has_started` is absent. Leave that optional drilldown outside this delivery.
- Research confirms source/test contracts, not runtime verification. Implement/check agents must run the relevant core/component and browser checks.
