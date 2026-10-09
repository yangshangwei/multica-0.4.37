# Confirmed iteration operations

The I1 APIs use complete server previews and durable request identities. Shared
implementation lives in `packages/core/iterations` and `packages/views/iterations`;
Web and Desktop only wire routes/platform adapters. The service supports I1 without a deployment flag; each workspace starts disabled and is enabled by a human owner/admin in Settings → Iterations.

## Request and permission boundaries

- `prepareIterationDraft` derives remaining work and starting terminal choices
  from one complete preliminary preview, then requests the final confirmation.
  Do not replace it with one HTTP request per issue or client-visible pages.
- `useIterationCommand` persists the complete original payload and request ID
  under server/actor/workspace/action identity. Unknown results recover through
  GET or the same exact POST, never a newly generated ID.
- Distinguish a lookup `operation_not_found` from a write `iteration_not_found`.
  The former retains the original identity; the latter is definitively rejected
  and permits editing a new intent.
- `isIterationAccessDenied` includes authenticated 401/403 and the middleware's
  explicit `404 workspace_access_denied`. That 404 must not become an old-server
  capability fallback or trigger a recovery POST. Other resource404s do not
  revoke the whole workspace.
- `protectIterationRead` fences late responses with an authorization epoch and
  captured API session scope. Check the session before returning data or handling
  an access error: a delayed old-account 403 must not clear the new account
  query cache or persisted commands. Pass Query AbortSignal to underlying reads.
  On revocation, cancel queries with `revert:false` before erasing protected
  data; normal cancellation can otherwise restore the stale successful result.
  Clear mutation payloads, persisted pending commands and active observer data.

## UI ownership and recovery

- Key period detail/workspace and issue assignment boundaries by stable entity
  identity, not revision. A cached A→B navigation must not reuse A's hidden
  issue ID, editor fields, cursor or preview.
- Lock inputs and confirmation while a preview is in flight (or explicitly
  discard obsolete generations). Clearing an old preview on input change is
  insufficient if its unresolved request can reinstall it later.
- Keep pending recovery independent of current period status, workspace enabled
  state and rollout availability. A WebSocket can announce completed before a
  successful HTTP response is lost. `IterationRecovery` still reads the exact
  original operation on the closed/reloaded page; status alone proves nothing
  about which request committed.
- `pending-signal.ts` only signals durable client command changes; server state
  remains in Query. Its external-store snapshot is a stable revision number.
- Ordinary issue/task/project/member/status events also affect iteration
  projections. The realtime classifier invalidates them before the generic
  event handler's specific-event early return, with workspace-scoped debounce.

## Route and display integration

Adding a route also requires desktop tab subject/presentation/icon/name lookup,
diagnostic path bucketing, editor internal-link registration and their parity
tests. Router wiring alone can render a working page under an "Unknown page" tab.
Use saved period timezone for historical timestamps, localized frozen categories,
and actionable validation text. The name limit counts Unicode code points.

Canonical regressions: core `command.test.tsx`, `access.test.ts`, `prepare.test.ts`,
`realtime.test.ts`, the mounted realtime WS-instance suite, views iteration
navigation/operation/assignment suites, and `e2e/iterations-i1*.spec.ts`.
Native E2E must proxy WebSocket upgrades as well as HTTP; otherwise assertions
on server state can pass while the actual rendered statistics stay stale.

Assignable iteration choices require both a supported manual mode and a planned
or active status. Unknown modes remain visible through read-only catalogue/detail
surfaces but must not become editable through issue create or batch selectors.
Optional legacy rollover counts and historical metadata remain unknown; never
render missing values as known zero or an empty label collection.

## Edit baseline and refresh recovery

`IterationForm` owns editable values and their baseline revision together.
`api.updateIteration` receives changed mutable `fields` and that baseline's
`expected_revision`. A clean form may adopt a newer resource atomically; a dirty
form must not borrow a live prop's newer revision for older field values.

| Situation | Required behavior |
|---|---|
| Remote revision changes during editing | Keep local values and old baseline; omit untouched fields |
| `iteration_revision_conflict` | Retain input, fetch current resource, offer explicit comparison/resolution |
| User applies local changes onto reviewed server version | Preserve untouched server fields; require a new explicit save |
| Known successful write and successful resource refresh | Advance values and baseline together |
| Write committed but resource refresh failed | Lock further writes; retry the read, not the committed command |
| Unknown write result | Recover original payload/request ID through `useIterationCommand` |
| Auth/session change or access denial | Do not adopt delayed results or display revoked data |

Operation receipts have no entity revision; never invent one. Transient
background capability/settings/detail query failures with cached data, including
HTTP 408 and 429, must not unmount the draft. Display a retry affordance while
retaining it. Access rejection or definitive deletion (including 401/403/404)
still removes the protected surface. Do not classify every 4xx as permanent.

Good: baseline revision N + changed local description rejects against server N+1.
Base: clean refresh updates fields and revision together. Bad: mount-only fields
plus `iteration.revision` from live props silently overwrites another user's edits.

Canonical tests: views `iteration-form.test.tsx` covers baseline/conflict, status,
post-commit refresh and exact retry; `iteration-navigation.test.tsx` covers parent
query errors retaining the editor. Core command/access suites remain the owners
of request persistence and authorization epochs.


## Workspace settings and business-page composition

- `IterationSettingsTab` is always discoverable at `settings?tab=iterations`.
  Use `supported/manual` for version compatibility and `settings.enabled` for
  the persisted workspace choice. Do not reintroduce `iterations_i1` or a
  client-owned enablement flag.
- `WorkspacePlanningTimezone` is edited only in workspace General settings
  (`settings?tab=workspace`), because projects and new iterations share this
  workspace value. Iteration settings show `settings.effective_timezone` as
  read-only context and link to that editor; do not mount another timezone form
  or make iteration availability depend on an editor callback. Enable sends
  the displayed saved value as `confirmed_timezone`; loading, failed refreshes
  and pending operations still block the switch. A stale-timezone rejection
  refreshes settings before another enable attempt. Read the effective timezone
  from the server (the shared fallback is currently `Asia/Shanghai`), never
  infer UTC from a missing value. Existing iterations keep their saved timezone.
- `IterationOperation` controlled `open/onOpenChange/hideTrigger` supports the
  switch and action menus without replacing complete preview/recovery logic.
  `IterationRecovery` stays outside supported/enabled and tab visibility gates.
- Successful commands and `iteration:updated` refresh iterations and triage
  settings' `iteration_assignment` capability; shared timezone queries must also
  remain current. New enable publishes only after commit, not on receipt replay.
- `iterationCatalogueOptions` is the single complete metadata traversal, including
  repeated identity/cursor detection and one full restart on `cursor_stale`.
  Do not give the same cache key a weaker view-local query function.
- `iterationTimeline` derives globally stable upcoming identity and descending
  groups. Its gaps require a complete unfiltered catalogue, comparable saved
  timezones and no ambiguous intervening intervals; hide gaps during refresh or
  error. Metadata rows do not fetch per-row statistics.
- Detail tabs retain mounted task filters and visited progress/event state.
  Editors and confirmed operations live outside tab panels; change entity keys
  on workspace/period navigation, not revision changes. Frozen views take
  `snapshot.statistics`, never current filtered tasks.

## Visible state and readable progress

The workspace settings status is derived from the authoritative settings query.
Its existing state label is a live region and also describes the switch. Do not
render an earlier enable mutation's persistent `isSuccess` as the current state:
another mutation or client can already have disabled the workspace. Same-mount
enable → normal disable → re-enable is the canonical feedback regression; a
response-loss test that reloads the page does not cover that mutation lifetime.

Business-page first reads and background refreshes have different presentation
needs, but neither may turn an error into zero statistics. A temporary first-read
failure offers a disabled-while-fetching Retry plus safe navigation. A cached
temporary error retains the draft and read data. Definitive access denial or
deletion removes protected details and charts; never show the cached overview
curve after a 401/403/404. Read-specific error copy must not claim an uncreated
edit draft was preserved or ask the user to confirm an operation preview.

Detail headers display the period's saved IANA timezone even when it equals the
workspace planning timezone. This is display-only: changing shared planning
settings must not alter saved period dates/timezones or frozen facts. Timeline
date columns use locale formatter parts to keep day suffixes with their number.

`IterationChartTable` and `IterationProgressChart` take the same `statistics`
object. Overview provides a keyboard-operable native disclosure next to the
chart; detail reuses the table with its caption and row/column headers. Prefer a
returned snapshot's statistics even if stale catalogue metadata still says
active, and never request details for every planned/history row to populate the
table. Preserve sparse real chart dates rather than filling invented history.

Overview tab triggers and timeline panels share one `Tabs` root. Each selected
tab controls an associated `TabsContent`; retain Base UI's keyboard model
(direction keys move focus; activation follows the component's supported
Enter/Space behavior). A styled tablist next to an unrelated section is not a
complete tab interface. Filters and creation drafts remain owned above panels.

Canonical coverage: settings tests for same-mount state and cancellation/error;
navigation tests for first reads, stored timezones, definitive error eviction and
tab/panel associations; `iteration-chart-closeout.test.tsx` for shared table and
frozen values. The Web/Electron settings E2E separately checks normal toggling
without a reload and unknown-result recovery with the original request identity.
When a modal is open, a background-switch assertion may explicitly include the
hidden background role; do not remove the assertion or disable modal isolation
to make a default visible-role locator succeed.

Canonical regressions: core `catalogue.test.ts`, `timeline.test.ts`; views
`iteration-navigation.test.tsx`, `iteration-settings-tab.test.tsx`; real settings
recovery and sibling-client refresh in `e2e/iteration-settings.spec.ts`.

## Progress and complete scope activity

### Scope and trigger

Progress and Scope Changes are projections of one iteration. Keep identity,
saved timezone and the freeze notice in the shared header; task-only counters
belong inside the task panel. Delivery results, scope events and closeout
destinations are separate facts, even when their labels mention the same task.

### Signatures

- `iterationActivityOptions(wsId: string, id: string)` owns complete live history
  under `["iterations", wsId, "activity", id]`, with `retry: false`.
- `projectIterationEvent(event)` owns stored-fact decoding;
  `groupIterationActivity(events, "scope" | "all")` owns operation grouping;
  `iterationActivityDays(groups, timezone)` owns contiguous saved-clock dates.
- `IterationEventsPanel({ wsId, id, timezone, statistics, snapshot })` renders
  scope counters and activity. `IterationProgressChart` keeps
  `showCompletionRate=true` for the overview; Progress passes false because its
  delivery summary already exposes the rate.

### Data contracts

The existing events API is ascending. Read all `limit=100` pages before sorting
or filtering; a reversed first page is not the latest complete history. Keep
the old single-page query separate. The complete query uses
`protectIterationRead` around the entire traversal, passes AbortSignal to every
request, checks response workspace/iteration identity, and detects duplicate
event IDs, sequences and cursors. One `cursor_stale` may restart the whole
traversal; never publish the failed attempt's partial items.

Order by authoritative sequence descending, not business timestamps. Group
records by iteration/operation identity, retaining every original record in
disclosures. Day buckets use the saved timezone and stable date/occurrence keys,
so prepending a same-day operation does not collapse already-open details.
Keep non-contiguous equal dates separate rather than reordering sequence.

The default scope filter includes membership changes, cancellations, baseline
and lifecycle anchors, and transitions across cancelled status. A reopen from
cancelled changes effective scope; done-to-todo changes completion only. Planned
membership activity retains its planning group label; ordinary metadata changes
remain available under All activity. Unknown kinds remain inspectable.

Historical titles, status/category and before/after values come from saved
facts. Explicit null means cleared/unassigned; absent or malformed values mean
unknown. Current actor/iteration directory names are display references only.
Only semantic moves get source/destination rows; non-moves must not produce
unknown-to-unknown arrows. Technical IDs stay available in event details.

Closed panels use `snapshot.statistics` and `snapshot.events` directly and do
not fetch live activity. Effective completion displays completed/effective;
original commitment displays original_completed/original. Zero denominators or
unknown ratios show Not applicable. Net scope change is effective minus
initial_effective. Added unique tasks and event counts retain distinct units.

Closeout summaries include known frozen nonterminal tasks only. A completed or
cancelled task with a null target is not an unfinished removal. When legacy
scope/destination facts are incomplete, show unknown instead of inventing a
zero count. Preserve the complete destination list in the closure disclosure.
Single-day charts show the actual date and values compactly; complete semantic
chart tables remain available through native, keyboard-operable disclosures.

### Validation and error matrix

| Condition | Required result |
|---|---|
| Multi-page live history succeeds | Publish one complete collection, then filter/group and reveal more groups locally |
| First stale cursor | Discard the attempt; restart once from the first page |
| Second stale cursor, repeated ID/sequence/cursor, mismatched identity | Reject the read; do not expose a partial successful result |
| Temporary refresh failure, including 408/429/409 | Keep previously authorized complete data and view state; offer retry |
| Workspace revocation | Remove all protected counters and events, even when cancellation leaves `CancelledError` |
| Iteration 404 | Remove this entity's cached history without revoking sibling iterations |
| Frozen snapshot | Read snapshot facts; no live activity request |
| No events versus no scope matches | Distinct empty states; All activity remains available for filtered-out records |

Use the existing `hasIterationReadAccess(client, wsId)` alongside the error
classification in the panel. Checking only `error instanceof ApiError` misses
the cancellation outcome produced by authorization-cache eviction.

### Good, base and bad cases

Good: original 4/current 5/effective 4/completed 2/original_completed 1 displays
2/4 (50%) and 1/4 (25%). C carries and D leaves at closure; during-period removals
remain 0 events while unfinished closeout removals are 1 task.

Base: a one-day closed iteration shows its actual single sample and a closed
data-table disclosure. A multi-day overview retains its completion-rate summary.

Bad: subtracting removal-event counts from unique additions, counting terminal
null destinations as unfinished removals, or replacing missing historical facts
with today's task would make a cleaner-looking UI report false history.

### Required tests

- Core `activity.test.ts`: complete cross-page operations; one stale restart;
  cancellation, identity/repetition rejection and authorization fencing; stored
  null/unknown differences; reopen boundaries; stable date groups.
- Views `iteration-events.test.tsx`: scope/all filtering, reveal-more,
  temporary-error retention/retry, revoked data removal and open-detail retention.
- `iteration-navigation.test.tsx`: combined frozen C/D plus terminal-null
  fixture, both denominators, closeout counts distinct from period events, no
  live activity fetch for a snapshot, retained filters/editors across tabs.
- `iteration-progress.test.tsx`, `iteration-history.test.tsx` and
  `iteration-chart-closeout.test.tsx`: zero/one/multiple-day states, unknown
  ratios, complete table cells and immutable chart data after live data changes.
- Web/Electron closure fixtures open View chart data / View closure details
  before checking hidden facts. All activity plus the start-operation disclosure
  exposes baseline titles; keep every original frozen-value assertion.

### Wrong versus correct

```ts
// Wrong: filtering a first page can claim no scope changes while later pages exist.
const page = await api.getIterationEvents(wsId, id);
const groups = groupIterationActivity(page.items, "scope");

// Correct: only a successful complete query supports latest-first filtering.
const history = useQuery(iterationActivityOptions(wsId, id));
const groups = groupIterationActivity(history.data ?? [], filter);
```

The view must still distinguish undefined/loading/error from a successful empty
collection; the second example is the grouping step, not an empty-state policy.


## Iteration filters and accessible controls

### Scope and trigger

Iteration task filters consume the original/current historical projection, including closed snapshots. Opening a filter must not download every task page merely to derive its choices. Iteration text/status, disclosure and confirmation controls share the app's UI primitives and retain the existing complete-preview/recovery semantics.

### Signatures

- `GET /api/workspaces/{workspace}/iterations/{iteration}/issues` retains its query and cursor contract and adds optional `filter_options`.
- `IterationIssueFilterOptionsSchema` validates compact metadata at the API boundary; `IterationIssuesSchema` discards malformed optional metadata without discarding an otherwise valid task page.
- `iterationGroupedIssuesOptions(wsId, id, params)` retains the first complete page's metadata while validating the complete grouped member collection.
- `IterationsPage({ mainLandmark?: boolean })` defaults to the platform shell's main landmark. Only Desktop iteration routes opt into the outer page landmark; do not change Desktop's shared canvas to main while sibling pages own their own landmarks.

### Contracts

`filter_options` contains `statuses: string[]`, `projects: {id: UUID|null, name: string|null}[]`, `assignees: {type: string|null, id: UUID|null, name: string|null}[]`, and `labels: {id: UUID, name: string}[]`. A typed assignee's type and ID are both present or both null. Derive distinct, sorted choices from the complete selected source before search, field filters and pagination. Preserve stored names, including deleted/renamed historical references. Missing names with nonnull identities remain unknown; null identities may render unassigned.

A missing metadata field from an older server disables only metadata-dependent selectors and explains that search, scope, priority and grouping remain available. Never add the previous all-pages fetch as a fallback. Explicit grouping still reads all coherent pages because it requires all members. Search/filter/scope/group/entity changes reset cursor history; stale-page retry returns to the first page.

Use `warning-foreground`, `info-foreground` and `destructive-foreground` for readable status/control text, retaining warning/chart/destructive fill tokens. Focusable shared TabsContent has a 2px foreground outline inset into its scroll boundary. Audited controls and select popup options use `pointer-coarse:min-h-11`; icon and short-reference targets also need sufficient width. Shared `iterationDisclosureClass` owns native disclosure focus and hit targets. Long live/historical task titles use explicit `overflow-wrap:anywhere`; document-level overflow checks cannot detect content clipped inside a scroll panel.

Lifecycle confirmation buttons name their operation. Cancel/delete/disable use destructive styling. Required reason cues stay alongside the associated label, avoiding duplicate label-text matches caused by including auxiliary cue text inside it. Base UI Checkbox/Select may add hidden form inputs: browser/component tests interact through named roles and actual options. Preview summaries retain complete task/destination facts, running execution semantics, immutable historical values and request identities. Global recovery names use cached human identity when available; without it, same-operation/same-reason requests include the stored subject/task identity. Original request IDs remain available in a secondary disclosure.

### Validation and error matrix

| Condition | Required behavior |
| --- | --- |
| Opening filters on a paginated scope | No extra iteration-issues request; choices include facts beyond the visible page |
| Search/filter shrinks the visible page | Choices still describe the complete chosen source |
| Scope switches to original | Use original historical choices and reset paging |
| Metadata missing or malformed | Read valid tasks; explain unavailable advanced choices; do not fetch all pages |
| Stored ID exists but name is absent | Display unknown, not unassigned |
| Temporary refresh error with authorized data | Retain content and view state; expose a secondary retry |
| Permission loss or definitive deletion | Remove protected rows and metadata |
| Two pending operations share type/reason without cache | Visible recovery names still identify different subjects |
| Theme transition is still running | Await finite CSS transitions before measuring the final state colors |

### Good, base and bad cases

Good: a label present only on task 61 remains selectable from the first page, and a frozen label retains its saved name after live rename/delete. Base: an old server returns tasks without metadata; search and priority remain useful. Bad: deriving choices from 50 visible rows, silently calling the complete grouped query on filter-open, or rendering a missing stored project name as None.

### Required tests

- Handler `iteration_filter_options_test.go`: complete, unfiltered, paginated and frozen projections, typed identities and stored names.
- Core `iteration-schemas.test.ts`: old response compatibility and malformed optional metadata; `grouped-issues.test.ts`: metadata propagation with coherent full collection.
- Views `iteration-details.test.tsx`: real filter controls, zero all-pages filter traversal, paging/reset and unknown historical references; `project-iterations.test.tsx`: previous-page and project identity reset; `iteration-recovery.test.tsx`: selected original request and same-type/reason fallback distinction.
- Real `iterations-audit-desktop.spec.ts`: stable rendered contrast, panel focus, control roles, coarse bounds, container overflow, operation previews and bilingual captures. `iterations-i1-desktop.spec.ts` covers real original-request recovery and atomic handoff.

### Wrong versus correct

```tsx
// Wrong: filter-open downloads every member and can still hide historical facts.
const choices = useQuery({ ...iterationGroupedIssuesOptions(wsId, id), enabled: filtersOpen });

// Correct: the task response already contains complete source metadata.
const choices = result.data?.filter_options;
```

For Select assertions, assert the selected `data-slot=select-value` text rather than the entire trigger's textContent, which also contains the decorative icon. Isolated Desktop E2E may set `MULTICA_E2E_DESKTOP_OUTPUT_DIR` to a separate production build; do not overwrite a running checkout's output. Authentication fixture scripts initialize locale/theme only for a new token, retaining a user's choices across reloads within that same synthetic session.

When testing themes, exercise the real ThemeProvider preference handler so React state, the dark class and native color-scheme change together. Manually toggling only the class leaves date icons in the wrong native palette and creates a false contrast finding; assert the actual date input color-scheme before accepting dark-mode captures.
