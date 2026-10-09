# Current implementation and reuse research

Read-only source review on 2026-10-09. Only this research file was written; no source, tests, other task documents, or configuration were modified. Tests and browsers were not run.

Guidance read: root `CLAUDE.md`; views frontend component/quality guidelines; core `iteration-operations.md`; shared reuse/cross-layer guides; Web E2E environment guidance. Some generic spec entries remain placeholders, so the root rules and active iteration contract are authoritative.

## Changes already present since the audit

- `packages/views/iterations/iteration-progress.tsx:9` exports `IterationChartTable`. The previous inline table in `IterationHistory` is now consumed through this shared component (`iteration-history.tsx:110`). It retains the caption, row/column headers, complete values, and horizontal overflow handling. This removes the need for another table implementation.
- `iteration-overview.tsx:88` / `IterationTimelineRow` already puts that table behind a focusable native disclosure at line 113. It fetches detail only for the active manual iteration and prefers snapshot statistics. Definitive read errors remove its cached chart/table; transient errors retain them. Reuse this disclosure pattern for the detail view.
- `iteration-chart-closeout.test.tsx:28` now verifies every table cell and the actual `LineChart` data prop against the same frozen chart. Rerendering with conflicting current values (99/98/97) must not change either representation. This strengthens an existing frozen-data contract; it does not introduce a new statistics formula or simplify the progress layout.
- `iteration-page.tsx:114` / `IterationReadFailure` and line 126 / `IterationRefreshError` provide read-specific errors, return navigation, disabled retry while fetching, and no write replay. Initial/retry state uses `errorUpdatedAt` to avoid an endless loading-only screen. `iteration-error.tsx:5` exports the existing definitive-error classification, keeping 408/429 transient; access-denied workspace 404 is distinguished from ordinary missing data.
- `iteration-page.tsx:98` now always displays the saved iteration timezone, even when it equals the workspace timezone. Navigation regressions cover UTC, Shanghai, and a workspace timezone change after closure.

The original audit gaps are still present in the target detail tabs: the 3+8 metric blocks precede the chart (`iteration-history.tsx:61,75`); the chart repeats effective completion (`iteration-progress.tsx:76`); the history table remains expanded; event rows still use generic labels and unconditional source/target references; `IterationEvents` still replaces one cursor/page and hides cached rows on any error (`iteration-page.tsx:130`); original-scope stroke/legend opacity is still 0.6/0.5 (`iteration-progress.tsx:63,71`). The improved parent read handling has not fixed the event sublist.

## Contracts to retain during restructuring

- Both `iteration-page.tsx:78` and `iteration-history.tsx:20` prefer `snapshot.statistics`. Chart, table, summary, closeout facts, and event task identities must not be reconstructed from live tasks or filtered task lists.
- Preserve all chart dates and values, semantic table headings, and an accessible way to open the table. Adapt disclosure interactions in tests instead of deleting the frozen-value assertions.
- Keep saved timezone formatting through `common/format-in-time-zone.ts`; do not substitute browser/workspace timezone for a closed iteration's clock.
- Keep stable workspace/iteration keys, visited-tab mounting, task-filter retention, and edit/confirmation dialogs outside tab panels. Do not key these by revision. `IterationRecovery` remains outside capability/enabled/tab gates.
- Retain authoritative access revocation: do not retain protected rows after 401/403 or explicit workspace-access-denied 404. A retry for a transient read must not replay a write.
- The shared complete catalogue remains `iterationCatalogueOptions`; do not replace its query function with a weaker local traversal under the same key.

## Reusable patterns

| Need | Existing owner and function | How to use / constraint |
| --- | --- | --- |
| Compact activity summaries and progressive disclosure | `packages/views/issues/components/issue-detail.tsx:293` `formatActivity`; `:526` `ActivityBlock` | Status/title changes use localized `from`/`to`; older activity groups collapse and the latest block initially shows eight items. Both functions are private and issue-specific: use their presentation pattern, not an import of the large issue-detail page. Do not copy their raw payload casts into a new event contract. |
| Changed fields with before/after values | `packages/views/triage/triage-history.tsx:26` `TriageHistoryRecord`, especially `:177`; `triage-ui.ts:128` `triageSnapshotChanges` | Native disclosure, labelled fields, responsive before/after columns, and separate name-resolution notice. The comparator's allowlist and nested `issue` shape are triage-specific and coalesce null/missing; iteration semantics must preserve their own missing-versus-null distinction. |
| Current/custom status names | `packages/views/issues/utils/status-label.ts:22` `useStatusLabel(wsId)` | Built-ins use i18n, custom keys use the current catalogue, unknown keys remain readable. Only use current names as display enrichment. |
| Frozen status categories | Same file `:38` `useStatusCategoryLabel(unknownLabel)` | Specifically resolves saved categories without applying today's custom catalogue. Prefer this for historical category facts. |
| Actor directory names | `packages/core/workspace/hooks.ts:65` `buildActorNameResolver`; `:94` `useActorName` | The pure resolver builds maps from explicit member/agent/squad snapshots and handles plugin/system actors. The hook requires workspace context and loads all actor directories. Existing fallback strings are English; do not silently introduce those into the Chinese event UI or claim loading names are known missing. Preserve explicit `wsId` and conditional directory reads where appropriate. |
| Frozen iteration references | `packages/views/iterations/iteration-catalogue.tsx:12` `IterationReference` | Already distinguishes missing (`undefined`) from explicitly unassigned (`null`). Only render a migration reference for an event that actually has that relationship. |
| Folded chart data | `iteration-overview.tsx:113`, `IterationTimelineRow`; `iteration-progress.tsx:9`, `IterationChartTable` | Existing localized `iterations.pages.viewChartData`, keyboard-focus styles, complete shared table. No new dependency or bespoke accordion needed. |
| Cached data plus transient error | `iteration-page.tsx:75,90,126`; `iteration-overview.tsx:95,112`; `packages/views/projects/components/project-overview.tsx:50,68` `ProjectOverview` | Decide whether authoritative data may remain, then show a separate refresh warning/retry. Keep definitive-access handling from iterations rather than borrowing the project guard. |
| Distinguishing empty from unavailable | `packages/core/workspace/hooks.ts:42` `useWorkspaceList` | Uses `data !== undefined` rather than `isFetched`: a successful empty list is ready; an initial failed read is unavailable; background failure retains ready data. Reuse the state distinction, not this unrelated workspace-list hook itself. |

## Existing tests and changes they will need

| Entry | Coverage / recommended use |
| --- | --- |
| `iterations/iteration-chart-closeout.test.tsx` | Canonical chart/table frozen-value regression. Keep exact rows and chart-prop assertions through a disclosure/layout change. |
| `iterations/iteration-history.test.tsx` | Accessible chart/table, counters, saved-clock closure time, frozen statistics, end reason, processed time, rollover warning. If metrics/table move behind disclosures, open them before asserting; preserve the underlying data checks. |
| `iterations/iteration-details.test.tsx:63` | Task cancellation versus iteration cancellation, saved-clock events, named membership transitions, frozen title/identifier and no live `getIssue` fetch. Line 77 currently requires two “Not recorded in this snapshot” labels from a non-migration event: this assertion locks the audited noise and should change. Preserve genuine missing history separately from non-applicable fields. |
| `iterations/iteration-navigation.test.tsx:171` | Saved timezone, first-read retry without write replay, transient dirty-editor retention, definitive deletion, access loss, entity-local state reset. Lines 566+ cover overview chart disclosure/frozen data/no per-row detail fan-out; lines 630+ cover retained task filters and dialogs. Add event-page recovery/pagination tests here or in a focused sibling mounting the real page. |
| `locales/parity.test.ts` | Required for new English and Simplified Chinese labels. |
| `e2e/iterations-i1.spec.ts:45` | Existing real keyboard interaction for the overview chart disclosure, read retry, and iteration-scoped creation. Do not weaken it when sharing the disclosure. |
| `e2e/fixtures/iterations-i1.ts:123,145` | Shared Web/Electron closure flow currently expects the detail table immediately visible and an event string matching `Task: ...`. Update interaction/text locators if the detail table is folded or events become sentence-based; retain immutable-snapshot/no-live-title assertions. |
| `e2e/iterations-i1-desktop.spec.ts:15` | At 680px it scrolls the detail data table into view. Open the disclosure first if folded; retain overflow verification. |

### Mock conventions

- Views tests use real React Query providers, mock `@multica/core/api`, and preserve original exports when `ApiError` / access guards are needed. Existing fixtures expose `getBaseUrl` and `getSessionScope`. New event-page tests must add a `getIterationEvents` mock; the current navigation test API object does not expose it.
- `iteration-details.test.tsx:29` creates a fresh `QueryClient` with query retries disabled. Navigation uses the real iteration query options (which already disable retries) and changes cached responses through `act` plus `invalidateQueries`.
- Use `test-fixtures.ts` for workspace, iteration, task, statistics, and receipt identities. Keep DOM tests focused on interaction/wiring; pure projection/enum matrices belong in a colocated `.test.ts` with `// @vitest-environment node`.
- `useT` mocks use the selector callback against the actual locale JSON. Navigation has an existing `language` switch for English/Chinese. If new copy interpolates values, extend that mock to honor interpolation rather than asserting an unexpanded template.
- Stores use the callable Zustand shape plus `getState`. Mock `../navigation` / `@multica/core/paths`, never `next/*` or `react-router-dom`.
- Retain real Base UI tabs/disclosures for keyboard behavior. Recharts is intentionally mocked; the closeout suite spies on `LineChart` while retaining the real HTML table.
- `packages/views/test/setup.ts` supplies matchMedia, ResizeObserver, elementFromPoint, scrollIntoView, and memory-storage fallbacks. These do not provide real layout or contrast measurements.

## Focused validation commands for the implementation owner

From the repository root:

```bash
pnpm --filter @multica/views exec vitest run iterations/iteration-chart-closeout.test.tsx iterations/iteration-history.test.tsx iterations/iteration-details.test.tsx iterations/iteration-navigation.test.tsx locales/parity.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec eslint iterations/iteration-history.tsx iterations/iteration-progress.tsx iterations/iteration-page.tsx iterations/iteration-events-view.tsx
```

Include newly introduced helper/test files in lint and the focused run. If core event queries/projections change, also run the relevant core tests (`iterations/access.test.ts`, `iterations/catalogue.test.ts`, plus new projection tests) and core typecheck. Broaden to `pnpm --filter @multica/views test iterations` once the focused behavior passes.

For an already configured, task-owned production Web environment:

```bash
MULTICA_RUN_I1_E2E=1 make env-exec ARGS="-- pnpm exec playwright test e2e/iterations-i1.spec.ts e2e/iterations-i1-history.spec.ts --workers=1 --retries=0"
```

The I1 specs otherwise skip. Playwright starts no servers: follow `.trellis/spec/web/frontend/e2e-run-environment.md`, verify the environment's API/Web provenance, and use production Web rather than `next dev`. Electron additionally needs the existing `project-p1-desktop` fixture setup. No validation commands were executed for this research-only handoff.
