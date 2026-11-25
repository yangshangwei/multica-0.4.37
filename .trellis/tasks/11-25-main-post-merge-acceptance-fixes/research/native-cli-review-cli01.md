# Research: Full old-CLI cli01 adapter review

- Query: Review the exact full ten-operation CLI request before execution.
- Scope: internal, read-only source/binding review.
- Date: 2026-11-25

## Findings

**One cleanup repair required before accepting this full invocation.** Exact decision: `native-cli-review-cli01.json`, request SHA256 `743a86ae2ad98883d3d2c1bc88efc23dec56408e233b6e39e73291a3f75c58fb`.

`run_cli.py:63–69` can throw during per-child signal/wait cleanup. In particular, a child exiting between `poll()` and `killpg()` yields ProcessLookupError. That aborts the rest of finally before Docker cleanup, gate restoration and terminal receipt publication. The post-SIGKILL `wait()` is also unbounded. Catch/record errors per child, bound waits, continue every cleanup stage and put terminal receipt publication in the outermost cleanup finally. No change to the original240-second business deadline is needed.

The actual scenario scope is correct: frozen spec/control/CLI fixture bytes exactly match the original source, preserving one positive and nine refusal operations, all state assertions, the240000ms deadline and zero retries. All declared request hashes match. The saved database URL matches the migrated task database and recorded loopback PG allocation. The runner inspects live PG ownership before writes and binds the spawned API through health PID/commit. Exact Docker policy and synthetic task accounts are preserved. Fallback resource removal retains cleanupFAIL.

Small evidence follow-ups can accompany the repair: retain initial/final gate epoch and verify restored booleans; pin the actual API output hash (recorded independently in this review JSON), since its current build receipt pins source/compiler/log but not the executable output.

## Files found / related specs

- Native `cli-review-request.json`, `run_cli.py` — exact request and full journey orchestration/cleanup.
- Native `cli_guard.py`, `cleanup.py`, `freeze.py`, `timing_probe.py` — request-pinned imports; guarded Docker boundary unchanged from the accepted timing subset.
- Native `api-build01.json`, `relay-build.json`, `source01.json` — build/source records, all declared hashes verified.
- Frozen `e2e/collaboration-installed-clients.spec.ts` and control/CLI fixtures — original complete scenario.
- `.trellis/spec/server/verification-environment.md` and `.trellis/spec/web/frontend/e2e-run-environment.md` — exact identities, owned cleanup and truthful final receipt.

## Caveats / Not Found

- No adapters, Docker/SQL/services/tests or Git operations executed; only research files written.
- Timing-only acceptance remains valid under its own exact request. Packaged Main is separate and not cleared here.
- No external references needed. Live ownership and process cleanup must still be observed by the actual admitted run; this review is not execution evidence.
