# Recording recovery continuation — 2026-11-01

Existing task is in_progress with approved PRD/design/implementation plan. No new Trellis task is created.

## Change boundary

The current host recovery path flushes launch observations and the original outcome/proof, but never calls pending-checkpoint replay or metadata-only recording seal. The ledger already keeps pending recording recovery after proof acknowledgement. The missing integration must not delay lifecycle Stop or starve original proof when telemetry remains unavailable.

Ownership is isolated in `mc-runtime-recording-recovery-20261101`. Product writes are limited to `server/internal/daemon/collaborationrun/host_runner.go`, `host_checkpoints.go`, `host_checkpoint_ledger.go`, and `ledger.go` only if needed, plus focused recovery tests. Add a failing end-to-end recovery regression first, then repair the smallest root cause. Reuse existing sender and proof lifecycle. Checkpoint requests retain exact original identity, attempt, stream, sequence, payload and issued business token; seal uses only original termination identity and bounded metadata. Restart must never Claim, Permit or Execute, and must not resubmit accepted proof.

Validate a persistent HTTP failure and a blocked recording upload with independent Stop/proof completion; reopen the on-disk ledger using a fresh Runner and assert exact request replay and eventual drain. Bound total recording recovery work, preserve acknowledged proof priority, and show missing telemetry remains pending rather than falsely complete. Cover checkpoint ACK loss and recording-seal ACK loss without duplicate content or rewritten payloads.

A separate read-only Trellis review audits original payload/credential boundaries and proof priority. Run focused regressions with race detection, the full daemon/protocol/CLI suites, Go vet/build and applicable repository static checks. Record exact commands, source hashes, exits and skipped scopes in this directory. Other live worktrees and original real-model experiment slots remain untouched. Integrate only a reviewed delta after checking the shared-file baseline; never replace whole shared files or adopt unrelated edits.

## Verified scope additions

The independent reviewer reproduced invalid recording-seal retention responses being accepted. The coordinator completed the paused client fix in `client_host.go` and `client_recording_ack_test.go`, reusing the existing protocol validator. Negative/out-of-range/contradictory watermarks and completed-with-unknown-tail are rejected. A completed sender observation with a partially retained known tail remains valid: `SealRuntimeRecording` and `collaborationRecordingState.retention` do not make completed synonymous with all body bytes retained. This preserves existing API semantics and supersedes the initial review assumption about unequal completed bounds. A Go-overlay integration regression verifies that the old client accepts a malformed seal response while the fixed client retains the original pending seal and accepted proof.

Full TS verification exposed two existing execution-panel tests whose sequential fetch queues were consumed by the new independent handoff GET. `execution-panel.test.tsx` now mocks that child, whose behavior has its own canonical component suite; all record pagination/retry/access-loss assertions remain intact. No UI behavior changes are required.

Both original implement/check children hit a model token-rate limit after saving their changes and red logs. A bounded explorer also failed due to an unavailable model route. The coordinator took over the remaining isolated implementation and validation; final independent review will be retried with a fresh, smaller context.
