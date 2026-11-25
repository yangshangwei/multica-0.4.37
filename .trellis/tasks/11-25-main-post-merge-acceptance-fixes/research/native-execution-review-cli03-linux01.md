# Native execution review — CLI03 and Linux01

CLI03 is **accepted for bounded execution**. Linux01 is **not accepted** until the three local runner issues below are corrected and its request is rebound. No source or runner was edited; no tests, services or Docker mutations were run.

## Findings (fixed)

None. This reviewer owns only `research/native-execution-review-*.md/json`.

## Findings (not fixed)

1. `run_linux.py:16` reads `native_api01`, whereas request `9feb8ffe380fbf0eb70f0ae7d38a417e9f315fffef205786b31b9c3fe7396d1e` binds newly allocated `native_linux_api01`. This would run against a different DB than reviewed. Select the exact allocation and rehash.
2. `run_linux.py:83–87` discards the recorded CID file identity when constructing final cleanup input. Preserve valid recorded container IDs as well as owner intents, so `cleanup.py` can reject an ID replacement before removing anything.
3. `run_linux.py:81,95` records log-close failures but can still exit0. Include those errors in final failure classification; preserve the original test failure independently.

These are mechanical fixes for the runner owner, Hooke. The review instruction prohibits editing them here.

## Accepted CLI03 scope

`native-execution-review-cli03.json` has `accepted:true` and request SHA `8f7aa519ef94dd440ad681927814ada9b2f578531d6dff7f890e2d2d97bda07e`. Its embedded bindings include all exact script hashes and the allowed command. The only source02→source03 file difference is the five independent snapshot reads now awaited through `Promise.all`. Ten CLI operations, state/authority/history assertions,240-second budget,workers1,retries0,12/1 API pool and cleanup rules are unchanged. Full frozen-tree audit matched149210 entries and found no escaping links. Actual cached image and parent PG metadata matched.

The CLI permit is independent of Linux and macOS. All previous timeout/cleanup failures remain failures. Execution results and independent final cleanup still need review.

## Linux reviewed boundary

Package80 entries and controller559 entries match. Host/controller Playwright all1.58.2,26provider overrides,synthetic machine/discovery identity,real packaged Main/file renderer and complete600-second original journey are preserved. Docker defaults remain unchanged; no macOS protocol mutation occurs. JSON records the accepted static subset and exact blocking findings.

## Verification

- Read-only hashes,metadata and Python AST parsing: pass.
- Lint: not run; this task explicitly restricts review to read-only metadata/source inspection.
- TypeCheck: not run under the same restriction.
- Tests: not run,as explicitly requested.
- Source/tool/asset identity and execution eligibility are distinct from functional acceptance.

Both requests use an older frozen native candidate. Neither includes the newly approved account-resume UI or local comment-principal fixes. A new final integrated candidate binding and regression remain required. No new product/spec contract was introduced by this read-only review.
