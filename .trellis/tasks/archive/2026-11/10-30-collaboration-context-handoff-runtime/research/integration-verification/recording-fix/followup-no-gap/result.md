# Follow-up: local checkpoint rejection preserves the gap

Fixed the final review's checkpoint P1 in the isolated integration worktree. The sender now distinguishes local staging failure from HTTP uncertainty with a sender-owned latch. After a batch cannot be persisted, later queued messages are drained without staging or rebasing. Already persisted checkpoints retain their exact original payload and remain recoverable. Existing closed/incomplete metadata leaves the missing tail unknown.

Only two source files changed in this follow-up:

| File | Final SHA-256 |
| --- | --- |
| `server/internal/daemon/collaborationrun/host_checkpoints.go` | `c620882037887fd67d0a6323b8155718de0c3a4d885d5bd6506914301d3d8eec` |
| `server/internal/daemon/collaborationrun/host_checkpoint_gap_test.go` | `8166927d5dae4c5156660fd4078ae6d2722a7c556d6e7da223f6004bd9fb2463` |

`sender-fix.diff` contains the exact production change. `scope-audit.json` proves `recoverHost`, the ledger, original recovery tests and frozen combined fixtures are unchanged. There is no new protocol dependency, wire/retention change, execution retry, dependency, commit or task-state update.

The canonical test uses the real `hostRecording.message`, sender, ledger and original checkpoint/seal/termination/proof recovery, with local HTTP fixtures and no Provider execution. It covers:

- Invalid first batch: later ordinals never become offset zero and no body is uploaded.
- Accepted prefix, invalid middle, later valid messages: only the accepted prefix survives; the suffix never appears in a checkpoint or final outcome.
- Uncertain prefix ACK, invalid middle: the saved prefix replays exactly; later messages are still discarded.
- Uncertain prefix ACK with a valid suffix: all queued valid data remains staged and recoverable, proving that network uncertainty does not trigger the local-failure discard rule.

All cases check bounded sender drain, original Stop/proof ACK, metadata sealing, honest observed-tail facts and stable original message ordinals.

## Verification

The original independent failure is retained in `review-probe.json`. Each numbered command has exact argv, HEAD, source/dirty-diff hashes, platform, exit code and source-change audit in `.json`, plus full output in `.log`.

| Evidence | Result |
| --- | --- |
| `01-red` | Exit 1: all three invalid-batch cases reproduced a rebased/post-gap upload; valid suffix after HTTP uncertainty passed. |
| `02-focused-green` | Exit 0: new regression, existing checkpoint suites and frozen combined handoff/checkpoint cases passed; 4.780 s. |
| `03-race` | Exit 0: 20 top-level tests passed, no skips; 18.824 s. Includes the backlog deadline, lost checkpoint/seal ACKs, blocked-recording proof priority and existing ledger/outcome/proof recovery. |
| `04-daemon-package` | Exit 0: all 95 top-level daemon collaborationrun tests passed, no skips; 27.462 s. |
| `05-vet` | Exit 0: package `go vet`. |
| `06-format` | Exit 0, empty output: both edited files are gofmt-clean. |
| `07-diff-check` | Exit 0, empty output: daemon collaborationrun diff check. |

Final commands:

```sh
go -C server test -race ./internal/daemon/collaborationrun -run 'TestHost(HandoffCheckpoint|Checkpoint|Recording|RunnerPending|Ledger|SupervisorLateCallback)' -count=1 -timeout 3m -v
go -C server test ./internal/daemon/collaborationrun -count=1 -timeout 5m -v
go -C server vet ./internal/daemon/collaborationrun
```

Every captured run had zero source changes during execution. No database, installed agent CLI or authenticated Provider account was used. Remaining integration/review gates belong to the coordinator; this result fixes the daemon P1 only and does not claim model or product-wide acceptance.
