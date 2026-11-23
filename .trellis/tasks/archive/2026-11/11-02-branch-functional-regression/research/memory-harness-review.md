# Independent memory harness review

Verdict: **PASS WITH NOTES**. No blocking finding in the one-line interaction repair for the observed integration flow. This reviews the test harness and its evidence handling; it does not approve a whole-system acceptance result or the old adapter-v5/RKI6 launcher.

## Scope inspected

- Task PRD/design and curated check context.
- Retained `memory-history-harness.patch` and binding JSON.
- The actual isolated checkout diff in `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102`.
- Product `packages/views/collaboration/memory-panel.tsx`, its component tests, and the surrounding account/detail/workflow mounting code.
- Original full-run and original single-case replay records, failure ARIA evidence, and the completed corrected-harness memory/knowledge run.
- The added patch recording in the private `run-e2e.py` and the line-migration fallback in `evidence/reconcile-e2e.py`.

No tests, database connections, APIs, services, Provider processes, or Git mutations were executed by this reviewer. Only this review file was written.

## Verified findings

1. The isolated E2E diff contains exactly one added normal user interaction: clicking **Version history** after completion and the closed-state checks, before the existing **Purge v2** click. No assertion was deleted or relaxed. No timeout, retry, force-click, route fulfillment, authorization rule, or product implementation was changed by this patch.
2. The original full run remains exit 1: **349 expected / 1 unexpected / 89 skipped**. Its unmodified single-case replay remains exit 1 with the same missing Purge control. The retained ARIA snapshot shows a closed issue, read-only shared context and a collapsed Version history control; it does not show an authorization error or a visible disabled Purge control.
3. The product renders history through a controlled `Collapsible` (`memory-panel.tsx:178`). Historical purge depends on `memory.can_purge`, a noncurrent version, and an unpurged item (`memory-panel.tsx:218`). The new interaction opens that existing UI rather than bypassing these conditions. The original confirmation/reason requirement, real purge write, version-2 `content: null`/`purged: true`, current version 4, closed editing refusal, narrow layout, and page-error assertions remain in the test.
4. The corrected-harness run `legacy-memory-harness-1` is complete with exit 0: **10 expected / 0 unexpected / 0 skipped / 0 flaky**, consisting of both memory cases and all eight knowledge cases. All individual raw results are `passed`; collection errors are empty. This is an actual supplemental result, not a reinterpretation of the failed full run.
5. The retained patch, the checkout's complete E2E diff, and the run's `harness.patch` all hash to `85b0ec959becb5b3ec4b19c53d8ae7347ffe65399df85e47638085cf0626e12e`. The run records the product candidate separately as `7af9ab9905c98c77bb091223ebe687c639475bb4` and declares the dirty harness path. The checkout also reports the previously noted generated `apps/web/next-env.d.ts` difference; that file is not part of this harness patch.
6. The reconciler first uses file/project/line/title identity, falls back only when the run declares a harness diff, and rejects nonunique matches. The second memory case moved from line 108 to 109 while retaining its complete title; the observed fallback is justified for that case. All original executions remain reachable through their own immutable result paths and run records.

## Nonblocking notes

- **N1 — Keep the collapse explanation factual.** `MemoryPanel` initializes `historyOpen` to false, but contains no explicit effect that closes history when `closed` changes. The component test at `memory-panel.test.tsx:125` changes the issue to closed within the same mount and still operates on its open history. The integration evidence establishes that history was collapsed after the full completion flow; the exact ancestor remount/navigation/query transition causing that collapse was not established by this review. Describe the repair as reopening the history observed closed in this integration flow. Do not claim that closing history on completion is a separately specified product requirement or that a UI state-retention regression has been conclusively ruled out.
- **N2 — Assert the closed restore restriction with the rows visible.** The existing `Restore v*` count check precedes reopening and can pass while history is collapsed. Repeating or moving that check immediately after opening history would provide stronger integration evidence that closed tasks expose historical purge while still refusing restore. The canonical component test and final API `can_edit: false` check already cover the underlying restriction, so this does not block the current purge interaction repair.
- **N3 — Narrow future line-migration fallback.** `reconcile-e2e.py:39` enables fallback for any file whenever any dirty harness path exists. The current result is unambiguous and correct, but future runs should restrict fallback to the result's specific file being present in `harness_dirty_paths`. Including the harness patch SHA in derived execution rows/coverage notes would also make provenance visible without a second lookup. Continue presenting latest per-case outcomes separately from the failed original full-run verdict.

The supplemental green verifies the corrected memory workflow and adjacent knowledge cases. It does not close the 89 conditional inventory items, the separate handler-package failure, missing platforms/accounts, or any inherited independent-review gate.
