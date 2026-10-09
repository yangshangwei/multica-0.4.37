# Implementation and verification progress

User approved both business pages and settings on 2026-10-08. Both parent and child are in_progress. No release/push requested.

Pre-existing concurrent timezone/project edits were preserved; they were independently committed as ed4187662 while this task was running. Initial snapshot is recorded at .omx/state/iteration-workspace-settings/baseline-path.txt. Leave .trellis/.template-hashes.json, installed .agents/.codex files, next-env generated changes and other task evidence alone.

## Implemented

- Backend retired iterations_i1 and Available. Workspace enablement, human/admin checks, default disabled, history, request recovery preserved. Enable broadcasts only after committed writes, not receipt replay.
- Settings tab/sidebar/runtime caches, shared planning-timezone reuse and controlled IterationOperation complete. Owner/admin writes, ordinary member read-only. Recovery outside availability gates.
- Business overview/detail, timeline/catalogue helpers, tabs, empty state, real task assignment and manual preselected iteration creation complete. Metadata-only future/history rows, no forecasting/capacity/cooldown additions.
- Review fixed active row pagination, emptied-plan effective tab, dialog draft retention (optional UI DialogContent keepMounted), fresh-write availability, external-disable dialog closure and timezone confirmation cache/retry lockups.
- Config samples/offline guide/current specs updated; no DB migration and no existing workspace switched automatically.

## Completed checks

- Backend guarded Iteration|Iterations|Triage suite passed (196 handler and 86 service top-level tests selected); full iteration (37) and featureflags (7) lower-level suites passed; relevant go vet passed. Opt-in performance cases remain opt-in. /tmp/multica-iteration-backend-tests.log.
- Core catalogue/timeline/recovery related 33 tests, core typecheck and scoped eslint passed.
- Modal lane 66 tests, scoped eslint passed.
- Settings lockup/operation/timezone suites 33 tests passed with three red-before-green cases.
- Root combined navigation/settings 37 tests passed; four additional business-review regressions now green.
- Initial repository lint/typecheck: 15 tasks successful. Repeat affected checks after final fixes as needed.
- Offline script/config suite: 32 pass, 3 existing real-image opt-ins skipped.
- UI exports check passed.

## Running / remaining

All non-mobile TS tests running sequential packages/two workers; log .omx/state/iteration-workspace-settings/logs/all-ts-tests.log. Production Web and Desktop builds now running; logs in browser registry and .omx/state/iteration-workspace-settings/logs/desktop-build.log.

Separate Go test DB env: .omx/state/iteration-workspace-settings/go-test.env (private; do not print contents). psql requires /opt/homebrew/opt/libpq/bin on PATH.
Browser registry path recorded in .omx/state/iteration-workspace-settings/browser-state-dir.txt. Current API localhost:18590, Web localhost:13510, verification environment check-20261008121012-21837. API is healthy; use its check.env through dev-env.sh exec for browser commands. No changes to root .env.

Root owns E2E integration: e2e/iteration-settings.spec.ts is new; I1 Web/desktop/history fixtures adapted to real settings tab, expanded assignment dialog, detail tabs and effective server timezone. Run with MULTICA_RUN_I1_E2E=1 and fixture env. Need real Web + Electron tests, screenshot comparison, final static checks, complete task checklists, scoped commit, archive/journal. Do not claim completion before these.
