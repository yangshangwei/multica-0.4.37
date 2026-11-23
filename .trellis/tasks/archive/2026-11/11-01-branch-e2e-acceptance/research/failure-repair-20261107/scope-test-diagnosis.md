# Independent scope-test diagnosis

**The repaired-canonical Web and Desktop scope failures occur at different boundaries. Only Web is a confirmed label mismatch.** No source, runtime, database or user state was changed by this read-only review.

## Web: obsolete Chinese label

Raw failure is `global-scope-navigation.spec.ts:197`, after the recovery/620 section, explicit-anchor resume and compact-width checks. It expects the exact button `协作`. The saved ARIA shows navigation `应用导航` with buttons `工作空间` and **`协作空间`**. Current `GlobalScopeNavigation` reads `collaboration.page.title`, whose current Chinese value is also `协作空间`. It is not `跨账号协作` in this source/artifact.

Minimal fixture correction: use the exact current product label or a shared catalog-backed fixture constant; preserve role/name specificity, visible navigation and no-overflow assertions. No product-label change is warranted.

## Desktop: recovery has not removed the error banner

The actual failure is earlier, line158: after `releaseRecovery()`, the top-level alert remains1 for the unchanged5s assertion. It never reaches the Chinese label assertion. The ready promise signals handler entry, not a completed successful response. The active route set can include older503 handlers; the finalizer correctly releases and awaits them before unregistering.

No trace ZIP, native failure DOM, or per-request fetch/fulfill/abort ledger was retained for this run. The API log during the exact16.27s Desktop test shows the selected issue alternately returning503 and200, with last200 records at15:45:12.519/.524/.540 local time. Other endpoints also report503 during scope switches. This alone cannot identify the held request consumed by the renderer, establish a backend incident, or prove an injection race.

Recommended bounded next diagnostic: retain operation ID, actual request method/path, failure/recovery branch at handler entry, real upstream response status and timing, fulfillment completion, browser request-failure/cancellation, and banner text. Check that the released **current GET** actually receives200. If that transport boundary is proved, await it before the existing UI assertion; do not raise the5s timeout, force an artificial200, remove the banner, or lower620. A stale handler/canceled request is a hypothesis until this correlation exists.

Old pre-fix scope regression remains separate: recovery-regression-red-2 failedWeb620→660 andDesktop620→580. These are not the latest recovery-banner or label failures. The canonical detail A17 screenshot failure is outside this read-only slice and remains with the scroll owner.

Exact raw errors, source references, hashes and bounded API log lines are in `scope-test-diagnosis.json`. Findings and the suggested passive probe were sent directly to the scroll implementer and parent.
