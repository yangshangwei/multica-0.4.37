# Late runtime reconciliation — verified delta

Completed in `/Volumes/artisan/code/2026/mc-runtime-integration-20261031` at HEAD `e3620721efacb820b89709c8db9d326d9cf2b00b`. Main was read-only; all nine compared main files still match their captured hashes. No commits, task/runtime state transitions, DB tests or authenticated Provider runs were performed.

## Result

The tested runtime already implements or strengthens every valid late production change. Keep `client_host.go`, `host_checkpoint_ledger.go`, `host_checkpoints.go`, `host_runner.go` and the Views execution-panel test unchanged. The reconciliation changes exactly four files:

- `server/internal/daemon/collaborationrun/client_recording_ack_test.go`: new compact HTTP tests for exact checkpoint ACK bounds, shared retention decoding, response binding, zero/unknown/missing-tail facts and later retained facts on an old seal ACK.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go`: append the unique late explicit-Stop-during-blocked-upload and malformed-seal-after-restart regressions, using existing fixtures. Every original test remains byte-for-byte intact.
- `.trellis/spec/server/collaboration-runtime-recovery.md`: preserve existing scan/proof contracts and merge late recording guidance with the actual one-second total telemetry budget, proof priority, strict shared retention decoder, unknown-tail drain marker and monotonic seal advancement. Parent explicitly expanded ownership for this file.
- `.trellis/spec/server/index.md`: retain actual-candidate coverage and startup-handoff entry while adding the late recording-recovery description.

The full missing/null/count retention matrix stays in the protocol suite. The existing combined checkpoint/handoff tests, budget tests, wire tests and local-gap tests are unchanged. All other source files in the 7,835-file final inventory are unchanged from this reconciliation's start.

## Superseded alternatives

- The late raw embedded-retention wrapper would promote `CollaborationRecordingRetention.UnmarshalJSON`, bypass the wrapper's own raw fields and reject valid receipts. The shared decoder already validates the exact late field/count cases; no production ACK patch is needed.
- The late two-second replay / ten-second shared report window predates the tested one-second total telemetry budget and metadata-after-proof ordering.
- Removing `Drained`, the `stagingFailed` latch or ledger nonce/monotonic ACK guards would weaken unknown-tail, no-gap and exact-replay guarantees.
- Late `host_runner.go` predates the integrated frozen launch snapshot/hash, fresh fallback source identity and Codex input-binding work.
- A second recovery call is not required for a metadata advance once the original seal ACK is durably stored and the same total budget permits the next send.
- Mocking the entire handoff component would remove real wiring from execution-panel tests. The existing endpoint-aware fetch fixture already isolates record pagination from handoff/issue reads.

Per-file reasoning and the mapping from omitted duplicate late tests to retained canonical coverage are in [decisions.json](./decisions.json). Original late bytes, the initial source snapshot and their metadata remain preserved here; nothing was deleted or overwritten in main.

## Verification

All commands and logs are stored beside this report, with source HEAD, dirty-diff hash, complete source inventory, platform, exact argv, duration and exit code. This run used macOS arm64, HTTP fakes and the existing test-owned executable.

| Check | Result |
| --- | --- |
| Focused daemon race | 5 top-level tests + 13 subtests passed |
| Full protocol race | 31 top-level tests + 267 subtests passed |
| Final full daemon race | 101 top-level tests + 217 subtests passed; final source inventory matches |
| Execution-panel Vitest | 9/9 passed; file unchanged |
| `go vet ./pkg/protocol ./internal/daemon/collaborationrun` | Passed |
| `gofmt -l` on changed Go tests | Empty output |
| Targeted `git diff --check` | Passed |
| Source/archive inventory | Only the four listed files changed; frozen/original tests and main bytes preserved |

There were no failing or skipped Go cases. The full protocol+daemon run passed first; then the unsealed-response test was strengthened to isolate the client's sealed-state check, and the entire daemon race suite passed again on those final bytes. Protocol source remained identical. No TypeScript source or test changed, so workspace lint/typecheck were not repeated; the affected retained Views test passed.

## Handoff

[after.json](./after.json) contains exact before/after hashes and the final complete source inventory. [reviewed-delta.diff](./reviewed-delta.diff) and [reviewed-delta.tar.gz](./reviewed-delta.tar.gz) contain only this reconciliation delta. [checks.json](./checks.json) and [test-counts.json](./test-counts.json) index validation evidence.

The parent should include the new client ACK test and the recovery spec in the final apply ownership list. Copying the integrated runtime to main, final whole-change review and committing remain parent responsibilities. No scoped runtime implementation gap remains; this pass does not add DB, real-model or native-platform acceptance evidence.
