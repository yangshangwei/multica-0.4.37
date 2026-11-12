# Recording recovery verification

The daemon recovery gap is fixed in `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`. No commit or global task-state update was made.

## Changes

- `server/internal/daemon/collaborationrun/host_runner.go`: only `recoverHost` changed. An already sealed proof keeps first priority. Original checkpoint replay runs after Stop and launch termination recovery; its errors cannot bypass outcome/proof. Metadata sealing runs afterward using the same bounded telemetry context.
- `server/internal/daemon/collaborationrun/host_checkpoints.go`: at most one second for the whole telemetry backlog, shortened when the caller's deadline requires it. Cancellation also stops local rejected-backlog processing. Pending checkpoint bodies retain their original identity, key, payload and business token. Unacknowledged seals replay exactly before one coalesced final ACK-watermark update, using the shared protocol validator.
- `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go`: persist successful sender drain separately from the retained prefix. A restart without that fact leaves the tail unknown. Validate a seal against an actually issued attempt/nonce; do not rewrite a settled checkpoint acknowledgement. Keep pending or outdated seal metadata eligible for recovery after proof ACK.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go`: five regressions cover a slow backlog/proof priority, original seal ACK replay and monotonic update, unknown crash tail, unissued seal nonce, and cancellation during rejected-backlog recovery.

The implementation reuses existing checkpoint/seal endpoints, ledger records, issued capabilities and the shared `ValidateCollaborationRecordingSealAdvance` helper. It adds no execution retry, claim, snapshot fetch, dependency or new business writer.

## Evidence

Every command has its full output in the matching `.log` and its exact argv, working directory, platform, source HEAD, dirty-diff SHA-256, per-source SHA-256, exit code and concurrent-source-change audit in `.json`. Checks used test-owned fake executables, local HTTP and local ledger fixtures; no database, installed Provider CLI or authenticated account was used.

| Evidence | Result |
| --- | --- |
| `01-red` | Exit 1: missing seals after `binding_refused` and `fallback_snapshot_drift`; no original checkpoint replay after restart. Blocked-recording proof regression passed. |
| `02-additional-red` | Exit 1: backlog processing absent, missing seal advance, invented observed crash tail and accepted unissued seal nonce. |
| `03-focused` | Exit 0: combined handoff/checkpoint and checkpoint/recording suites passed after wiring. |
| `04-cancelled-backlog-red` | Exit 1: expired recovery continued discarding unresolved checkpoint payloads. |
| `05-recording-race` | Exit 0: 19 top-level tests passed, no skips; combined handoff/checkpoint, recording, host ledger, pending outcome/proof and late-callback recovery under `-race`. |
| `06-vet` | Exit 0: `go -C server vet ./internal/daemon/collaborationrun`. |
| `07-daemon-package` | Exit 0: all 91 top-level daemon collaborationrun tests passed, no skips, 22.303 seconds. |
| `08-format` | Exit 0, empty output: `gofmt -l` on all four edited Go files. |
| `09-diff-check` | Exit 0, empty output: `git diff --check -- server/internal/daemon/collaborationrun`. |

Final test commands:

```sh
go -C server test -race ./internal/daemon/collaborationrun -run 'TestHost(HandoffCheckpoint|Checkpoint|Recording|RunnerPending|Ledger|SupervisorLateCallback)' -count=1 -timeout 3m -v
go -C server test ./internal/daemon/collaborationrun -count=1 -timeout 5m -v
```

All source files remained unchanged during these final test and vet commands. `scope-audit.json` verifies both frozen combined suites and the existing checkpoint suite are byte-for-byte unchanged from the red baseline. `host-runner-owned-hunks.diff` contains this worker's exact shared-file hunks; reconstructing the baseline from HEAD plus `01-red.diff` verified that every byte outside `recoverHost` is unchanged.

## Remaining integration scope

Service/handler authorization, SQL record projections, full-repository checks and independent review belong to the coordinator/server lane. This worker's tests prove daemon delivery/recovery, not model understanding, real Provider reliability, database ordering or Web/Desktop acceptance. Unavailable telemetry intentionally remains pending without reopening execution or withholding an accepted proof. Previously unknown or partial recording facts are not upgraded into fabricated completeness.
