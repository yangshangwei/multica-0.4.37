# v5 adapter / final4 launcher independent preflight review

Verdict: **changes-required**. Three blocking launcher/environment contracts remain open. AV4-1 is corrected, and the current fake suite passes 11/11; those passes do not authorize a complete check before the items below are corrected and independently re-reviewed.

## Findings (not fixed)

### AV5-P1 — API pool overrides contaminate Go tests (high)

`/Volumes/artisan/code/2026/mc-collaboration-final4-20261102/.omx/private/final-candidate/run-make-check.py:25` exports `DATABASE_MAX_CONNS=10` and `DATABASE_MIN_CONNS=1` for the entire check. `/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v5/check-bin/go:136`–`:147` copies them into every package test subprocess. `server/cmd/server/dbstats_test.go:24` / `:33` only calls `t.Setenv` for a nonempty requested override; `TestPoolSizing_DefaultsWhenNothingSet`, `TestPoolSizing_URLParamsHonoredWhenEnvUnset` and `TestPoolSizing_PartialURLParam` therefore receive settings whose required input is unset. The fake propagation probe captured max10/min1 in both package children without executing real Go or SQL.

Keep API pool settings at the task service-start boundary and remove those keys from actual Go test children, including recursive mode forwarding. Add a fake env witness asserting those keys are absent while the selected owned database remains present. Keep original product tests unchanged.

### AV5-P2 — dotenv is executed as shell (medium)

`/Volumes/artisan/code/2026/mc-collaboration-final4-20261102/.omx/private/final-candidate/run-make-check.py:16` invokes `bash -c 'set -a; . "$1"; env -0'`. The current database isolation contract explicitly says not to source dotenv as shell code. Parse the task configuration as data and reject unsupported syntax/expansion. Because repository `scripts/check.sh:131` reloads its selected file, retain the original bytes and, where needed, provide a separately fingerprinted validated/canonical shell-safe task copy. This report does not request a broad repository env-loader refactor.

### AV5-P3 — review clearance is not bound to actual executable bytes (medium)

`/Volumes/artisan/code/2026/mc-collaboration-final4-20261102/.omx/private/final-candidate/run-make-check.py:14` compares clearance-declared adapter hashes with constants; `:34` computes actual hashes only to record them. Neither actual adapter hash nor launcher hash is checked against the review before `Popen` at `:36`. Current v5 bytes do match the older expected values, but the required launch-time reviewed-byte invariant is absent. Compare actual current adapter/launcher hashes and selected candidate/configuration to the independent clearance before starting services or tests, and exercise mismatch rejection with fakes.

No source fix was made: the dispatch explicitly reserves adapters, launcher and clearance to root / implementation ownership.

## Verified positive contracts

- V4 AV4-1 is closed by refusal of unmarked ordinary tests before real Go/SQL; real non-test forwarding and compile-only handling remain separate.
- All 11 fake cases pass, including consumed `-c` values, `-args`/`--` refusal, later `-c=false`, distinct package clones, missing/remote/default context, recursive ledger requirement, named-test accounting and pnpm guard wiring.
- The current full-check command shape in `scripts/test-go.sh` is preserved. All packages are expanded, serial test invocations are retained, no-test-files is labeled compilation-only and silent success is refused.
- pnpm retains test selections, workers/retries and explicit reporter. It adds default line/JSON reporting only if absent and invokes the actual candidate Agent CLI guard.
- The launcher strips inherited `MULTICA_TASK_*`, listed Multica identity/token keys and real-smoke gate. The adapter itself forwards task-root variables, but the launcher's prior strip covers inherited values; the probe does not establish task-root contamination in the current final4 launch.
- HOME remains inherited. The bounded CLI test inspection found explicit fixture HOME overrides and did not establish actual user-account access. A package-owned HOME with explicit Go caches is the documented isolation pattern when required; do not globally override test-owned task-root variables.

## Verification

Command: `/opt/homebrew/opt/python@3.14/bin/python3.14 /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v5/check-go-isolation-test.py`. Actual exit **0**, 11 named tests passed, zero failed. Complete stdout/stderr and their hashes are retained in `adapter-review.json`. All four reviewed Python sources parse via `ast.parse`. Lint/TypeCheck are not run because no JS/TS source was changed and this dispatch permits only fake/AST verification. No real database, migration, service, browser, installed Provider/CLI, account access, network or dependency install occurred.

Source bindings:

| File | SHA-256 |
|---|---|
| v5 Go | `2c32f8309a17c5558e004184ec490df3bb6aebbf6101ee0f9156dae3d09e7943` |
| v5 pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| fake suite | `5c5aad4d0e04427a1c53eadec2067a2ac355701c9d4605e722a87ccf9acad4e7` |
| final4 launcher | `3c50f99cb9b995fa29a0d1ee4e54e98d6aaa28bdf10ebc827c3cb5b302f5f566` |

No independent clearance was written. Full candidate acceptance and actual listener/database resource readiness remain coordinator gates.
