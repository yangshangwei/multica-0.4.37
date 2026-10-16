# UG 最终关单（2026-10-07）

**UG passed。** 本轮100项views回归、生产Web/Electron四项I1 E2E、94/pass最终视觉、移动196项兼容回归已完成；12张最终截图和源码指纹保留。详见[最终验收](../10-05-iterations-i1-verification/verification.md)与[UI审查](../10-05-iterations-i1-verification/ui-final-audit.md)。开关默认关闭、未提交或发布；移动真机和旧二进制范围限制如实记录。以下实现过程及pending措辞保留为历史。

# UG implementation evidence (2026-10-06)

Status: implementation in progress; **UG not yet accepted**. Existing approved task tree retained. No commit, push, merge, deployment, dependency installation or release enablement performed by the client agent.

## Implemented client surfaces

- `packages/core/api/iteration-schemas.ts`: validated identities, revisions, statistics, frozen histories, draft/preview/write receipts. All lifecycle/settings/read endpoints in existing API client; authenticated old-server capability probe. Optional issue fields preserve unknown rather than inventing rollover zero.
- `packages/core/iterations/`: workspace/connection scoped query keys, complete paginated target choices, complete closure draft preparation, persisted exact-payload command recovery and confirmed-rejection correction.
- `packages/views/iterations/`: capability-gated list/settings/create/edit/detail; complete preview/confirm flows for start/end/cancel/handoff/disable/delete; per-issue destinations and terminal choices; assignment with existing issue picker and secondary identifier input; statistics chart plus accessible data table; original/current scope and paged events/history.
- Web `/[workspaceSlug]/iterations[/id]` and Desktop `iterations[/id]` mount the same page. Sidebar entry is capability-gated. Issue creation/detail, T1 acceptance, project iteration filter, inbox navigation, and WS/reconnect invalidation use the shared contracts.
- English/Chinese copy added under existing locale namespaces; release flags unchanged.

## Executed narrow checks

- `pnpm --filter @multica/core exec vitest run api/iteration-schemas.test.ts api/iteration-client.test.ts iterations`: **41 passed**, 5 files (latest 23:38 local run). Includes malformed read/write responses, capability 400/401/403/404/503 distinction, mixed identities, unsafe revisions, unknown statuses, complete pagination, done exclusion, reload/network recovery and original payload replay.
- `pnpm --filter @multica/views exec vitest run iterations`: **3 passed**, 3 files. Capability gate, keyboard submission/long input/409 correction, chart data table.
- `pnpm --filter @multica/views exec vitest run triage/triage-action-dialog.test.tsx projects/components/project-detail.test.tsx modals/create-issue.test.tsx inbox/components/inbox-page.test.tsx`: **114 passed**, 4 files.
- `pnpm --filter @multica/views exec vitest run iterations/iteration-page.test.tsx inbox/components/inbox-display.test.ts locales/parity.test.ts`: **81 passed**, 3 files (overlaps other counts; do not sum as unique tests).
- Views typecheck passed after history/identity/recovery refinements (23:42 local verification).
- Scoped core lint passed. Scoped views lint: zero errors, two existing project-detail exhaustive-deps warnings.
- Impeccable mechanical detector for `packages/views/iterations`: `[]`. This is not rendered visual acceptance.

## Pending evidence

Root integration owns real Web/Electron E2E, visual/verdict evidence, full repository checks and CG/FCG/VG. Narrow mocked tests do not prove live API compatibility, timing, or full UG. Confirm pagination conflict recovery, revoked permission states, multi-target closure and start terminal choices through real integration. The final error-copy refinement passed the three iteration component tests. No rendered screenshot evidence is claimed by the client agent.

## Additional confirmed refinements

- Both completion ratios render with an explicit non-applicable state and task-count disclaimer; overdue stays active and default 14-day dates use the iteration timezone across DST.
- Existing P1 planning-timezone write hook is reused; current/history immutable date fields are disabled and omitted from writes.
- A stale preview rebuilds current revisions/full source membership while preserving per-task destination/terminal choices. Confirmed rejection invalidates authoritative reads without clearing form input.
- Shared issue picker supports task selection; whole target catalogs are loaded through all pages. The secondary Task ID input also accepts human identifiers through the existing issue loader.
- Client agent started no dev server, API, browser, or database process.

## Follow-up verification and formatting pass

Before formatting, add interaction regressions for unknown-result recovery, 403/409 preservation, stale cursors, revoked history visibility and independent terminal/destination choices. Fix only failures demonstrated by those tests. Then reformat the new iteration components into readable JSX without changing behavior, re-run the same tests, typecheck and scoped lint. No dependencies or runtime processes are added.

### 2026-10-07 follow-up evidence

- `iteration-operation.test.tsx`: **6 passed** after final preparation refactor. Real shared dialog exercises multi-target selection and 409 refresh, independent terminal choices, unknown-result lookup/exact-payload retry, first apply403, later preview-read403 (all titles/IDs/counts/target/source text hidden), and no ignored target selector for planned cancellation.
- `iteration-navigation.test.tsx`: **3 passed**. RED exposed duplicate stale-cursor fetch, missing issue-page retry and retained protected history cache. GREEN resets to the first page and clears active/inactive protected query results after revocation.
- `iterations/access.test.ts`: **2 passed**. Late responses cannot repopulate a revoked authorization epoch; a fresh successful capability check is required to restore access. Revocation also removes matching persisted commands and cached mutation data.
- `iterations/prepare.test.ts`: **7 passed** after replacing sequential per-issue reads. A 1,000-item source is derived from one complete preliminary preview; no individual issue or paginated history HTTP requests. Source remaining work and next-target terminal members are separated, incomplete/stale preview is rejected, final confirmation still uses a new complete server preview/hash.
- New components/helpers/tests formatted with an **already cached** Prettier executable; no package or dependency installed. Post-format views typecheck, scoped core/views lint and `git diff --check`: exit 0.
- Last complete pre-N+1 follow-up runs: own views **11 passed**, core selected suites **43 passed**. Final N+1 change has the targeted 7+6 checks above; counts overlap and must not be summed.
- No dev server/API/database/browser started by the client agent. Root owns isolated production-build Web/Electron acceptance.

### Real-browser name validation correction

Root's production Web screenshot/visual verdict (score 86, revise) showed that a name exceeding 200 characters received only a generic error. Added client field validation after CRLF/trim normalization using Unicode codepoint count, with a localized 1–200 hint, specific required/too-long errors, `aria-invalid`, `aria-describedby`, and invalid-field focus. Input is never truncated; no UTF-16 `maxLength` is applied.

RED component test demonstrated the missing field guidance. GREEN: `iterations/validation.test.ts` **9 passed** and `iteration-form.test.tsx` **2 passed**, including 201 astral characters rejected before HTTP and 200 accepted despite a UTF-16 length of 400. Views typecheck, changed-file core/views lint and diff check passed. Root owns the replacement rendered screenshot/verdict.

### Final tab, history display, locale and entity-boundary correction

Root's second real-browser/native visual review found an unknown Desktop tab label and raw microsecond timestamps. The shared tab page/resource registry now recognizes iterations, renders CalendarRange consistently, observes the cached iteration name, and falls back to localized “Iteration” after protected cache removal rather than retaining a persisted secret title. The command palette uses the same registry and gates iterations on confirmed capability support.

History closure/events reuse `formatInTimeZone` with the saved iteration timezone, the reader's locale, year and minute precision, semantic `<time dateTime>` and visible timezone. Period statuses and frozen task categories use localized labels; known validation codes explain corrective actions, unknown codes/categories use safe localized defaults.

A route-change regression demonstrated carried-over task search state. The workspace and detail boundaries now key only by workspace/entity identity (never revision), clearing previous-period form/filter/dialog/preview state when switching to a cached second period while retaining core persisted command recovery per entity.

Checks: core route/tab suites **62 passed**; tab/history/date-format view suites **30 passed**; locale/navigation/tab/search view suites **70 passed**; final entity-boundary/operation suites **12 passed**. These are overlapping runs, not additive totals. Final views typecheck, changed core/views lint and diff check returned zero. Root owns the final native tab assertion, refreshed screenshots and visual verdict; no new rendered acceptance is claimed here.

### Independent integration-review corrections

Three RED regressions reproduced retained task-A assignment state after IssueDetail moved to task B, editable destinations while a preview was in flight, and missing iteration invalidation from ordinary `issue:updated` events.

GREEN fixes key the inner assignment by workspace/issue/target identity, disable inputs and confirmation while preparation/preview is pending, and debounce live iteration projections for ordinary issue/task/project/member/agent/squad/status/workspace/triage events. Streaming task message/progress and comment/reaction events are excluded. Tests confirm deferred responses cannot confirm a destination different from the displayed selection; full WS-hook dispatch of two `issue:updated` events produces one iteration invalidation after the debounce interval.

Checks: assignment/operation component suites **11 passed**; full WS-instance suite plus event matrix **52 passed**; views typecheck, changed core/views lint and diff check passed. The reconnect assertion now includes the already-required iteration prefix (33 invalidations rather than the pre-I1 count of 32). Root owns the final shared snapshot/full-check run.

### Qualified 404 access and recovery boundary

Real workspace middleware uses HTTP 404 with `code=workspace_access_denied` to conceal revoked membership. Added shared `isIterationAccessDenied` in the API boundary (401/403 or that exact qualified 404), used by capability probing, read epochs/cache cleanup, command recovery and UI denial rendering. Capability revocation no longer triggers an older-server fallback probe; qualified operation lookup never retries POST. Pending state reacts immediately to authorization revocation and cannot resurrect from cleared storage.

RED tests also exposed cancellation restoring a query's pre-revocation data during observer teardown; cancellation now uses `revert:false` before protected query/mutation data is erased. Late reads remain blocked by epoch checks. Documented `iteration_not_found` write responses are definitive rejection and unlock retained form input/new intent; `operation_not_found` and untyped lookup 404 preserve same-ID recovery.

Final checks for this batch: core API/access/command suites **30 passed**, all iteration component suites **22 passed**, views typecheck, scoped lint and diff check exit 0. Tests cover no capability probe, no POST after denied lookup, cached title/persisted/pending cleanup, late response rejection, ordinary resource-404 access preservation and editable deleted-iteration write rejection.

### Recovery survives authoritative lifecycle changes

Root's real native WebSocket test reproduced a committed end followed by an aborted HTTP response: the status refetch unmounted the action dialog before recovery. Scoped operation controllers now remain mounted while pending or reporting failure, independently of whether that action remains available; unavailable controls cannot start a new preview/write. Workspace disable uses the same rule.

An independent recovery surface enumerates only validated persisted requests for the current server/actor/workspace, outside period-status, settings-enabled and rollout-supported gates. It performs original-request lookup and only retries the same payload/ID. A missing lookup followed by a definitive 409 remains visibly failed; neither a closed period nor disabled settings is treated as proof that this actor's request succeeded. Draft-storage signals keep recovery discoverable after changes and remount; server rendering uses an empty recovery snapshot until hydration.

RED/GREEN component regressions cover delayed failed HTTP response after a closed WS projection, live disable after settings refetch, reload of closed/disabled/rollout-off pages, and another actor's closure causing original-request retry rejection. Final checks: all iteration component suites **28 passed**, core iteration suites **50 passed**, views typecheck and scoped core/views lint passed. Root owns rerunning the real native WS scenario and final build snapshot.

### Original I1 completeness pass (integration in progress)

Added the omitted original PRD surfaces: current/future/history list grouping and earliest-future label; current count and scope-event counters; coordinator/manual mode/platform share link; date-overlap warning; frozen end reason, processing time and per-task destinations/counts; rollover review notice; actual current membership/count and source in assignment; ordinary batch entry using the exact resolved selection. First chunk passed 44 component tests, scoped ESLint and views typecheck.

The second chunk adds complete-filtered-set task grouping, status/assignee/project/priority/label filters, issue participation history, frozen/current task comparison and deleted-or-inaccessible live-task explanation. Grouping depends on the coordinated core `iterationGroupedIssuesOptions` query and additive historical priority/label schema fields; the isolated views wiring suite passed 5 tests, but full integration remains pending those core changes. Existing original-ID, epoch and recovery boundaries are retained. Recovery now reports processing and disables another lookup while the same command is still in flight; its mounted regression passed (assignment suite 5 tests). No UG, final E2E, performance or complete-check approval is asserted by this section.

### Replaced-session read isolation

A final review found that delayed comparison reads were fenced by workspace epoch
but not API session. The RED test reproduced both an old successful payload being
returned and an old 403 erasing replacement-account data after QueryClient.clear.
The read wrapper now captures session scope and rejects changed-session outcomes
before any return, access recovery or cache/storage cleanup. The UI forwards the
query AbortSignal as an additional cancellation path. A second regression excludes
unknown-mode periods from assignment choices while retaining read-only visibility.

Core iteration/API suites: 103 passed; core typecheck and scoped lint passed.
Logs: `.omx/logs/i1-verification/session-read-behavior-red.log`,
`session-read-green.log`, `core-final-lint.log`. These focused proofs do not replace
final native/Web and full-check evidence.

### Original completeness integration — source handoff

The coordinated core schema/grouping query and backend filters are now integrated. Active and future lists come from the protected complete catalogue independently of historical pagination. Task groups use actual status keys (custom statuses sharing a category stay distinct); project/assignee/label selections remain visible when their options disappear. Optional uncaptured priority/labels remain unknown rather than being enriched from live records.

Shared live/frozen events render localized kinds, actor identity/name, saved-zone business timestamps, reason and guarded source/destination references. Closed-period destination links resolve through the protected catalogue, with current-directory names explicitly distinguished from frozen task facts. Historical task titles open frozen/current comparison, show current project/assignee names and guarded priority/labels, preserve deleted/inaccessible history, and offer an explicit current-task link only after retrieval. Unknown planning modes remain readable without new I1 operations.

UUID assignment selections now use one complete preliminary move preview and one final preview, validating the exact selected set and deriving fresh source/revision values. Only readable aliases require individual resolution, with workspace verification. The mounted 1,000-task regression confirms exactly two preview calls and zero per-UUID issue reads. Terminal candidates are excluded by default; cancelled tasks remain excluded and completed work requires an explicit acknowledgement plus an active target. Existing whole-selection atomicity, pending identity and preview locks remain intact.

Verification at handoff: all iteration component suites **50 passed** before the last audit-label regression; final affected details/navigation suites **24 passed** after it, plus views typecheck, scoped iteration ESLint and `git diff --check` all exit 0. The final regression distinguishes lifecycle cancellation from task cancellation. No test/build process remains live. Root owns final fresh Web/Desktop E2E, visual verdict, full checks and gate promotion; this handoff does not itself assert UG/FCG/VG passed. No known unimplemented item remains in the bounded UI completion list above.
