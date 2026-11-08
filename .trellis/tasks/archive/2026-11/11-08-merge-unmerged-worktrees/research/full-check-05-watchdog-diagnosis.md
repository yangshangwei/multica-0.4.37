# Research: full-check-05 idle watchdog failure

- Query: Diagnose `TestExecuteAndDrain_IdleWatchdog_DoesNotFireDuringInFlightToolCall` without changing or testing the candidate while its complete gate runs.
- Scope: internal; integration checkout `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`, supplied candidate `7038918a7793d722f47124a19b976a446b3c7f3e`.
- Date: 2026-11-08.
- Method: read-only source/log inspection. No tests, production edits, database operations, or Git operations performed.

## Findings

### Observed failure and its limit

The retained `verification/full-check-05/make-check.log` under `/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108` reports at lines8437–8440:

```text
--- FAIL: TestExecuteAndDrain_IdleWatchdog_DoesNotFireDuringInFlightToolCall (0.30s)
    daemon_test.go:3725: watchdog must not fire while a tool_use is in flight, got status="idle_watchdog" (err="agent produced no new messages for 50ms and message queue was empty; force-stopped by idle watchdog")
FAIL
FAIL github.com/multica-ai/multica/server/internal/daemon 86.392s
```

The test name/error assumes the watchdog fired while the tool was in flight. The assertion checks the final status only; it does not establish that timing. The log contains no per-test watchdog timestamp or `tool_in_flight` detail. It therefore cannot prove which scheduling interval produced the failure.

### Concrete lifecycle gap: completed backend remains subject to idle timeout during final delivery

The source exposes a reproducible candidate mechanism rather than a reason to enlarge a test timeout:

1. `longToolCallBackend` queues `tool_use`, waits200ms, then queues `tool_result` and text, closes Messages, and provides a completed Result (`server/internal/daemon/daemon_test.go:3662`).
2. The test uses a50ms ordinary idle window and a zero tool window (`daemon_test.go:3702`). An in-flight tool is intentionally exempt with a zero tool window (`daemon.go:8992`).
3. The drain records new activity on message receipt (`daemon.go:8657`) and decrements the in-flight count for `tool_result` (`daemon.go:8723`). Once final messages are consumed, the ordinary50ms window applies again.
4. Receiving `session.Result` does not stop the watchdog. `executeAndDrain` first invokes `waitForDrain()` and only afterwards checks `idleWatchdogFired` and potentially replaces the backend status (`daemon.go:8868`). Its `agentCancel` is deferred until the whole function returns (`daemon.go:8493`).
5. `waitForDrain` waits for the final transcript HTTP POST (`daemon.go:8812`; final `flush()` at8798). Each POST uses `context.Background()` with a5second timeout (`daemon.go:8608`), deliberately independent from provider cancellation.
6. The watchdog sees no pending message and zero in-flight tools during this HTTP completion interval. If delivery/scheduling lasts over50ms, it can mark an already completed backend `idle_watchdog` (`daemon.go:8978–9023`). This is consistent with the reported0.30s duration (200ms simulated tool plus completion work), but the original log alone cannot assign that exact schedule.

The intent is to detect backend inactivity. Transcript delivery is a separate completion phase; stopping monitoring at provider termination must not skip or cancel pending transcript delivery.

### Minimal recommended repair, pending red reproduction

Give the watchdog its own stoppable lifecycle in `executeAndDrain`. Stop/join it after receiving the terminal provider result and before `waitForDrain`, while preserving a watchdog that already fired and preserving the final transcript drain. Keep teardown idempotent for other return paths. A separate context derived from `agentCtx` can stop monitoring without cancelling the provider/drain context. Joining is useful to prevent a pending ticker selection from recording a new watchdog result after monitoring was considered stopped; verify the timer path respects termination before firing.

Do **not** simply invoke `agentCancel()` before `waitForDrain`: that also cancels `drainCtx`, whose select can abandon buffered final messages. Do not remove `waitForDrain`, ignore all watchdog flags for completed results, disable the watchdog in the test, extend thresholds, or add retries. Existing true-timeout cases must remain failures.

`runIdleWatchdog` is also called by the collaboration host (`server/internal/daemon/collaboration.go:395–408`). Prefer a local lifecycle repair unless reproduction proves the shared helper itself requires changes. Any helper change additionally needs host watchdog coverage.

### Reproduction and verification proposed after full-check-05 has ended

1. Preserve the completed failed run/log hash. Add a named regression using existing `sessionBackend` (`daemon_test.go:2549`) and a channel-controlled transcript endpoint modeled on `newTranscriptRecorder` (`daemon_test.go:2469`).
2. Deliver the terminal result, then hold the actual final `/messages` response past the idle threshold. Use channels to establish that the result handoff and HTTP entry occurred. Confirm `executeAndDrain` has not returned before release. Release the endpoint and assert the provider result, output, tool count, and complete ordered transcript are preserved. The new regression should fail on the current source as `idle_watchdog`.
3. Test both a successful and a failed terminal provider result so the fix preserves disposition rather than blindly declaring success. Include buffered tail messages to reject a naive early `agentCancel` fix.
4. Apply the narrow lifecycle repair and rerun the new regression plus all `TestExecuteAndDrain` and `TestIdleWatchdogTickInterval` cases under race. Repeat the original failing case enough times to exercise scheduling; retain counts/commands and source identity. These include true initial silence, per-run shortening, global disable, stuck in-flight tool, and post-tool silence coverage.
5. A mutation removing/bypassing the new monitoring stop must make the new regression fail again; preserve this red evidence separately. The ordered transcript assertions should reject an early cancel/early return mutation.
6. Run the complete daemon package with its existing agent CLI guard and then a new fixed-SHA complete `make check`. Targeted success cannot substitute for that gate.

### Existing synchronization and time patterns

- `sessionBackend` exposes message/result channels; the cancellation transcript test uses an unbuffered send to prove the drain consumed a message (`daemon_test.go:2557–2604`). Reuse this existing seam instead of adding production test hooks.
- `transcriptRecorder` synchronizes its snapshot with a mutex (`daemon_test.go:2457`). Its assertions already pin flush-before-terminal and continued sequence numbering (`daemon_test.go:2491–2545`).
- Daemon tests use actual `time.After` and channels; no `testing/synctest` usage or injected daemon clock was found across `server/**/*.go`.
- Lark HTTP/chunk tests have a local manually advanced `fakeClock` (`server/internal/integrations/lark/http_client_test.go:1501`), but the daemon watchdog does not expose that seam. Importing that unrelated abstraction is unnecessary for the proposed channel-controlled HTTP regression.
- A tiny real-time window still carries scheduling uncertainty. Establish phase ordering with channels and include bounded cleanup. Fake-time testing would require replacing network I/O with an in-process RoundTripper and checking current standard-library behavior; it is not an already established daemon pattern.

## Files found

- `server/internal/daemon/daemon.go` — provider lifecycle, transcript drain, watchdog startup and terminal classification.
- `server/internal/daemon/daemon_test.go` — observed failure, backend fixtures, transcript recorder and channel-controlled session fixture.
- `server/internal/daemon/client.go:520` — real task-message reporting boundary used by the regression.
- `server/internal/daemon/collaboration.go:395` — second shared-watchdog consumer; avoid widening its behavior without corresponding evidence.
- `server/internal/daemon/collaborationrun/host_recording.go:117` — host watchdog/recording lifecycle; separate scope unless helper changes demand review.
- `server/go.mod` — Go1.26.6; no dependency addition required.

## Related specs and references

- Root `CLAUDE.md`: narrow changes, checked Go behavior, tests at canonical backend layer, default tests must not resolve installed Agent CLIs.
- Root `.trellis/spec/server/index.md`: relevant server contract index.
- Root `.trellis/spec/server/database-test-isolation.md`: preserve failed full checks, explicit owned test databases, no additional concurrent test environments during a complete gate.
- Root `.trellis/spec/server/verification-environment.md`: frozen candidate identity and complete-run evidence; do not aggregate separate partial passes into complete acceptance.
- Task `prd.md`: R6 requires targeted verification plus a new complete gate; retain original failures.
- External references: none needed for this source-based diagnosis; no unfamiliar external API proposed.

## Caveats / Not Found

- This is read-only diagnosis, not a reproduced or fixed bug. The completion-phase watchdog gap is visible in code; the original failure's exact interleaving still needs instrumented/red confirmation.
- Other theoretically possible interleavings exist: the watchdog reads separate atomic activity/count snapshots, and a drain goroutine may be preempted between receiving a message and stamping activity. Do not claim those caused this failure without evidence, or broaden the patch preemptively.
- The actual full-check05 result remains owned by the main session. Do not mutate or restart its candidate until it has finished and its evidence is retained.
