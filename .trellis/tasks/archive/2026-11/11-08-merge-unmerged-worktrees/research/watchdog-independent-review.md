# Independent review: stop inactivity monitoring before terminal transcript delivery

Reviewed: 2026-11-08. Scratch checkout: `/Volumes/artisan/code/2026/mc-merge-watchdog-repair-20261108`, based on `7038918a7793d722f47124a19b976a446b3c7f3e`.

**Final verdict: APPROVE for the two-file patch and final hashes in the follow-up below. The initial P3 test cleanup finding is resolved; its original analysis is retained as history.** The new complete fixed-SHA gate remains mandatory. No code, test, process, database or frozen integration mutation was performed by this reviewer.

## Findings (fixed)

None in this read-only dispatch.

## Findings (not fixed)

### P3 — entry notification can block HTTP cleanup on an early failing test path

`server/internal/daemon/daemon_test.go`, new `TestExecuteAndDrain_IdleWatchdog_StopsBeforeFinalTranscriptFlush`: the HTTP handler sends `entered <- struct{}{}` for every request, but the test consumes the channel only once. The capacity-one channel is sufficient for the usual single final flush, but it can block teardown when an earlier test deadline/failure occurs before that receive: a ticker flush fills the channel, a remaining batch is flushed after cleanup releases the first HTTP request, and the second handler blocks on the still-full entry notification. `releaseFlush` releases only the following wait, not this send; `srv.Close` can consequently wait indefinitely for the blocked handler.

This is a test failure-path robustness finding, not an observed failure of the completed validation runs. The notification represents “at least one request entered”, so make it nonblocking or close a first-entry channel once. Preserve the real HTTP barrier, late-message and terminal-result assertions. Do not edit while a validation command still owns the source snapshot; rerun the narrow regression after the change. Main and implementer were informed through the main session.

## Production lifecycle review

- Monitoring now has a child context distinct from `agentCtx` and `drainCtx`. Canceling that child does not cancel provider or transcript work. The test's unbuffered late messages would be lost by an early `agentCancel` shortcut; the retained mutation demonstrates that distinction.
- On a terminal result, monitoring is canceled and joined before final process accounting, transcript wait, or `idleWatchdogFired` classification. The goroutine closes its done channel on return. Calling the stop function in the selected terminal branch and again through defer is safe: context cancellation is idempotent and reads from the closed done channel return immediately.
- On upstream/drain cancellation, monitoring is likewise joined before terminal classification. Watchdog-triggered `agentCancel` does not wait for this join, so it can return and close watchdogDone; no self-join or context-lock cycle was introduced.
- The helper checks `Err()` after selecting a ready tick, so an already-observed cancellation wins over a simultaneously ready timer. An already executing timeout evaluation may still finish while stop is joining; that is the existing timeout/terminal arbitration. The join ensures no later watchdog mutation occurs after stop returns or after result classification. A previously fired timeout is not erased merely because the provider subsequently returns `aborted` or another result.
- The ordinary and in-flight-tool budgets, queue check, activity updates, threshold/error construction, global disable and per-run shortening remain unchanged. There is no timeout extension, retry, skip or broad success override.
- `waitForDrain`, final/tick flush ordering and bounded drain cancellation remain intact. `finishProcess` still uses actual ProcessExited evidence and runs before waiting for the transcript. Knowledge Stop/Seal and failure evidence retain their previous branches; the new local stop changes only watchdog lifetime.
- The collaboration host still calls the same shared helper with its existing execution context, activity counters and cancellation callback. The only shared change exits an already-canceled monitor instead of evaluating a ready tick. It does not change host budget selection, recording, receipt, process ownership or completion handling. This patch does not claim to redesign the host's separate result/recording lifecycle.

## Regression and retained evidence

Read `research/watchdog-fix.md`, the original diagnosis, the actual patch and full JSON output of the narrow evidence files under `/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification/watchdog-repair-01`.

- Original-source red run exits1: both completed/failed subcases are wrongly returned as `idle_watchdog` after the controlled HTTP delay.
- Fixed green run exits0: three repetitions, six subcases pass (nine named pass events including the three parent test events).
- Removing only the terminal stop exits1 and fails both subcases again. The test detects the new production line's omission.
- Early provider-context cancellation exits1 and fails both subcases on missing remaining transcript. The test rejects canceling the drain as a shortcut.
- Focused race run exits0: 42 named tests/subtests pass, no failures/skips or DATA RACE marker. It includes actual idle/no-message/stuck-tool/post-tool-silence timeouts, disabled/per-run budgets, transcript flush and cancellation behavior.
- The original full-gate failing test repeats100 times with race: 100 pass, no failure/skip or DATA RACE marker.
- `daemon-package-race-state.json` subsequently records exit0 at16:05:58 for the same source hashes. This is the implementer's execution, not a second suite launched by the reviewer.

The new test proves the provider result was received before later transcript messages, holds the actual HTTP response for four idle windows, rejects returning before delivery, and checks successful and failed status/error/output/session/process evidence, tool count, message contents/order and sequence. Its clocks still use the repository's real-time/channel pattern; the barrier gives the lifecycle ordering, while normal scheduler timing is not asserted as an exact event timestamp.

The original full-check-05 log does not prove that its specific 0.30-second failure had this exact interleaving. The controlled red establishes a real completion-phase bug that the original assertion can observe; it is appropriate to retain that distinction in closure claims.

## Exact reviewed input

```json
{
  "server/internal/daemon/daemon.go": "27a914766f0cfebf0be1f66fa6a089a27506cf442fdf597fa0d1f4740fdc7bd9",
  "server/internal/daemon/daemon_test.go": "5dbf3586b2db68600658c54fe117af3e6304244c513b2062eb963c3fa01aa220"
}
```

The hashes match green, focused race, original100-repeat and full daemon race state files; each mutation carries a different production hash and retains its failed output. A cleanup-notification adjustment will change only the test hash and needs a follow-up entry below.

## Verification limits and spec impact

- Reviewer: source/diff and archived evidence inspection only; no tests/builds/lint/vet were launched during the implementer's validation.
- Implementer lint/format/vet: final evidence remains main-owned; read the eventual result before committing.
- Full gate: still required on the final clean integration SHA after the failed fifth run ends and this increment is reviewed/applied.
- No public configuration/API/dependency changes need a new module contract. The lifecycle comment and named regression document the boundary; the existing task diagnosis records why final delivery cannot be counted as provider inactivity.


## Follow-up: cleanup fixed, final patch approved

Reviewed after the final package race and vet completed on 2026-11-08 at16:09.

**APPROVE. No remaining concrete finding in this two-file repair.**

The HTTP handler now closes an unbuffered `entered` channel through `sync.Once`; every handler can reach the release barrier without waiting for the test to consume a notification. Existing cleanup still cancels the caller, releases the HTTP barrier and then closes the server in LIFO order. The signal means first request entry, exactly as the test consumes it. No transcript or terminal assertion was weakened. Production source is unchanged from the initially reviewed fix.

Independently inspected final source hashes and parsed the retained complete JSON logs:

- `reviewed-focused-race`: exit0; three repetitions, 90 top-level /126 named tests and subtests pass,0 fail/skip/race markers.
- `reviewed-daemon-package-race`: exit0;900 top-level /1723 named tests and subtests pass,2 pre-existing platform skips,0 fails/race markers. Skips are `TestCollaborationBatchPollerSlotsAndRunnerLinux` (real Linux containment) and `TestIsDriveRoot` (Windows path semantics).
- `reviewed-vet`: exit0 with empty log. Commands run through the repository ambient Agent CLI guard; real-Agent opt-in remains0.

Final approved hashes:

```json
{
  "server/internal/daemon/daemon.go": "27a914766f0cfebf0be1f66fa6a089a27506cf442fdf597fa0d1f4740fdc7bd9",
  "server/internal/daemon/daemon_test.go": "7c377d4a6e3e808fb806b2f0a3e8004d74b2b6404858ad2f5b227c9aa4825c48"
}
```

This approval covers the reviewed repair and its retained focused/full-daemon validation. It does not pass full-check-05, certify a different later SHA or authorize landing before the new complete repository gate. Main owns incorporation after the frozen failed run ends, the final static/format checks, complete-gate execution and original-root preservation proof.
