# Research: Iteration scope UI integration and verification

- Query: Find the smallest shared UI changes for phase-aware scope summaries, business event explanations, metric drilldowns, category/task search, current-task navigation, and technical audit disclosure; identify reusable verification infrastructure.
- Scope: internal; shared Web/Desktop views, navigation, locale contracts, existing tests, and production browser evidence. No product changes, server starts, or git operations performed.
- Date: 2026-10-10 (the task was created on 2026-10-09; the supplied session date advanced during research).
- Requirements: ISB-01 through ISB-07 and AC-01 through AC-08 in `../prd.md`.

## Findings

### Existing behavior to preserve

| File / evidence | Existing responsibility and implication |
| --- | --- |
| `packages/views/iterations/iteration-page.tsx:59` | Detail owns the resource, tab selection, visited panels, editors and lifecycle operations. The page already selects `snapshot.statistics` before live statistics at line 81. |
| `packages/views/iterations/iteration-page.tsx:119` | Tasks, Progress and Events remain mounted after visiting. Search/page state survives ordinary tab changes; workspace/iteration identity keys reset it, rather than resource revision. |
| `packages/views/iterations/iteration-page.tsx:126` | Events receives only workspace/id/timezone/statistics/snapshot. It lacks `iteration.started_at` and lifecycle status: add the validated iteration metadata here rather than guessing phase from a zero count. |
| `packages/views/iterations/iteration-events.tsx:24` | Identity-keyed panel already owns scope/all selection and locally revealing ten operation groups at a time. |
| `packages/views/iterations/iteration-events.tsx:34` | Complete activity query is disabled for snapshots. Snapshot identity is checked; permission and deletion guards run before rendering counters. |
| `packages/views/iterations/iteration-events.tsx:55` | Unconditional initial/current/net counters are the immediate presentation defect. Existing first-read/loading/refresh/retry states at lines 90–118 should remain intact. |
| `packages/views/iterations/iteration-events-view.tsx:113` | Frozen title/reference, movement source/destination, before/after field changes, and recorded reason already exist. Add impact and compact composition; do not introduce a second event decoder. |
| `packages/views/iterations/iteration-events-view.tsx:148` | Technical event details are **already collapsed by default**; raw actor/before/after facts have a second closed native disclosure. Rename the first disclosure to clearly mean technical audit, retaining all fields. The screenshot shows an opened disclosure, not an always-expanded implementation. |
| `packages/views/iterations/iteration-events-view.tsx:190` | Timeline preserves saved-timezone day sections, operation grouping, per-entry audit and reveal-all records. The embedded `IterationEventsView` at line 227 also serves history/participation consumers. |
| `packages/views/iterations/iteration-issue-list.tsx:50` | Tasks already has server-backed search, current/original scope, filters, grouping, previous/next pagination, and clear/no-match states. Do not present these as new features. |
| `packages/views/iterations/iteration-issue-list.tsx:164` | Live rows use `AppLink`; historical rows use `IterationCurrentComparison`. Long titles wrap and repeated rollover advice already exists. |
| `packages/views/iterations/iteration-current-comparison.tsx:20` | Historical/current comparison fetches the current issue only after explicit expansion, through `protectIterationRead`. Deleted/unavailable copy preserves historical facts; the current-task link only appears with successful data (lines 55–69). |
| `packages/views/iterations/iteration-history.tsx:22` | Progress is already snapshot-first, exposes both completion denominators, unknown closeout facts, complete destinations and chart tables. Avoid rewriting it to implement the Events tab. |
| `packages/views/iterations/iteration-catalogue.tsx:12` | `IterationReference` already distinguishes unknown ID, explicit no iteration, and a named/deleted reference through platform navigation. |
| `packages/views/iterations/iteration-presentation.ts:1` | Shared native-disclosure focus/hover/44px coarse-pointer treatment. Reuse instead of adding another disclosure style. |

### Minimal integration design

1. **Phase comes from validated resource facts.** Pass `iteration` (or a typed pick containing identity, status and `started_at`) from the existing detail query into the panel. The schema already has `started_at`, status and nullable snapshot (`packages/core/api/iteration-schemas.ts:34`, `:110`). No response schema extension or new endpoint is necessary merely to render the phase.
   - Planned: show the current plan membership count and “commitment is determined when this iteration starts”; hide started-window metrics rather than rendering misleading zero/+N.
   - Started: show actual initial effective scope and current effective scope; zero initial commitment remains a genuine zero, not “not started”.
   - Snapshot: prefer frozen final counts, but distinguish a plan cancelled before starting from a started period. Snapshot presence alone does not prove it ever started.
   - Unknown/inconsistent lifecycle information must remain readable with neutral/unknown wording, not silently become an active period.
   - The task-panel original-count wording and Progress's planned-only condition at `iteration-page.tsx:122–125` are adjacent phase-sensitive surfaces. Route them through the same phase result if required for AC-02; do not leave a cancelled-before-start plan claiming a start baseline in one tab.
   - Empty plans currently hide the tab strip and force Tasks (`iteration-page.tsx:86–87`, `:120`). Decide explicitly whether the task requires accessing events after all planned tasks have been removed. Preserving historical plan adjustments argues for keeping Events discoverable even at zero membership; do not accidentally hide recorded adjustments behind the existing empty-plan layout.

2. **Keep decoding/selection in core.** `projectIterationEvent` already normalizes frozen fields and null/unknown distinctions; `groupIterationActivity` preserves complete operations (`packages/core/iterations/activity.ts:203`, `:245`). UI should consume a core result for stage/window, category, impact and metric membership rather than re-reading raw `before_facts`/`after_facts` in JSX.
   - `planned_activity` may project to `join`/`leave` (`activity.ts:175`); the presentation must also know its original type/window before saying “added after start” or “effective scope +1”.
   - Start operations include baseline entries. End/handoff operations may include task movements. Search/category selection must not mislabel a filtered child as a different whole operation or lose the lifecycle anchor.
   - Distinguish “N matching records/tasks” from “N operations shown”. A group count is neither an event count nor a unique-task count.

3. **Compact business rows, retain auditable facts.** Put the action and frozen task identity together, with actor/time as secondary text; keep reason and source/destination visible where applicable. Only display a signed scope impact that the typed projection can prove. A normal done-to-todo reopen need not change effective scope, a cancelled-to-active restore can, and a cancelled task joining does not automatically add one effective task. Do not derive risk, urgency or blocked state from titles.
   - Keep `EventDetails` as the technical layer, default closed. Do not auto-open it when applying a metric or search.
   - Consider a business detail disclosure only for genuinely long before/after content; raw UUID/type/sample time should not acquire primary visual weight.
   - Preserve `IterationEventsView`'s default all-events behavior for embedded consumers; page-only filters belong in `IterationEventsPanel`.

### Metric selection: inline records, not destructive task-tab navigation

**Recommendation:** keep drilldown state inside Events. Use typed event selection for activity metrics and an independent inline task-detail view for membership metrics. Do not programmatically rewrite the already-mounted main Tasks list's private search/scope/filter/cursor state just to follow a metric.

| Metric type | Correct inline detail | Important limit |
| --- | --- | --- |
| Planned/current/final membership | Complete matching task identities from the validated current projection or snapshot scope. | Current membership is not the same set as current effective scope; cancelled tasks must follow the selected metric's exact definition. |
| Initial effective scope | A proven start-membership projection, including its start-time categories, or authoritative original snapshot data with the correct predicate. | Do not substitute today's original-task status for its initial effective membership. |
| Added unique tasks | One row per distinct qualifying issue, with its matching event(s) available beneath it. | A task added, removed and re-entered is still one task under the unique-add definition; it may no longer be a current member. |
| Removal/cancellation/re-entry/reopen event count | Exactly the core-selected counted event entries, retaining access to the full operation context. | Repeated events for the same task remain multiple events; closure movements and pre-start adjustments must not leak into a started-window event metric. |
| Net change / percentage | An explanation and the relevant contributing records only when provenance is proven. | The net number is not a set cardinality; never label a list as “N tasks” by taking absolute net change. |
| Incomplete historical evidence | Show the authoritative count plus an honest explanation that a complete matching list cannot be reconstructed. Related known records can still be inspected as such. | Never make a partial list look complete, infer a missing field as zero, or replace the authoritative count with list length. |

`IterationIssueList` is not currently a controlled reusable metric list: its scope/search/filter state is private and it can fetch grouped or paginated data. It is suitable for the existing Tasks tab and an explicitly configured independent membership browser, but **do not mount it with its default “current” scope under an arbitrary metric label**. For this task, prefer a small read-only inline renderer for the core-provided matching identities; reuse the existing row/link/disclosure treatment. A separate whole-list instance is only warranted if the final core contract deliberately supplies authoritative API parameters for that exact metric.

State/interaction contract:

- Store selected metric, category and task search locally in Events. Clicking a metric clears incompatible local category/search so its initial results explain that number; later refinement displays a separate matching count. It never changes the top authoritative statistics.
- Keep the current Tasks list mounted and untouched when opening/clearing a drilldown or switching tabs. No URL, new route, new global store or extra persisted preference is needed.
- Use a real button for a metric action, with a readable name and selected state. Associate it with the inline detail region through `aria-controls`; announce the result count. Native Enter/Space and a visible focus outline must work.
- Reuse `Input` + `Label`, `Select` and `Button` for search/category. `TaskSelect` at `iteration-issue-list.tsx:28` is private; a one-off event category control does not justify moving all task filters into a new generic framework.
- Search frozen task title/identifier and stable issue identity; never fetch live titles to make historical search work. Normalization and matching belong in the pure selector.
- Reset local reveal count on a changed metric/category/search. Preserve it and open disclosures across an authorized temporary refresh. Reset on workspace/iteration identity, not revision changes.
- Distinguish no activity, no scope activity, and no results for active search/category/metric. Keep All activity and Clear filters usable.

### Current task links and access boundaries

- `IterationActivityEntry.issue` contains only `{id, title?, identifier?}` (`activity.ts:98`). It is not a complete `HistoricalIterationIssue`. Do not cast it into `IterationCurrentComparison` or invent project/assignee/status defaults to satisfy that component.
- A reliable event `issue.id` can use `useWorkspacePaths().issueDetail(id)` with `AppLink`, explicitly labelled “Open current task”. Keep the visible historical title and event facts unchanged. Missing `issue_id` must not turn into a guessed link from the title/identifier.
- Where a real snapshot/original/scope row is available, reuse `IterationCurrentComparison` for historical/current comparison. It already handles lazy access checks and deleted-task presentation.
- `packages/views/navigation/app-link.tsx:32` handles the desktop shareable URL, normal navigation, modifier/middle clicks and focus prefetch. Do not use `next/link`, `react-router-dom`, `window.location`, or a custom link handler in shared views. No platform route work is needed.
- Existing panel guards (`iteration-events.tsx:35–44`) include `hasIterationReadAccess`, explicit access denial and iteration 404. Keep new inline counters/details beneath these guards. Any newly introduced read must use the established protected Query boundary; a raw `api.getIssue` request in render/effect would bypass its auth epoch.
- Existing member/agent/squad catalogue lookups fill missing display names only (`iteration-events-view.tsx:25–52`). They must not enrich frozen task facts. Current-reference-name explanations already exist.

### Proposed ownership boundaries

These are implementation recommendations, not edits made by this research agent.

| Owner | Exact write boundary | Dependency / review note |
| --- | --- | --- |
| Core/domain implementation | `packages/core/iterations/activity.ts`, `activity.test.ts`, `index.ts`; if useful, new `scope.ts` / `scope.test.ts` beside them. | Own the single typed stage/impact/filter/metric selection contract. Preserve the complete protected activity reader. UI waits for this contract. API files change only if separate domain research establishes an actual missing field. |
| Shared UI implementation | `packages/views/iterations/iteration-page.tsx`, `iteration-events.tsx`, `iteration-events-view.tsx`; an optional tightly scoped inline detail component in this directory; `iteration-events.test.tsx`, `iteration-navigation.test.tsx`, and affected `iteration-details.test.tsx` assertions. | Own phase props, interaction state, inline rows and audit labels. Keep `iteration-history.tsx`, `iteration-issue-list.tsx` and `iteration-current-comparison.tsx` unchanged unless a concrete reuse or phase-consistency change requires touching them. |
| Same UI owner, to avoid translation conflicts | `packages/views/locales/en/projects.json`, `packages/views/locales/zh-Hans/projects.json`. | Change only iteration keys. Update existing label assertions with the implementation, not unrelated namespace copy. |
| Browser verification owner | New `e2e/iteration-scope-business.spec.ts`, optionally new `e2e/fixtures/iteration-scope-business.ts` and a thin `e2e/iteration-scope-business-desktop.spec.ts`. | Reuse existing auth/API/capture/native fixtures. Do not change global Playwright config, production route registration or dependencies. Keep browser writes inside synthetic workspace data. |
| Main session | Task design/implementation artifacts, final evidence, any justified spec update and final integrated checks. | This researcher owns only this document. |

### Canonical tests and narrow checks

Existing reusable coverage:

- `iteration-events.test.tsx:82`: full cross-page operation, scope/all, keyboard disclosure, and no eager current-issue fetch; `:110`: all authoritative task/event counts and snapshot precedence; `:129`: local reveal + retained open disclosure on 429; `:157`: 403/404 removal; `:169`: first load/error/empty; `:193`: identity reset; `:208`: Chinese frozen data and safe actor fallback.
- `iteration-navigation.test.tsx`: page metadata/permissions, editor and query state across tabs, frozen progress and history, no live history request for snapshots. Use this for the new page-to-panel phase wiring and preservation of an existing Tasks filter.
- `iteration-details.test.tsx:194–229`: event identity, unknown references and no live-title enrichment; `:249`: server task filters do not affect summary; `:264–289`: real historical/current comparison and deleted-task state.
- `packages/core/iterations/activity.test.ts` is the existing canonical home for grouping, decoding, complete traversal and auth matrices. Put any new pure stage/impact/selection matrix in core, not duplicated via dozens of DOM mounts.
- `packages/views/locales/parity.test.ts` enforces bilingual key/placeholder parity.

Suggested commands for implement/check agents (not run during research):

```sh
pnpm --filter @multica/core exec vitest run iterations
pnpm --filter @multica/views exec vitest run iterations locales/parity.test.ts
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
pnpm typecheck
```

Add package/app checks only if their files change; use the repository's final verification policy rather than inventing a new test runner. Existing package scripts are `vitest run`, `tsc --noEmit` and `eslint .` (`packages/core/package.json:7`, `packages/views/package.json:7`). Views defaults to jsdom; pure helpers should use the node environment directive required by `CLAUDE.md`.

New behavior assertions should cover planned two-task labels; started empty baseline; completed, cancelled-after-start and cancelled-before-start; known and unknown scope effects; repeated issue events; metric/category/search intersections; clearing; unchanged top counters; retained Tasks filter/cursor; no eager per-event current-issue fetching; correct link identity; closed audit; loading/429/retry; access eviction; no live activity for snapshot; and narrow long-text layout. Do not weaken the incumbent chronology, complete-operation or historical-value assertions just because markup changes.

### Deterministic production browser recipe

**Environment evidence:** the main session reported the registered API `localhost:18574`, Web `localhost:13494` and desktop renderer `5668` were stopped and the old agent TTL had expired. This is a parent-supplied observation, not a running environment established by this researcher. Do not assume those ports are valid for the new run.

`playwright.config.ts` starts no servers, uses one Chromium worker, zero retries, and chooses `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`. `.trellis/spec/web/frontend/e2e-run-environment.md:10–58` requires a production Web build with matched API/Web configuration and checkout ownership. One build per checkout: different ports do not isolate `.next`.

After the main session prepares the task-owned local environment, the documented launcher recipe is:

```sh
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- env MULTICA_RUN_I1_E2E=1 pnpm exec playwright test e2e/iteration-scope-business.spec.ts --project=chromium --workers=1 --retries=0 --output=.trellis/tasks/10-09-iteration-scope-business/browser-evidence/playwright"
```

This is a recommended execution recipe, not an authorization for this read-only agent to launch services. Verify task-only legacy auth and configuration before using the classic fixture (`MULTICA_AUTH_MODE=legacy`, device auth disabled; managed installation/admin flags cannot remain enabled in that mode). `make check` prepares such a task copy for its full pipeline. Preserve the user's source env and manual workspace. Save launch/provenance records; a stopped old build or a Next dev server does not validate the feature.

Reusable test setup:

- `e2e/fixtures/project-p1.ts:21` `p1Session(page, locale)` creates a random `p1-*` synthetic user/workspace, sets language and onboarding, and authenticates the browser. `p1Capture` at line 73 writes through `TestInfo.outputPath`; `p1Failure` captures page errors/state; `p1NoOverflow` checks document overflow.
- `e2e/iterations-i1-history.spec.ts:11–29` demonstrates API setup: read the saved timezone, enable iterations, create a period, create issues, request preview, assert no invalid items, then apply its exact draft/hash with a new request ID. Fetch current revisions for subsequent operations. Its `finally` at line 79 deletes only the synthetic workspace.
- `e2e/fixtures/iterations-i1.ts:8` provides dates based on the saved timezone. Its shared closure flow exercises real Web/Electron lifecycle, WebSocket updates and frozen assertions, but should not be copied wholesale into a focused test.

Recommended focused scenario, run in English and Chinese:

1. Create a synthetic plan with two distinct tasks, including one long title, via TestApiClient and real preview/apply. Navigate directly to its existing iteration route. Verify planned count 2, no initial-zero/net-+2/after-start-added-0 presentation, and visible plan adjustment history. Capture the stage.
2. Set a Tasks search/scope filter, switch to Events and keep that Tasks panel mounted. Start the period through the real API setup helper and await the real rendered update or reload as explicitly chosen by the test; do not claim realtime coverage if the test reloads.
3. After start, add a new task; cancel then restore one task; remove and re-enter the same added task. Capture actual returned statistics and event IDs. Verify unique-task vs event counts, category + title/identifier search, exact metric details, empty/clear behavior, and unchanged authoritative counters. Return to Tasks and prove its earlier filter is preserved.
4. Assert a metric button, category selector, search input and technical disclosure work by keyboard. Audit UUIDs/raw type remain hidden until explicit expansion. Assert no horizontal overflow in the **active panel and inline record container**, not only the document.
5. Activate a current-task link and verify its existing workspace issue route; return to the iteration. For a frozen period, mutate the live task's title/status after closure, then prove frozen event/title/statistics remain unchanged. If current comparison is included, assert the new live title appears only in the explicit current section. A deleted live task must not overwrite frozen values.
6. Use a targeted route fault only for refresh/retry and permission error branches when needed; ordinary stage/filter/navigation behavior uses real API data. Document mocked branches separately. Never mock away the business values under test.
7. Capture wide and 680px/coarse layouts, the zero-match state, selected metric state and an opened audit. Assert `matchMedia('(pointer: coarse)')` really changed before claiming touch-target coverage. In `finally`, delete the synthetic feature workspace.

For minimal native parity, reuse `e2e/fixtures/project-p1-desktop.ts` in a thin wrapper of the same scenario: it has a disposable profile, HTTP **and WebSocket** proxy, no daemon startup, and fixture cleanup. It accepts `MULTICA_E2E_DESKTOP_OUTPUT_DIR` for a separate production build (`:13`). Desktop entry can follow the existing sidebar to the seeded period rather than `page.goto` to an unsupported SPA URL. Run this only when the native build is prepared; a Web pass is not a native pass.

### Evidence files

Existing baseline/reference files confirmed present (read-only):

- `.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/scope-light-narrow.png`
- `.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/history-comparison-light-narrow.png`
- `.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/keyboard-panel-focus.png`
- `.trellis/tasks/archive/2026-10/10-09-iterations-audit-remediation/browser-evidence/closeout/rendered-metrics.json`
- `docs/audits/2026-10-09-iterations-impeccable-remediation.md:66–86` explicitly records production Electron verification and **no independent production Next.js E2E**. These captures establish incumbent layout only; they cannot prove this new implementation.

Proposed new task evidence, to be generated by the implementation/check/main owners:

- `browser-evidence/api.running.json`, `web.running.json`, and a run manifest recording actual environment/source/build identity and commands, copied from the chosen environment ledger without credentials.
- `browser-evidence/playwright/` for `TestInfo` artifacts: `planned-{locale}-wide.png`, `scope-metric-{locale}-narrow.png`, `scope-empty-{locale}.png`, `technical-audit-{locale}.png`, `frozen-current-separation-{locale}.png`.
- `browser-evidence/rendered-metrics.json`: selected source/counter values, matched unique tasks versus events, actual viewport/coarse flag, active-panel overflow and relevant control bounds, console errors, and fixture IDs.
- `browser-evidence/visual-verdict.json` and the applicable task-scoped visual verdict state required by the main session's AGENTS contract. Visual review must inspect the new captures, not merely check that PNG files exist.
- A test result report with exact executed test names, pass/fail/skip counts, and which error branches used mocks. Do not hardcode this task directory inside permanent E2E tests; they should use `TestInfo` paths and the runner controls the output directory.

### Related specs and external references

- `CLAUDE.md`: server state in Query, platform-adapter routing, bilingual conventions, semantic typography/colors, canonical test layer ownership.
- `.trellis/workflow.md`: research persistence and role-specific boundaries; the main session owns planning/execution transitions.
- `.trellis/spec/core/frontend/iteration-operations.md`: complete activity, snapshot precedence, current/frozen distinction, retry/auth eviction, kept-mounted tabs, no all-pages fetch merely to populate task filter choices, accessible disclosures.
- `.trellis/spec/views/frontend/quality-guidelines.md`: correct callable-store test mocks; most generic quality sections remain placeholders, so use CLAUDE and the active iteration spec.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production mode, environment ownership, build identity and retained evidence.
- `apps/docs/content/docs/developers/conventions.mdx` and `.zh.mdx`: `issue` is 任务, count units differ from execution/records, shared text lives in shared locale namespaces, direct Chinese copy. Reuse `iterations.activityPanel`, `iterations.pages` and existing current/historical labels instead of a second namespace.
- Versions observed in package manifests/catalog: pnpm `10.28.2`, React `19.2.3`, TanStack Query `^5.96.2`, Base UI `^1.3.0`, Vitest `^4.1.0`, Playwright `^1.58.2`. These are declared versions, not an installation audit.
- No external documentation fetch was necessary: this change can follow already-used local Query, Base UI, AppLink and Playwright APIs. No new library or route is proposed.

## Caveats / Not Found

- The exact provable membership/metric selectors and effective-impact rules remain the core/domain research owner's contract. This UI research must not be treated as proof that every historical counter can be reconstructed from optional event facts.
- Empty planned periods currently hide Events; this matters if a plan with past adjustments becomes empty. Address deliberately in design rather than preserving it accidentally.
- The event row's minimal identity is not a full historical task record. Any reuse of the full current-comparison component must supply real recorded fields or remain an explicit current-task link.
- No product code, locale file, spec, metadata or other task directory was changed. No tests, browser session, server, database operation or git command was run. All verification commands and new evidence paths above are proposed, not passed results.
