# Watchdog terminal-drain lifecycle repair

## Boundary and observed gap

Scratch checkout: `/Volumes/artisan/code/2026/mc-merge-watchdog-repair-20261108`, based on `7038918a7793d722f47124a19b976a446b3c7f3e`. Only `server/internal/daemon/daemon.go` and `daemon_test.go` change. The integration checkout remains frozen during full-check-05. No production process, database, API environment, browser, or real Agent is used by this repair's unit tests.

A provider terminal result ends the backend inactivity interval. Previously, `executeAndDrain` kept its idle watchdog active while waiting for the final transcript HTTP response. An empty queue after tool completion restored the ordinary idle budget. Slow delivery could therefore replace either `completed` or `failed` with `idle_watchdog` even though the backend had already returned.

The original full-check-05 failure lacks an exact interleaving trace; this repair does not claim the old log proves that schedule. The new controlled regression directly reproduces the lifecycle gap on the original source.

## Repair

A dedicated watchdog context is cancelled and its goroutine joined when the terminal result is received, before waiting for the transcript drain. The drain/context used by the provider remain separate, so queued tail messages still finish. The cancellation terminal also joins monitoring before classifying the outcome. A watchdog that already fired remains authoritative. The shared helper checks cancellation before processing a ready ticker event, avoiding a stale tick when both cancellation and the timer are ready.

No budget, retry count, skip, provider behavior, dependency or persisted state changes. The fix stays inside the existing lifecycle rather than adding an alternate executor or testing hook.

## Regression contract

`TestExecuteAndDrain_IdleWatchdog_StopsBeforeFinalTranscriptFlush` covers completed and failed results. An unbuffered result handoff establishes provider termination; three final messages then pass through the real drain and actual httptest HTTP endpoint. The final response remains blocked for four idle windows. The caller must wait for delivery, retain provider status/error/output/session/process evidence and one tool count, and preserve all three messages in sequence.

The test rejects an early `agentCancel()` shortcut: that cancels the shared drain context and loses the remaining transcript.

## Retained evidence

Directory: `/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification/watchdog-repair-01`.

- `red.log` / `red-state.json`: original production source; both terminal-result cases fail as `idle_watchdog` (exit 1).
- `green.log` / `green-state.json`: fixed source; three repetitions, six subcases pass (exit 0).
- `mutation-no-terminal-stop.log`: removing only the result-branch monitoring stop makes both cases fail again (exit 1).
- `mutation-early-agent-cancel.log`: adding early agent cancellation makes both cases fail on missing remaining transcript (exit 1).
- Both mutations were restored to the fixed source immediately after execution; retained source hashes identify every run.

Every test command uses `scripts/go-test-with-agent-cli-guard.sh`, removes inherited DATABASE/PG/POSTGRES/MULTICA/REDIS settings, and sets `MULTICA_RUN_REAL_AGENT_SMOKE=0`. Only the daemon package is executed; no database TestMain is reached. JSON test logs and command/exit/source records remain outside Git.

Initial fixed-source verification:

- Focused race (`TestExecuteAndDrain*` and `TestIdleWatchdogTickInterval*`): 30 top-level / 42 including subtests passed, no failures/skips, exit 0.
- Original full-check-05 failing test: 100/100 race repetitions passed, exit 0.
- Full daemon race: 900 top-level / 1723 including subtests passed, 2 platform skips (`TestCollaborationBatchPollerSlotsAndRunnerLinux`, `TestIsDriveRoot`), no failures, exit 0. No named collaboration/host watchdog-specific tests exist; full daemon covers the existing host/collaboration consumers.

Independent review then found a failure-cleanup risk in the new test: a blocking first-entry notification could stall a later flush if the test exited before consuming the earlier notification. After the initial full package run completed, notification changed to `sync.Once` closing a channel. Production source is unchanged. The reviewed test hash receives separate focused/race validation; prior results remain identified by their original source hashes.

Final reviewed-source verification (all exit 0):

- `reviewed-focused-race`: three repetitions, 90 top-level / 126 including subtests passed, zero failures/skips.
- `reviewed-daemon-package-race`: 900 top-level / 1723 including subtests passed, the same two platform skips, zero failures. Entire log scanned; no race report.
- `reviewed-vet`: `go -C server vet ./internal/daemon` passed.
- `gofmt -l` empty and `git diff --check` passed. Scratch status contains only the two owned Go files.
- Final source SHA256: daemon.go `27a914766f0cfebf0be1f66fa6a089a27506cf442fdf597fa0d1f4740fdc7bd9`; daemon_test.go `7c377d4a6e3e808fb806b2f0a3e8004d74b2b6404858ad2f5b227c9aa4825c48`. Rechecked after verification: unchanged.
- `repair-reviewed.patch` contains the exact final two-file patch; `summary.json` retains commands, counts, failures/skips, source and log hashes. Earlier `repair.patch` is the pre-review test variant and remains historical evidence.

No implementation commit was made. Main must await full-check-05 completion before applying this patch to the integration checkout, finish independent review, commit with the Lore protocol, and run a new complete fixed-SHA `make check`. These unit results cannot substitute for that gate. The original full-check-05 schedule remains unproven, while the independently reproduced completion/drain gap is fixed and regression protected.
