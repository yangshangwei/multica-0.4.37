# Verification — 2026-10-17

All three reported defects are repaired. Fresh baseline reproduced all six original failures; those same identities now pass. Across the related suites, all 28 unique existing cases pass with zero skipped cases and zero automatic retries.

## Changes and acceptance

| Requirement | Repair | Evidence |
|---|---|---|
| QA-01 / AC1 | Keep the complete localized confirmation error in layout while hiding inactive error semantics; blur no longer moves Submit between pointer down/up. | Original exact-match browser case passes; both password suites: 14/14. Invalid input sends no request, correction submits once, visibility controls and pending-submit protection pass. |
| QA-02 / AC2 | Reduce only the compact recommendation list cap; retain all suggestions and native scrolling. | EN/ZH 375×812 retain at least half the editor and all of Create; Tab reaches all three suggestions, last adoption preserves text/default/history. Final recommendation suite: 4/4, including Electron. Visual verdict: 97/pass. |
| QA-03 / AC3 | Select semantic listitems and their exact named native title buttons. | All three original inbox cases pass with content/emoji, read/archive, reload and narrow-screen return assertions retained. |
| AC4 | Preserve failures, configured production provenance and unique case accounting. | Baseline: 5 legacy + 1 password failures. Final unique result: 28/28; repeated recommendation runs are deduplicated by file/suite/title/project. |
| AC5 | Separate test checkout, database, development API and production Web, provider, renderer and profile. | Task source/test SHA-256 values match the tested checkout; no dependencies added; unrelated local edits preserved. Cleanup logs record owned resources removed. |

## Checks

- Views: **563 test files / 6907 tests passed** (`pnpm --filter @multica/views exec vitest run --maxWorkers=2`).
- Non-mobile monorepo lint/typecheck: **15 tasks passed**, 31 pre-existing lint warnings and no lint errors (`pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=2 --force`).
- Strict TypeScript check of the three changed E2E files, UI export validation, and `git diff --check` passed.
- Production Next build and Electron main/preload/renderer build passed. Browser checks target `next start`, never development compilation.
- Independent source/evidence review found no required changes. Final visual review inspected six settled EN/ZH/Web/Electron screenshots.

## Environment correction retained in the record

The first expanded native run failed before opening the dialog: the isolated API allowed the Web origin but omitted the static Electron renderer origin. Diagnostic console, screenshot and OPTIONS response established CORS blocking `/api/me` and `/api/config`. Adding the exact renderer origin only to the task environment and restarting its API made the unchanged native case pass. The initial failure and diagnostic remain recorded; no product permission policy or assertion was weakened.

## Evidence and scope

Full evidence: `.omx/reports/qa-three-regressions-20261017/`.

- `result-summary.json`: 28 unique identities and six-defect mapping.
- `baseline-legacy.json`, `baseline-password.json`: fresh original failures.
- `repaired-password.json`, `repaired-legacy.json`, `final-recommendations.json`: final results and retained environment failure.
- `*-api.running.json`, `*-web.running.json`, `repaired-files.json`: commit/build/configuration provenance and exact task-file hashes.
- `qa02-visual-final.json`, `independent-review.md`, `cleanup.log`: visual, review and cleanup evidence.
- `*-command.txt`, `run-e2e.sh`, `set-auth-mode.sh`: exact commands and task environment handling. A fresh isolated environment is required to reproduce after cleanup; credentials are not stored in task artifacts.

The test checkout starts at `4806f6167` plus only these repair files. The unrelated pre-existing local sidebar/docs/journal edits are outside its product snapshot. This task reran the 28 relevant cases, not the previous full 328-case inventory. No backend code changed, so Go tests were not repeated. The recommendation provider and unrelated native services are controlled fixtures; no paid model or real agent execution is claimed. Windows/Linux packaging, production deployment and pushing commits are outside scope.

## Work commits

- `44e675384`: stable registration validation.
- `1a47a28d5`: compact recommendations and keyboard/screenshot regression.
- `52a451bad`: semantic inbox test controls.
