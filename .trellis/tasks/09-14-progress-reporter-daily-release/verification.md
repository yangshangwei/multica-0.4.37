# Verification and release evidence

Source verification is complete; publication and final installer checks are still pending.

## Source and integration checks

- Unchanged registry baseline passed before edits. Role, daily-template and temporary-comment-file regressions each produced the expected red result before their fix.
- TypeScript typecheck: 9/9 package tasks, forced fresh execution.
- Lint: 6/6 package tasks, no errors; existing hook/disable warnings retained.
- TypeScript tests: 8,139 passed across the current package suites (docs 60, core 1,947, desktop 600, web 260, views 5,272), no cache replay.
- Guarded full Go race suite: passed, including subprocess-backed agent tests. The first full run found one stale seven-skill router assertion; it was updated to eight and the complete wrapper reran successfully.
- `go vet ./...`: passed. `go tool govulncheck ./...`: no vulnerabilities found.
- Production build: 5/5 package tasks, forced fresh execution.
- Packaging/changelog tests: 52 passed, 4 existing opt-in Docker cases skipped; selfhost config passed. The existing Desktop packaging suite passed 30/30; the full Desktop suite reran with two new Windows workflow contracts and passed 600/600.
- UI wildcard-export contract checks passed (62 files).
- Independent code review: spec PASS; quality APPROVE after report-body temporary-file permissions and durable offline image selection were fixed.

## Browser and desktop E2E

- Focused report tests: 3/3 passed against the current embedded API. They cover one listed reporter, actual role creation/default skill, reuse, adjacent daily/weekly templates, distinct cadence/timezone, prompt persistence, dispatch linkage, visible report comments, review-triggered run completion and unchanged source tasks.
- Complete suite: **85/85 passed, zero skips**, in 3.1 minutes against the production web build plus a real Electron fixture.
- First dev-server full run: 79 passed, 4 failed, 2 skipped. Logs proved Next dev memory-threshold restarts interrupted navigation; default intranet device login invalidated an unauthenticated-user test. The task-owned environment uses `next start`, `MULTICA_DEVICE_AUTH_ENABLED=false`, and the optional changelog/Electron fixture endpoints. No production auth or virtualizer behavior was changed to satisfy tests.
- Final frontend API is port 18714, production web 13634, isolated desktop renderer 13635. `.env.worktree` and generated evidence are not release inputs.
- Fourteen focused screenshots were inspected. Visual verdict 96/100, matching the supplied dark catalog design; daily is adjacent to weekly and 390px views have no horizontal overflow.

## Model evidence and limits

- Native Claude skill-eval failed with HTTP 429 Service Unavailable before skill or fixture tool calls. It is explicitly not a pass.
- Two fresh native Codex agents read isolated raw snapshots with the exact current role and skill. An independent reviewer recomputed the results: daily new=5, closed=4, net=+1; weekly new=8 with unavailable closed/net totals correctly marked unconfirmed.
- Custom done categories, duplicate records, reopens, cancellations, report exclusions, half-open time boundaries and incomplete histories were handled correctly. All 22 input files per case remained unchanged.
- These model reports are offline drafts, not live daemon execution or online publication. Browser/API tests separately prove comment delivery and closeout. Exact Codex model identity and standalone native tool-event traces were not captured; no claims about those are made.

## Existing advisory findings

`pnpm knip` reports the same nine unused files and two unused dependency declarations as unchanged main. CI explicitly runs this step with `continue-on-error: true`. No new finding was introduced and unrelated cleanup is out of scope.

## Local evidence

- `.omx/reports/progress-reporting-verification/`: complete source checks and production E2E logs.
- `.omx/reports/progress-reporting-e2e/`: focused red/green tests, API drift proof, screenshots and JSON.
- `.omx/reports/progress-reporting-model-eval/semantic-review.json`: independent actual-output review and hashes.
- `.omx/state/progress-reporting-e2e/ralph-progress.json`: visual verdict.

## Remaining release work

Commit and push; verify fork CI; create v0.4.45 from main; wait for publication; build and smoke the Linux amd64 upgrade archive; build and verify Windows x64/x86-64 installer; upload exact artifacts/checksums and document upgrade instructions. Native Windows installer/CLI verification is now part of the existing manual workflow; actual Windows execution remains a release gate.
