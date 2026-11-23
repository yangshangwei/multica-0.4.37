# Independent final evidence check

Reviewed the `2026-11-01T17:37:50Z` aggregate against retained raw results on frozen target `822c15785449b6c1cef699f6fd4d8e1f5483bd5e`. The acceptance verdict **NOT PASSED** is supported. No product code, tracked test, existing failure, database or runtime state was changed by this review.

## Verified evidence

- The 437 existing case keys are unique and all have terminal acceptance classifications: 421 PASS, 14 FAIL and 2 BLOCKED. The additional ENV-1 invariant fails, giving 438 planned cases including that addition.
- Independently parsed all 189 Playwright JSON reports. Their 553 terminal test results match the published execution identity multiset, including run, original/derived filename, title, project and raw status. Raw results are 449 passed, 79 failed, 19 skipped, 5 timed out and 1 interrupted. All 553 results have `retry=0`.
- Acceptance classifications preserve those results as 449 PASS, 52 FAIL, 19 SKIPPED and 33 BLOCKED. Original failures remain in the execution inventory; later passes do not erase them.
- All 653 declared attachments exist: 388 PNG, 31 Markdown, 40 ZIP, 186 JSON and 8 plain text. A bounded scan of public text artifacts found no JWT, long PAT or credential-bearing PostgreSQL URL matches. This scan does not inspect ZIP contents or prove absence of every possible secret format.
- Compared the four derived suites with frozen originals. Lifecycle/control copies add observation only; workbench changes recipient interaction; memory adds explicit history reopening. Original business assertions are retained. The passing derived suites do not repair the tracked original fixtures.
- Both detached worktrees remain at the target commit. Their only tracked differences are the documented Next-generated route-types import rewrite. Retained production-Web records include source/configuration fingerprints, target commit and build IDs.
- Independently probed all six cleanup-record ports with `lsof`; none has a listener. The retained cleanup record also reports task Redis stopped.
- ENV-1 retains mutually consistent API, persistent base-version and production-UI evidence. Its seeded run proves the attribution defect; it does not establish that actual Provider input changed.

## Findings (fixed)

- File: `.omx/reports/branch-e2e-822c15785-20261101/review/tool-versions.json`
- Issue: the original `baseline/tool-versions.txt` contains only Node's version.
- Fix: added a separately timed review supplement recording Node, pnpm, Go, psql, Playwright, installed Electron package and Docker client/server versions. The original baseline is preserved. These are review-time observations, not retroactive launch-time measurements.

## Findings (not fixed)

- The final `report.md`, `defects.md` and `evidence-index.md` were not yet present when this check ran. Their final prose/link audit remains with the coordinating session. The then-current `remaining-risks.md` and execution checklist also contain interim state and require synchronization. These shared final documents were left to their active owner.
- The Go summary combines selected completed packages from the interrupted shared-database run with later isolated-package and Redis runs. `cmd/server`, handlers and earlier database-backed packages completed before per-package isolation was introduced. Preserve this isolation limit and the original migration/service failures; do not describe the result as one clean complete fresh-database run. The selected test summary contains 10,261 passing and 58 skipped top-level identities, plus 10,359 passing and 1 skipped subtest identities; those are selected unique identities, not all attempts.
- Earlier launcher invocations lack Playwright JSON because the command failed before test collection, including the unsupported screenshot option. They remain command evidence. The count 553 describes parsed test results, not all launched commands. Three collection-error records also remain separate.
- The restored password gate is `enabled=false`, `creation_enabled=true`, `host_execution_enabled=false`, epoch 5, matching its saved synthetic baseline. Do not claim all three booleans are false. Cleanup must distinguish baseline restoration from closing every gate.
- The 14 remaining existing E2E failures, ENV-1, the retained TypeScript failure, historical open runtime/knowledge findings and unavailable platform/Provider conditions prevent a full acceptance PASS. Product repairs and broader acceptance decisions are outside this test-only review scope.

## Verification

- Lint: PASS, independently rerun on the frozen target.
- TypeCheck: PASS, independently rerun on the frozen target.
- Command: `pnpm exec turbo run lint typecheck --filter=!@multica/mobile --concurrency=1 --force`; 15/15 tasks successful, exit 0. Evidence: `review/static-execution.json` and `review/static.log` under the report root.
- Tests: FAIL overall, based on preserved raw acceptance/unit outcomes; no broad browser/Go rerun was performed by the reviewer. Independent result-identity, count and attachment audits passed.
- Spec sync: no new product contract or convention was introduced. Existing environment/isolation specs already describe the relevant limits; the findings belong in this task's evidence and final report.

Review writes are confined to this document and the ignored `review/` evidence directory. No simplification or product fix was made. The remaining risk is incomplete/failed acceptance, not an unverified claim that the frozen branch is release-ready.
