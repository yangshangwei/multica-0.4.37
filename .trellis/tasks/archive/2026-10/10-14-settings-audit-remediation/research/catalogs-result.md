# Catalogs and triage implementation result

Lane A completed its assigned implementation and package verification on 2026-10-14. The coordinator owns combined native acceptance and final task completion.

## Change boundary and findings

The defects lived in settings query-state branching, repeated status-catalog observers, viewport-based catalog grids, property-dialog sizing, and accessible control metadata. Changes stay in the assigned settings/triage views, their tests, and the subsequently assigned core query-access boundary and five query option functions. Shared Dialog/Tabs primitives, global retry policies, navigation, dependencies, backend code, and other lanes' files are unchanged by this lane.

| Finding | Implemented result | Verification |
| --- | --- | --- |
| F01 | The status parent owns recovery and supplies localized category labels. Category sections and editor dialogs no longer subscribe to the catalog just to format a category. | Real QueryClient and production catalog query; bounded failing-before test observed 11 requests instead of 1 in 200 ms. Passing-after test settles after one request with `retry:false`, isolating remount retries from normal configured retry policy. |
| F02, catalog portion | Labels, properties, Quick Actions and statuses distinguish pending, failed, successful-empty and cached-refresh states. Retry stays mounted and disabled while fetching. Property/status catalogs wait for permission data; unavailable permissions do not claim confirmed member-only access. | Real-query recovery tests for all three small catalogs; status tests and property permission failure test. |
| F03 | Catalog grids and their toolbars respond to available content width. Labels and Quick Actions use the `@3xl/catalog` grid tier; properties use `@4xl/catalog`. | Native geometry/pointer acceptance is coordinator-owned. No CSS-class assertions were added. |
| F04 | The property dialog has a bounded height, scrollable body, stable header/footer, and a separate full-width type row on narrow screens. | A real dialog test creates and submits all 12 options without dropping fields. Native visibility/scroll geometry is coordinator-owned. |
| F05, property/status portion | Property Type and status Category use stable visible-label associations. Catalog search controls also reuse their existing localized purpose as an accessible name. | Real Base UI role/name assertions. |
| F06 | Every shortcut recorder describes the current formatted binding or unassigned state, updated after recording, disabling and resetting. | Behavior test checks all four transitions on the real shortcut store. |
| F07, Quick Actions portion | Warning text uses `warning-foreground`. Archived Quick Action rows retain the archive badge without reducing the entire row's text opacity. | Static detector returned no findings. Actual light/dark contrast measurement is coordinator-owned. |
| F08, triage portion | Transient settings refresh errors retain the same focused switch and unsaved draft. A definitive denied read hides controls and clears the draft. | Real QueryClient/triage query tests verify node identity, focus, retry state and the saved draft values. |

## Shared interfaces

`packages/views/settings/components/settings-query-feedback.tsx` exports:

- `getSettingsQueryState(query)`: returns `{ hasData, failed, accessDenied }` from an existing query result.
- `SettingsQueryFeedback({ query, loadingMessage, errorMessage, refreshMessage })`: renders initial loading, specific failure/refresh copy and localized Retry/Retrying. Render it as a stable sibling before retained content; gate that content with `hasData`.
- The helper accepts successful `null` configuration as loaded data, uses query timestamps to retain cold Retry, and calls `refetch({ cancelRefetch: false })` to avoid restarting a pending request.

`packages/core/query-access.ts` exports:

```ts
withQueryAccessBoundary<TData>(
  queryFn: (context: QueryFunctionContext) => Promise<TData>,
): (context: QueryFunctionContext) => Promise<TData>
```

The wrapper captures the exact Query before awaiting. An active 401/403 clears `data` and `dataUpdatedAt` so a later 500 or remount cannot restore denied data. Cancelled requests and stale credential responses cannot clear a newer snapshot. Other errors preserve cache. It changes no retry options and owns no separate data store.

For options with `select` or a data-reading callback such as `refetchInterval`, bind the wrapped query function to a local constant before passing it to `queryOptions`. A compiler probe confirmed that the inline generic call loses contextual response inference; hoisting preserves types without assertions or repeated response types. Lane B owns adoption in shared workspace queries, and lane C owns provider adoption.

## Files changed by this lane

Core:

- `packages/core/query-access.ts` (new)
- `packages/core/query-access.test.ts` (new)
- `packages/core/issue-statuses/queries.ts`
- `packages/core/labels/queries.ts`
- `packages/core/properties/queries.ts`
- `packages/core/quick-actions/queries.ts`
- `packages/core/triage/queries.ts`

Views and tests:

- `packages/views/settings/components/settings-query-feedback.tsx` (new)
- `packages/views/settings/components/settings-query-feedback.test.ts` (new)
- `packages/views/settings/components/labels-tab.tsx`
- `packages/views/settings/components/properties-tab.tsx`
- `packages/views/settings/components/quick-actions-tab.tsx`
- `packages/views/settings/components/issue-statuses-tab.tsx`
- `packages/views/settings/components/keyboard-shortcuts-tab.tsx`
- `packages/views/settings/components/keyboard-shortcuts-tab.test.tsx`
- `packages/views/settings/components/catalog-recovery.test.tsx` (new)
- `packages/views/settings/components/issue-statuses-recovery.test.tsx` (new)
- `packages/views/triage/triage-settings-tab.tsx`
- `packages/views/triage/triage-settings-tab.test.tsx`
- `packages/views/triage/triage-settings-recovery.test.tsx` (new)

Task artifacts:

- `research/locale-catalogs.json`: bilingual nested additions, merged into locale files by the coordinator.
- `research/catalogs-result.md`: this report.

## Verification evidence

All final commands below exited successfully.

| Command/scope | Result |
| --- | --- |
| `pnpm -C packages/views test settings/components/catalog-recovery.test.tsx settings/components/labels-tab.test.tsx settings/components/keyboard-shortcuts-tab.test.tsx settings/components/issue-statuses-recovery.test.tsx settings/components/issue-statuses-tab.test.tsx settings/components/issue-statuses-ordering.test.tsx settings/components/settings-query-feedback.test.ts triage/triage-settings-tab.test.tsx triage/triage-settings-recovery.test.tsx` | 47 tests passed, 9 files. Includes the existing status-ordering and archive protections. |
| `pnpm -C packages/core test query-access.test.ts labels properties quick-actions issue-statuses triage/queries.test.ts` | 47 tests passed, 6 files. Includes denied-cache persistence, cancellation, stale credentials and non-access error retention. |
| `pnpm -C packages/views typecheck` | Passed. |
| `pnpm -C packages/core typecheck` | Passed. |
| `pnpm -C packages/views exec eslint` with the lane's changed view/test paths | Passed. |
| `pnpm -C packages/core exec eslint query-access.ts query-access.test.ts labels/queries.ts properties/queries.ts quick-actions/queries.ts issue-statuses/queries.ts triage/queries.ts` | Passed. |
| Impeccable `detect --json` on all seven changed view components | Returned `[]`. |
| `git diff --check` on owned paths | Passed. |

Before implementation, the new catalog/shortcut suite had 8 failures (with 8 incumbent shortcut cases passing), and the triage recovery suite had 2 failures. The first unrestricted status-loop probe timed out; the final bounded version safely reproduced exactly 11 calls against the one-call expectation before the fix. Its fixture caps repeated failures so a future regression cannot spin indefinitely.

Local command logs are `/tmp/multica-catalogs-{status-red,controls-red,triage-red,green,core-tests,views-typecheck,core-typecheck,views-eslint,core-eslint}.log`; the detector output is `/tmp/multica-catalogs-detector.json`.

## Remaining acceptance and limits

There are no remaining known lane source, lint, type or test failures. The coordinator must still record native 900px/450 CSS px menu reachability, 12-option dialog geometry, screen-reader/AX values and measured light/dark contrast against the audit baseline. These package tests use memory-only API fixtures and establish component/query behavior, not real service writes or backend throughput. No real credentials, invitations, plugins or agents were created or executed.
