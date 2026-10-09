# Verification record

## Baseline

- Base commit: `06d91b0eb`; branch `codex/projects-p1` (already ahead of its remote before this task).
- Incoming unrelated edit: `apps/web/next-env.d.ts` imports `./.next/types/routes.d.ts`. Preserve those bytes; do not stage the file.
- `pnpm --filter @multica/core exec vitest run iterations --maxWorkers=2`: **12 files / 144 tests passed** before changes, `evidence/baseline-core.log`.
- `pnpm --filter @multica/views exec vitest run iterations/iteration-events.test.tsx iterations/iteration-navigation.test.tsx iterations/iteration-history.test.tsx --maxWorkers=2`: **3 files / 65 tests passed** before changes, `evidence/baseline-views.log`.
- Original screenshot copied to `evidence/reference-planned.png`.

## Test environment

- Fresh environment: `check-20261009161520-47936`, created with the repository's `scripts/check.sh` preparation helper. The existing expired checkout environment was not reused or collected.
- Dedicated database: `multica_check_20261009161520_47936_api`; API at `http://localhost:18591`, reserved production Web at `http://localhost:13511`.
- API built and started successfully from the task checkout at base commit. API runtime health verified process ownership.
- Public `/api/config` confirmed legacy auth, managed installations disabled and platform administration disabled for this synthetic test environment. Source user env was not modified.
- Environment configuration/secrets remain in the external registry. `browser-evidence/runtime.json` records only safe names, URLs and paths.
- PostgreSQL client is installed at `/opt/homebrew/opt/libpq/bin`; prepend that directory to PATH for the existing launcher. No dependencies installed.
- This initial runtime was used for preparation only; final production builds and browser runs completed in the isolated runtime described below.

## Implementation and review

Core implementation is complete. New `scope.ts`/`scope.test.ts` and iterations exports retain the service counters and provide phase, event impact, metric evidence and complete-scope filtering. The first red run exercised 72 new tests (57 failed against a placeholder); additional focused regressions brought the final count to 79 new tests. The net-evidence mismatch check separately reproduced two failures before being fixed.

- Core iteration suite: **13 files / 223 tests passed**, `evidence/core-iterations-final.log`.
- Core typecheck and lint: passed, `evidence/core-typecheck-final.log` and `evidence/core-lint-final.log`.
- Signed net-evidence regression: `evidence/net-evidence-red.log` retains the failing proof; the final suite above includes the passing repair.
- The independent Trellis reviewer confirmed the core slice with no remaining findings, 79/79 focused tests and passing core lint/typecheck.

The UI implementer completed 246/246 iteration and locale tests before independent review. The reviewer reproduced and fixed three further regressions: selecting the same metric must clear its task refinements; planned cancellation/restoration categories must use known status changes independently of post-start metrics; a newly frozen snapshot must not retain old live-read errors or offer a live refetch. The final focused events suite passed 24/24, with changed-file ESLint and whitespace checks passing. No scoped review finding remained.

Another session created a separate triage task/critique while this work was running. All code owners have been reminded to retain iteration-only write boundaries and leave those artifacts untouched.

## Isolated verification of the final application sources

Concurrent triage work subsequently changed source and tests and produced unrelated type errors during the reviewer's shared-checkout typecheck. Main created a detached verification worktree at `06d91b0eb`, copied only the 16-file allowlist in `evidence/verification-files.json`, and SHA-256 checked every copy. Existing installed dependencies were cloned; workspace links resolve inside the verification checkout. No dependencies were added.

The verification checkout is recorded in `browser-evidence/verification-checkout.json`; copied source fingerprints are in `browser-evidence/verification-source.json`. This is the task's verification source, not the other session's evolving triage work.

- Fresh runtime: `check-20261009165940-62719`, with its own API database, API `http://localhost:18470` and production Web `http://localhost:13390`.
- API/Web provenance: `browser-evidence/api.running.json`, `web.running.json`; Web build ID `8NwDegBMW71-973bexToe`.
- Production Electron build: passed, `evidence/desktop-build.log`; output stays in the task runtime's external state directory.
- `pnpm exec turbo run lint typecheck --filter=@multica/core --filter=@multica/views --filter=@multica/web --filter=@multica/desktop --concurrency=1 --force`: **11/11 tasks passed, none cached**, `evidence/isolated-static.log`.
- Full affected-package unit tests with two workers per package: **10,564 tests / 885 files passed, none cached**, `evidence/isolated-tests.log`.

| Package | Files passed | Tests passed |
| --- | ---: | ---: |
| core | 235 | 2,918 |
| views | 528 | 6,394 |
| desktop | 86 | 970 |
| web | 36 | 282 |

### Browser round 1

Three real-API Web scenarios passed (English, Chinese, empty-plan/cancellation states). Both Electron scenarios reached the current-task link and failed the fixture's relative-href assertion: the desktop adapter intentionally renders the same target as an absolute shareable URL. The fixture is being corrected to compare normalized full URLs while retaining actual navigation/identity assertions. This is not a product navigation failure. `results-round1.json` records **3 passed / 2 failed / 0 skipped / 0 flaky**.

Visual inspection also caught an evidence defect: the shared full-page screenshot helper resets CDP-only viewport/touch overrides. First narrow DOM measurements were correct, but the image was cropped from a restored desktop layout and subsequent nominally narrow actions ran wide. `visual-verdict-round1.json` records the revise decision. The fixture will keep Playwright's viewport state synchronized, verify geometry before/after capture, and regenerate evidence. The application sources and successful static/unit/build results remain unchanged.

### Final browser validation

The corrected capture helper uses Playwright's tracked viewport plus explicit touch emulation and a viewport screenshot. Every saved layout asserts its expected width/height/coarse-pointer/touch count before and after capture and checks the PNG dimensions. A further fixture correction waits for the application's stable human-readable task route, as required by `IssueDetailRoute`, rather than racing its UUID-to-identifier replacement. Round 2 is retained in `results-round2.json`; neither correction changed application code.

The final production run passed **5/5 tests, 0 skipped, 0 unexpected, 0 flaky, with retries disabled**, in 34.7 seconds:

| Platform / locale | Scenario | Result |
| --- | --- | --- |
| Electron / English | Plan, exact scope evidence, filters, keyboard, current task, frozen history, narrow layout | Passed |
| Electron / Chinese | Same real-API flow and localized controls | Passed |
| Web / English | Same real-API flow | Passed |
| Web / Chinese | Same real-API flow | Passed |
| Web | Empty-plan adjustments, cancellation before start, true empty start and cancellation afterward | Passed |

Evidence: `browser-evidence/results-final.json`, `summary.json`, and `evidence/e2e-final.log`. The shared fixture uses real API operations and records no mocked branches. The four main flows also record empty page-error lists and preserve source statistics while exercising repeated leave/reentry and cancelled/restored transitions. Final task navigation checks both target identity and the displayed title.

Native coverage uses the existing isolated Electron renderer/preload fixture and a production renderer build with HTTP/WebSocket proxying. It is not a Windows/Linux installer or full desktop-main-process release acceptance run. No real agent execution was started.

### Visual and mechanical UI checks

- Final `visual-verdict`: **94/100, pass**, `browser-evidence/visual-verdict.json`.
- Manually reviewed wide Chinese Electron, narrow Chinese Electron, narrow Web frozen/current comparison and wide English Web screenshots under `browser-evidence/review/`.
- The corrected narrow captures show collapsed sidebars, contained controls and wrapped long titles. Historical/current values have separate visible labels; technical audit remains collapsed until explicitly opened.
- Impeccable detector over all four changed UI surfaces: **no findings**, `evidence/ui-detector.json` (`[]`).
- Strict standalone typecheck of the three E2E files passed, `evidence/e2e-fixture-typecheck.log`.
- Raw Playwright captures and test logs remain local under the task/repository ignore rules. Selected review images, normalized browser reports and `evidence/checks-summary.json` are committed; the summary records raw-log hashes and exact check counts.

## Remaining limits

- Windows/Linux packaged desktop behavior and live deployment were not exercised; this task changes shared frontend presentation and has no schema/deployment migration.
- Live activity and detail metrics remain independent reads. Exact task sets check revision and count; activity is described as supporting evidence, and incomplete evidence is explicit.
- Full Go unit/race suites were not rerun because no backend code changed. The browser scenarios exercised the actual iteration APIs and database transactions.

## Cleanup

Synthetic feature workspaces were removed in each browser fixture's `finally`, including failed fixture runs. Both task-created runtimes were stopped and destroyed after explicit ownership/database-name checks; their two synthetic databases and temporary profiles were removed. The verification worktree was removed only after its changes matched the allowlist and all source hashes matched the original checkout. The earlier expired environment and parallel triage work were left untouched.

## Delivery

Implementation and executable specs committed as `6d3bf906d9639924e2d6493b3349e0d98bfafb7d`. The commit contains only the 17 owned files. Requirements, design, execution plan, research and selected verification evidence are archived with this task.

Task archived on 2026-10-10; developer journal session 51 records the implementation commit and verification. All task acceptance and execution checkboxes are complete.
