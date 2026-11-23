# S1 hold commit fault targeting — 2026-10-21

Status: bounded test-harness repair verified; coordinator review and commit remain separate. Existing task: `10-20-collaboration-gap-recovery`; no task, plan, product, migration, environment or gate changes were made by this lane.

## Root cause

The unmodified `TestCollaborationPromoteHoldUnknownCommitIsNotRetried` failed independently on the dedicated `multica_gap_service_s6_hold` database. `ListRuntimeRuns` now commits its short wake-scan cursor transaction before scanning waiting targets and entering the promotion transaction. The old `queueFaultStarter.lostCommit` injected a PostgreSQL `40001` error after the first commit, so it lost the cursor reply rather than the discovered hold reply. Its `begins == 1` assertion still passed, but the next poll correctly created the first hold and audit, changing the saved business snapshot. This was test-fixture drift, not proof that the product retried an uncertain hold commit.

The adjacent `TestCollaborationPromoteHoldRequiredWritesRollback/commit` had the same selection problem: its untargeted pre-commit failure stopped the cursor transaction, leaving the intended hold/audit rollback unexercised. The coordinator explicitly expanded this lane to repair that coverage and minimally extend the existing runtime-scan fault helper.

## Changed files and preserved assertions

- `server/internal/service/collaboration_hold_test.go:417`: the required-commit case selects `SetCollaborationTargetHold`, rejects its commit before the underlying PostgreSQL commit, and requires the exact injected error, one selected rejection and zero post-commit lost replies. The original complete before/after business snapshot assertion remains, covering session, channels, targets, commands, groups, runs, grants, delegations, proofs, attempts, reservations and events.
- `server/internal/service/collaboration_hold_test.go:444`: the unknown-commit case selects the same named business write, preserves the serializable-looking `pgconn.PgError{Code: "40001"}`, and requires that error plus exactly one post-commit lost reply. Before recovery it directly verifies `config_invalid`, one hold-version increment, unchanged channel control, the original unconsumed waiting input, no group/run/reservation and exactly one system audit correlated with the target, refusal code, hold reason and hold version. The next poll must return no run and preserve the entire business snapshot and single audit.
- `server/internal/service/collaboration_runtime_fault_test.go:14`: contains the existing `runtimeScanFaultStarter` / `runtimeScanFaultTx` support. Optional `rejectCommit` and `rejectedCommits` implement selected pre-commit refusal; the existing named-query selection and default post-commit `lostCommits` behavior remain. This block was initially extended in the scan test file and subsequently moved unchanged into this small independent support file. No product helper or additional dependency was introduced.
- `server/internal/service/collaboration_runtime_scan_test.go`: the follow-up removed only that moved block and its unused `sync` import. Remaining scan tests/helpers/imports are preserved.
- This report and the raw logs linked below record the repair. Other agents' changes were preserved; this lane made no git commit.

The simplification is reuse of the existing named-query injector instead of an ordinal transaction assumption or another fault-wrapper type. Preliminary scan transactions can complete normally in both repaired cases.

## Local extraction boundary — recorded before the follow-up edit

The coordinator needs to commit the hold-test repair without staging the large, pre-existing uncommitted S1 scan implementation. The repaired hold tests currently obtain `runtimeScanFaultStarter` from that untracked scan test file, creating an unnecessary commit dependency.

Move the existing `runtimeScanFaultStarter`, `runtimeScanFaultTx` and all their methods verbatim into `server/internal/service/collaboration_runtime_fault_test.go`, with only their required imports. Remove that same block and its now-unused `sync` import from `collaboration_runtime_scan_test.go`. Keep the remaining scan helpers/imports and the hold tests unchanged. This is file placement only: no new abstraction, product edit, fault semantics, staging or commit. The completed 34-top-level/68-total race run above locks existing behavior; verify identical extracted block bytes, then rerun the same focused hold/scan race scope and static checks. The coordinator separately owns validation against a temporary HEAD server snapshot.

## Environment and source identity

- OS: `Darwin arm64`; actual test toolchain: `go1.26.6 darwin/arm64` (forced by the existing task launcher).
- Isolated database: `multica_gap_service_s6_hold`, separate from the coordinator's suite databases. A read-only `SELECT current_database(), max(version) FROM schema_migrations` returned `737_collaboration_runtime_preparation`.
- Tests used the existing agent-CLI guard. No user-installed provider CLI, account or quota was accessed.
- SHA-256 before the file-placement follow-up, `server/internal/service/collaboration_hold_test.go`: `6c0f4fdfbc00a5d6adf4f727e0aee77045bade1881fb1df52ccfc4494ecd7d9e`.
- SHA-256 before the file-placement follow-up, `server/internal/service/collaboration_runtime_scan_test.go`: `e83f79eacbf97823f91a863f1b65bac8260ad5106391fa6e1e53152047b44e6e`.

## Executed results

Counts distinguish top-level tests from the total including subtests. Every listed run used `-race`, `-count=1`, `-p 1`, `-parallel 1` and a five-minute package budget. Green runs had zero failures and zero skips.

| Execution | Result | Top-level / including subtests | Package seconds | Raw evidence |
| --- | --- | --- | --- | --- |
| Unmodified unknown-commit test, dedicated DB | Expected reproduction, exit 1 | 1 failed / 1 failed | 1.109 | [isolated red](hold-unknown-commit-isolated-red.jsonl) |
| Target-selected unknown-commit repair | Passed, exit 0 | 1 / 1 | 2.247 | [first green](hold-unknown-commit-isolated-green.jsonl) |
| First adjacent hold/runtime-scan run, before rollback-helper extension | Passed, exit 0 | 34 / 68 | 49.054 | [intermediate adjacent run](hold-runtime-scan-adjacent-race.jsonl) |
| Controlled targeted rollback experiment using the helper's existing post-commit mode | Expected harness failure, exit 1 | 1 failed / 2 failed | 1.213 | [targeted rollback red](hold-required-commit-targeted-red.jsonl) |
| Final required-write rollback and unknown-commit cases | Passed, exit 0 | 2 / 5 | 3.224 | [final focused green](hold-required-and-unknown-commit-green.jsonl) |
| Final adjacent hold/runtime-scan run after all changes | Passed, exit 0 | 34 / 68 | 41.665 | [final adjacent race](hold-runtime-scan-adjacent-race-final.jsonl) |
| Same hold/runtime-scan scope after support-file extraction | Passed, exit 0 | 34 / 68 | 43.826 | [post-extraction race](hold-runtime-scan-after-extraction-race.jsonl) |

The controlled rollback red was not an unmodified product failure: changing only the fixture's selection to the existing post-commit injector made the unchanged rollback snapshot correctly detect committed hold/audit state. Adding the selected pre-commit mode then restored the intended rollback exercise. Its rejection counter verifies that the business transaction, rather than a cursor transaction, hit the failure.

The coordinator's earlier independent red is also retained as [hold-unknown-commit-red.jsonl](hold-unknown-commit-red.jsonl).

Exact Go commands (working directory `server/`); redirects used the evidence filenames in the table:

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s6_hold \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/service \
  -run '^TestCollaborationPromoteHoldUnknownCommitIsNotRetried$' \
  -p 1 -parallel 1 -count=1 -timeout=5m -json

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s6_hold \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/service \
  -run '^TestCollaborationPromoteHoldRequiredWritesRollback$/^commit$' \
  -p 1 -parallel 1 -count=1 -timeout=5m -json

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s6_hold \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/service \
  -run '^TestCollaborationPromoteHold(RequiredWritesRollback|UnknownCommitIsNotRetried)$' \
  -p 1 -parallel 1 -count=1 -timeout=5m -json

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s6_hold \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/service \
  -run '^TestCollaboration(TargetConsentRetainsInitialHold|ResourceHold|UnusedAliasHold|TargetResume|AccountInvalidationFindsNonRootBatchAuthor|ReconsentHold|RuntimeHold|ResumeRejectsHold|ResumeAllMaterializesDiscoveredHold|HoldControl|ControlOperationReplaysOnlyAuthorizedOriginalResult|ReconfigureRetainsSwitchHold|Promote|Runtime(WaitingScan|CandidateScan|Scan|Wake|PendingDerived))' \
  -p 1 -parallel 1 -count=1 -timeout=5m -json

GOTOOLCHAIN=go1.26.6 go vet ./internal/service
```

`go vet ./internal/service` passed before and after the helper extension (exit 0, empty output); the final capture is [hold-go-vet-final.log](hold-go-vet-final.log). `gofmt` was applied to both changed Go test files. After the final race run, `gofmt -l` returned no files and scoped `git diff --check` exited 0. An independent evidence parser confirmed the complete package result, 34 top-level/68 total passes, no failures/skips, existing report links, clean whitespace in both test files/report and unchanged source hashes.

## Extraction result and final source hashes

The code move has no semantic change. Mechanical inverse reconstruction after `gofmt` restored the original scan source SHA-256 `e83f79eacbf97823f91a863f1b65bac8260ad5106391fa6e1e53152047b44e6e` by putting back only the moved block and unused import. The extracted block itself hashes to `789a667037e9c5c0b48f71e35b34019c633bafdc82423283989a8d0859ac7d35` when its original separator newline is restored; no struct field, method or condition changed. The hold-test hash is unchanged from the first verified fix.

| Final source | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_hold_test.go` | `6c0f4fdfbc00a5d6adf4f727e0aee77045bade1881fb1df52ccfc4494ecd7d9e` |
| `server/internal/service/collaboration_runtime_fault_test.go` | `e91e3a6eac2c8ea95684e4a1270eddacf3b7dab31ebd022a0288e77d4a8bd6a9` |
| `server/internal/service/collaboration_runtime_scan_test.go` | `33a62235826dd4329a86b37b0c4835ca75a82ee860ddbe2619a00c182e6e4e61` |

The post-extraction run used the same fourth Go command above, with its output saved in the new evidence file. `GOTOOLCHAIN=go1.26.6 go vet ./internal/service` passed again (exit 0, empty output; [capture](hold-go-vet-after-extraction.log)). All three Go files are gofmt-clean. This lane made no staging or commit changes; the coordinator's independent HEAD-snapshot candidate validation remains separate.

## Remaining boundaries and coordinator handoff

This verifies service/database behavior and the test harness on macOS; it does not prove Web/Electron wiring, a real Linux provider process, other operating systems, packaged clients, quota use, API/Web/Playwright pipeline completion or the full service package. Full acceptance and the 97-row ledger remain owned by the coordinator. Production creation/execution gates remain outside this lane.

The runtime-recovery spec should retain the general rule already used by FIFO tests and extend its examples to hold commit faults: select the business SQL/lock identity, since preliminary cursor commits are expected. Spec, progress, acceptance catalog and Lore commits remain coordinator-owned.
