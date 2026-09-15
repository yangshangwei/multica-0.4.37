# Verification and release evidence

Source verification, publication and final artifact validation are complete.

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

## Released artifacts and final checks

- PR #1 merged to main: https://github.com/yangshangwei/multica-0.4.37/pull/1
- Release: https://github.com/yangshangwei/multica-0.4.37/releases/tag/v0.4.45
- Immutable release source: `88522b44478fc59ccf1a6a4556b8ffa99d1ca01d`.
- PR CI `34899068659` and main CI `34905382224`: passed.
- Release workflow `34905861554`: passed.
- Branch Windows build/install `34899554181` and final tagged build/install `34906105951`: passed. Final Windows JSON confirms the exact installer was silently installed and x64 desktop/CLI versions matched; GUI and report-model execution are explicitly not claimed.
- Final Windows installer: 177,516,218 bytes, SHA256 `82c9a7f40b78d0437039d0d6461727b720d8ebbc68b781dbf92c7a6ce101ed2d`. ASAR, renderer assets, PE/build metadata, updater SHA512 and all 8,513 blockmap chunks verified.
- Final Linux amd64 upgrade archive: 287,449,725 bytes, SHA256 `88fe470265f2356166b7708bb37ded1e5966627eb1df1e3d391def51392ac56e`. On macOS use `COPYFILE_DISABLE=1` for the outer tar: the first archive included AppleDouble metadata; the final clean archive preserves all 12 payload file hashes exactly.
- Actual upgrade `upgrade-smoke-2`: passed all ten check groups and scoped cleanup. A prior v0.4.42 deployment retained its API task, SQL sentinel, JWT, secrets and ports; the new image selection survived a normal Compose force-recreation with transient overrides removed. Backend/CLI commit and version, actual web X-Client-Version, nine-role/ten-template catalogs and published feed all matched. The controller ran in genuine Rosetta x86_64 Bash and the services in Linux amd64 containers; architecture checks and health responses were not spoofed.
- Eleven uploaded delivery assets were verified against GitHub's actual SHA256 digests and byte sizes; the three original changelog assets were retained.
- Delivery directory: `dist/release/v0.4.45/` in the primary checkout. It includes installers, individual/combined checksums, Chinese upgrade instructions and verification summaries.
- Preserve `%USERPROFILE%\.multica\desktop.json` and its `updateUrl` for intranet desktop updates; omitted updateUrl still uses the original upstream feed.

## Operational notes

- Use a task-local Corepack shim directory at the front of PATH for Turbo subprocesses: this host's outer pnpm shim is v11 while the project uses v10.28.2. No global package-manager settings were changed.
- This host's Lore pre-tool hook recognizes only its fixed Lore keys in the final footer. Use an inline message ending in those keys plus `Co-authored-by: OmX <omx@oh-my-codex.dev>`; custom public-note trailers in that footer make the entire block unrecognized. These commits therefore use public-facing Conventional subjects. The guard was not disabled or modified.
- Test web/static servers and the task-owned API were stopped after successful validation. The primary local API was rebuilt to the released source. After amd64 image loading, restore the shared development PostgreSQL image to native arm64 with its original `multica_pgdata` volume; data health was rechecked.
