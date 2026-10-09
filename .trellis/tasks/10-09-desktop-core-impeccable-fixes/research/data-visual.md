# Research: Read failures, Gantt contrast and visible-window rendering

- Query: Confirm audit P1-02, P1-05 and P2-10 against the current worktree; identify reusable recovery, color and virtualization patterns and regression coverage.
- Scope: Internal repository and audit evidence; official reference links listed for verification follow-up.
- Date: 2026-10-09

## Findings

### Current state and scope

The audit records confirmed cold-cache HTTP 500 reproductions for Inbox and Projects, a reproducible token contrast calculation for Gantt, and static unbounded date-DOM growth. It explicitly does not claim a full Gantt business fixture, measured long-span jank, or an injected project-detail failure.

The current working copy already contains candidate product fixes for all three findings and additional recovery/virtualization tests. Treat them as existing shared work: review and verify them, rather than replace them or assume the audit describes the present source. This research changed no product code and did not run the test suite.

### Files found

| File | Purpose |
| --- | --- |
| `docs/audits/2026-10-09-desktop-core-impeccable.md` | Original issue groups, severity, scope and evidence limitations. |
| `.impeccable/audit/2026-10-09-desktop-core/failure-evidence.json` | Two failed 500 requests per cold-cache page followed by empty-state copy. |
| `.impeccable/audit/2026-10-09-desktop-core/gantt-token-contrast.json` | Electron Canvas sRGB values and original white-text contrast ratios. |
| `packages/core/query-client.ts` | Production QueryClient defaults: infinite stale time, reconnect refetch and one automatic retry. |
| `packages/core/inbox/queries.ts` | Workspace-scoped live/archive queries and deduplication; no view-local API call is needed. |
| `packages/core/projects/queries.ts` | List/detail query options with explicit workspace, AbortSignal and protected request wrapper. |
| `packages/core/projects/access.ts` | Definitive access/deletion classification and protected cache/draft cleanup. |
| `packages/views/layout/collection-page.tsx` | Existing centered state with icon, accessible alert/status role and action slot. |
| `packages/views/inbox/components/inbox-page.tsx` | Candidate live/archive cold error, cached refresh banner, Retry and selection preservation. |
| `packages/views/projects/components/projects-page.tsx` | Candidate cold list error and cached refresh banner while retaining the list subtree. |
| `packages/views/projects/components/project-detail.tsx` | Candidate 500 vs 404 distinction, permission guard and cached-detail recovery. |
| `packages/views/inbox/components/inbox-page-recovery.test.tsx` | Real QueryClient recovery, active/archive selection, retained reply node/value/focus and compact mode. |
| `packages/views/projects/components/projects-page.test.tsx` | Mocked-query cold/refresh branch tests plus row/filter/selection retention. |
| `packages/views/projects/components/project-detail.test.tsx` | Real QueryClient cold 500 recovery, 403/404 distinction and retained progress draft. |
| `packages/views/issues/components/gantt-view.tsx` | UTC geometry, candidate color pair map, bounded date window and virtual rows. |
| `packages/views/issues/components/gantt-view.test.tsx` | Real virtualizer with jsdom geometry substitutes; long span, row count, scroll, sort and keyboard regressions. |
| `packages/ui/styles/tokens.css` | Existing semantic fills plus candidate Gantt-specific light/dark foreground tokens. |
| `e2e/squads-audit.spec.ts` | Reusable browser Canvas/getComputedStyle contrast measurement and a 4.5 assertion. |
| `e2e/admin-resource-publishing.spec.ts` | Browser contrast measurement with recursive alpha/background composition. |

### P1-02: Distinguish failure from empty data and preserve recoverable work

The original root cause was using a default empty array while ignoring query failure. The production QueryClient retries once (`packages/core/query-client.ts:11`), which explains the audit waiting for two failures before evaluating the page. Infinite stale time means a refresh-recovery test should explicitly invalidate/refetch or trigger a reconnect, rather than expect every mount to refresh.

The appropriate existing pattern is:

1. Initial pending read: loading state.
2. `isError && data === undefined`: centered `CollectionPageState` with `role="alert"` and Retry.
3. Successful empty response: ordinary empty-state copy/action.
4. Failed background read with defined cached data: retain the list/detail/editor subtree and add an alert with Retry.
5. Definitive permission revocation/deletion: use existing domain guards and remove protected content rather than retain the recoverable 500 UI.

`data !== undefined`, not `data.length > 0`, distinguishes a known empty cached response from never having received a response. The Retry action calls the existing query's `refetch()` and is disabled during `isFetching`; it does not create an extra query, clear caches, navigate away, or reset filters.

Confirmed current paths:

- Inbox query classification: `packages/views/inbox/components/inbox-page.tsx:143`; cold alert/Retry: `:602`; refresh alert: `:650`.
- Inbox failure must not trigger the shared-link fallback: the effect now returns for error/undefined data at `inbox-page.tsx:255`. The existing archive-drain effect separately guards archive errors at `:285`.
- Inbox detail is keyed by issue identity at `:715`, allowing refresh failure/retry to retain the mounted editor. Compact detail includes the refresh alert at `:886`; this prevents a hidden list banner from being the only recovery action.
- Projects list guards `scopeDenied` before ordinary query recovery at `packages/views/projects/components/projects-page.tsx:925`. Cold failure is classified at `:927` and alerts appear at `:949`/`:958`.
- Project detail guards access loss/deletion before read recovery (`packages/views/projects/components/project-detail.tsx:270`). An actual API 404 renders not-found at `:286`; no-data service failure renders alert/Retry at `:290`; cached data receives a refresh banner at `:609`.
- `projectListOptions` and `projectDetailOptions` already protect requests (`packages/core/projects/queries.ts:12`, `:20`). `isProjectAccessLost` and `isProjectDeleted` distinguish domain codes (`packages/core/projects/access.ts:35`, `:38`); protected cleanup and stale-result rejection are at `:100`/`:108`. Do not bypass these while adding recovery.
- `IterationEventsPanel` already uses the same first-read/cached-refresh distinction (`packages/views/iterations/iteration-events.tsx:89`). `CollectionPageState` supports the required alert/action composition (`packages/views/layout/collection-page.tsx:155`, `:172`), so a new generic error component is unnecessary.

Existing regression coverage is meaningful:

- Inbox real QueryClient suite (`inbox-page-recovery.test.tsx:99`) retries both active and archive cold failures, asserts no empty copy or URL replacement, then verifies the selected reply appears. Cached cases assert the same row/editor DOM nodes, unsent value and focus survive error and Retry. Compact recovery and genuine empty success are separate cases.
- ProjectDetail real QueryClient suite (`project-detail.test.tsx:541`) distinguishes 500 from actual 403/404 and preserves a mounted progress editor/draft through refresh recovery.
- ProjectsPage (`projects-page.test.tsx:431`) currently mocks the query result; it proves branch wiring, filter/selection/node retention and refetch invocation, but not a real 500-to-success QueryClient transition. Browser fault injection should verify that transition; a small real-query test is optional if the browser harness already owns it.

### P1-05: Keep semantic status hues; choose a verified foreground pair

The audit measured white-text ratios of 2.245 for light warning, 2.701 for dark warning, 3.059 for dark success and 3.243 for dark info. These are ordinary `text-micro` titles, requiring 4.5:1, not the relaxed large-text threshold. Light success was already close to the threshold at 4.556:1.

Current candidate implementation uses status **category**, including custom statuses, rather than status key (`packages/views/issues/components/gantt-view.tsx:310`):

| Category | Fill | Foreground |
| --- | --- | --- |
| backlog / todo / cancelled | solid `bg-muted` | `text-foreground` |
| in_progress | `bg-warning` | `text-gantt-warning-foreground` |
| in_review | `bg-success` | `text-gantt-bar-foreground` |
| done | `bg-info` | `text-gantt-bar-foreground` |
| blocked | `bg-destructive` | `text-gantt-bar-foreground` |

The title inherits the link's foreground rather than forcing white (`gantt-view.tsx:449`). Gantt-specific tokens are exposed through `@theme inline` at `packages/ui/styles/tokens.css:37`; light foreground definitions are at `:225`, dark at `:308`. Light success/info/destructive use full white; both warning themes and dark success/info/destructive use dark ink. Neutral categories use the existing theme foreground. Keep global status fills untouched because many other surfaces use them.

A read-only Python calculation converted the current OKLCH values to clamped 8-bit sRGB and computed WCAG relative luminance. It is a useful preflight, not an Electron-rendered acceptance result (browser gamut mapping/rounding can differ):

| Current pair | Light estimate | Dark estimate |
| --- | ---: | ---: |
| muted / foreground | 18.100 | 14.270 |
| warning / Gantt warning foreground | 7.891 | 6.560 |
| success / Gantt bar foreground | 4.556 | 5.791 |
| info / Gantt bar foreground | 4.808 | 5.463 |
| destructive / Gantt bar foreground | 4.770 | 6.134 |

Every estimate passes 4.5, but light success has little margin. For acceptance, sample **rendered** title foreground and bar background with Electron/Chromium Canvas in both themes, across all categories including a custom category. The existing Canvas recipe is at `e2e/squads-audit.spec.ts:145`; recursive alpha composition is at `e2e/admin-resource-publishing.spec.ts:194`. Current neutral fills are solid, simplifying composition. If opacity/transparent fills return later, compute the effective background rather than contrast against an uncomposited token.

The current Gantt unit suite does not assert rendered color contrast, and jsdom cannot prove it. Keep the authoritative color check in a browser fixture/report rather than a class-string unit test that merely repeats the map.

### P2-10: Bound mounted DOM while preserving the full calendar and keyboard access

The audit's root cause was two full-span per-day arrays rendered into axis/background plus a complete row render. Changing month zoom alone did not stop per-day DOM growth.

The current candidate has the appropriate existing-dependency solution:

- Date window: arithmetic `first`/`last` offsets based on horizontal scroll and usable width, with 120px overscan (`packages/views/issues/components/gantt-view.tsx:545`). The array is allocated only for visible days; no full-span day measurement array is created.
- Axis and background render that shared window (`gantt-view.tsx:122`, `:182`, `:267`); month headers iterate the visible month range (`:134`). The full canvas width and absolute UTC-date/bar geometry remain intact, so offscreen dates are still reachable.
- ResizeObserver and passive scroll listener update the window and clean up on unmount (`:518`). Initial/zoom positioning centers UTC today in the timeline area beyond the 320px sticky label column.
- Vertical rows use existing `@tanstack/react-virtual`, with 36px row height, 56px header scroll margin and overscan 5 (`:568`). The dependency already exists in `packages/views/package.json`; no install is needed.
- Stable item keys use issue id (`:558`). Focus/context-menu targets are pinned via a custom range extractor (`:559`) so an ordinary scroll does not unmount the active target.
- Rows expose list position/total metadata and links; keyboard handling supports ArrowUp/Down, Home/End and Tab/Shift+Tab across window boundaries (`:596`). The outer timeline is a named keyboard-scroll region (`:674`).

Mounted cost is governed by viewport size instead of total calendar span or row count. At a 900px-wide test viewport, month zoom has about 138 visible/overscan days per layer; vertical mounted rows are roughly viewport rows plus two overscan sides and one pinned row. Whole-list sorting/range scanning still depend on issue count, which is expected and memoized; this change addresses DOM growth rather than redefines server filtering or date semantics.

Existing `gantt-view.test.tsx:149` uses the real virtualizer with only missing jsdom layout/scroll APIs supplied. Its named regressions cover:

- 2020-01-01 through 2030-12-31, all three zoom modes, first/last dates reachable and under 750 total DOM elements (`:189`).
- UTC-today geometry and zoom widths (`:212`).
- 1,000 rows, far-end scrolling and reverse sorting, under 2,000 DOM elements (`:227`).
- Focus pinning, Home/End beyond the mounted window (`:252`).
- Tab/Shift+Tab continuity at the last mounted row (`:269`).
- A previously pending Home request cannot steal focus from a later toolbar click (`:286`).

Browser acceptance should use the real Electron scroll container to check horizontal/vertical scroll geometry, all zooms, UTC/locale labels, last issue/date reachability, context menu behavior, and focus visibility/continuity. A DOM-count bound alone is not a measured frame-time/jank claim.

### Suggested verification handoff

Run the existing focused suites before deciding whether any extra product edit is needed:

```sh
pnpm -C packages/views exec vitest run inbox/components/inbox-page-recovery.test.tsx inbox/components/inbox-page.test.tsx projects/components/projects-page.test.tsx projects/components/project-detail.test.tsx issues/components/gantt-view.test.tsx
pnpm -C packages/core exec vitest run projects/access-lifecycle.test.ts
pnpm typecheck
```

Then use the audit's isolated Desktop fixture or an equivalent existing harness:

- Cold-cache 500 for live/archive inbox, projects list and project detail; assert localized error/Retry and absence of false empty/not-found copy.
- Retry success with the same URL; cached-refresh 500 keeps row position, filters, selection and typed reply/progress draft.
- Delayed retries to verify button disabling or the ordinary pending state while in flight.
- Genuine project 404/403 remains terminal and protected content disappears under the existing access guard.
- Render all Gantt categories in light/dark and persist actual Canvas contrast ratios (minimum 4.5).
- Render the ten-year span and 1,000 rows; scroll to both ends at each zoom, measure DOM bounds and keyboard/context-menu continuity. If reporting performance, record actual browser timing separately from DOM bounds.

### Related specs

- `CLAUDE.md`: authoritative ownership/boundaries, semantic tokens, one canonical regression layer, query/mutation server access and honest verification claims.
- `.trellis/spec/views/frontend/index.md` and `component-guidelines.md`: shared views, accessible navigation and reuse of established patterns.
- `.trellis/spec/views/frontend/quality-guidelines.md`: callable-store mocks for singleton core stores; several other views guideline sections are still placeholders, so defer to CLAUDE.md and existing source.
- `.trellis/spec/core/frontend/index.md` and `.trellis/spec/core/frontend/iteration-operations.md:145`: first-read failure vs cached temporary error, disabled-while-fetching Retry, preserved drafts and eviction on definitive access loss.
- `.trellis/spec/ui/frontend/index.md` / `component-guidelines.md`: shared primitive boundaries and real geometry/browser verification for layout behavior; most generic sections are placeholders.
- `.trellis/spec/guides/index.md`: cross-layer/reuse checks and corroboration of audit claims.

### External references and versions

Declared catalog ranges (not a claim that each exact version was resolved): TanStack Query `^5.96.2`, TanStack Virtual `^3.13.0`, Vitest `^4.1.0` (`pnpm-workspace.yaml`). Audit runtime: Electron 39.8.7.

- TanStack Query v5 useQuery: https://tanstack.com/query/latest/docs/framework/react/reference/useQuery
- TanStack Virtual Virtualizer options (`rangeExtractor`, `scrollMargin`, `scrollPaddingStart`, `getItemKey`): https://tanstack.com/virtual/latest/docs/api/virtualizer
- WCAG 2.2 Contrast (Minimum), 4.5:1 ordinary text: https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html

The internal implementation/test patterns were inspected. These official links are follow-up references; external pages were not fetched in this research slice.

## Caveats / Not Found

- Source is a shared, substantially modified working copy. Line numbers describe the inspected snapshot and may move as another agent edits. Preserve existing task work and unrelated edits.
- This is research, not a verification sign-off: product suites and Electron reproductions were not run here.
- The original audit's HTTP-500 examples are valid historical evidence; current source already has candidate fixes, so do not carry forward a claim that it still shows empty data without fresh reproduction.
- ProjectsPage's current recovery test uses mocked query state; the real network/cache recovery transition still needs browser evidence or a dedicated real-query test.
- No current Gantt unit test establishes rendered all-state contrast. The local conversion supports the candidate token choice; a browser-rendered acceptance matrix remains necessary.
- Inbox APIs intentionally use schema fallback to empty arrays for malformed successful payloads (`packages/core/api/client.ts:3247`, `:3269`). HTTP failures reject before that parsing. Changing compatibility fallback semantics is outside this audit's confirmed 500 fix.
- Very long physical CSS scroll extents beyond the tested ten-year fixture are not validated; windowed DOM does not by itself prove every date range accepted by an API remains representable by the browser's layout engine. Do not claim such coverage without a fixture.
