# Runtime AC1 / AC5 deterministic test addendum

Date: 2026-11-01. Both assigned gaps are now covered by database tests. No production code, existing tests, migrations, task ledgers, or provider processes changed. No commits were made by this implementation lane.

## Source and environment

- Product baseline: `0e1ee8e1e`. Prior integration evidence: `2b861347e`.
- Final race run HEAD: `765ab186a222df1e0135f311d081a79779d51726`. [Committed server comparison](committed-server-baseline-diff.txt) against the product baseline is empty. Concurrent documentation/task edits are recorded, not claimed, in [race-source.json](race-source.json).
- Platform: macOS 26.2, arm64; `go version go1.27.0 darwin/arm64`.
- Database: `multica_runtime_integrate_20261031_f2478725`, loaded from the assigned private `/tmp/multica_runtime_integrate_20261031_f2478725.env`. Credentials were not written to evidence. Service database tests ran exclusively; no handler/full suite ran concurrently. Vet ran alongside race tests without database access.

| New test file | SHA-256 of tested source |
| --- | --- |
| `server/internal/service/collaboration_checkpoint_authority_matrix_test.go` | `2adea8eadd3d6da5ba12a0b6e296b4c0fc0d92ba6f1a89673b5532681f4bd2e8` |
| `server/internal/service/collaboration_wakeup_boundary_test.go` | `073822db3b3e130259b572b53e7b4562c477a057a6e4485937d3a1c08e5d412e` |

## Gaps closed

`TestCollaborationCheckpointAuthorityTransactionOrder` runs six subtests: `stop`, `revoke_grant`, and `author_leave`, each with `checkpoint_first` and `authority_first`.

The first real service transaction is held before commit using the existing `recipientCommitStarter`; `waitFIFOBlocked` observes the competing transaction through PostgreSQL `pg_blocking_pids` before release. Each case starts with one confirmed checkpoint. The second checkpoint is retained only when its transaction wins. The authority-first path returns the explicit `business_capability_required` refusal. Both paths preserve the original author and committed prefix, reject rewritten/original-key or new late body content as appropriate, return only receipt fields on exact original replay, reject checkpoint writes using termination credentials, and accept a metadata-only seal with a known missing tail. The seal leaves the unknown proof and physical reservation intact. Original launch termination and repeated original final proof settle the run and release capacity. Authorized human records and exact event counts verify retention without duplication. Bounded contexts fail stalled completion; all six controlled orderings completed without deadlock or timeout.

`TestCollaborationHandoffTwoMembersAndClaimBoundary` uses two distinct member agents on the shared managed-runtime fixture. Their success/failure outcomes, two member comments, and one admitted human joined input coalesce into exactly one continuation with five exact assigned causes. The handoff verifies both original result event IDs/bodies, member parent identity, typed `provider_result`, and unknown final-answer status.

After that continuation claims and freezes its snapshot, one late human input and one runtime comment allocate the next leader turn. A fresh delegation to one existing member supplies a late proof. All three late causes belong only to the second continuation. Original claim inputs and the frozen snapshot replay unchanged; later human history retains only the original five causes for the first turn. Exact reader sets are checked for the root input (six readers), first joined input (three), and late input (only the second continuation). Comment/proof replays create no duplicate sources or leaders. Ordinary untargeted human discussion creates neither a command nor a continuation. The final leader settles the group with no pending wakeup.

## Commands and observed results

Database commands ran after `set -a; source /tmp/multica_runtime_integrate_20261031_f2478725.env; set +a`. The database name was checked before the checkpoint and final race commands.

| Exact command | Observed result | Evidence |
| --- | --- | --- |
| `go -C server test ./internal/service -run '^TestCollaborationCheckpointAuthorityTransactionOrder$' -count=1 -timeout 3m -json` | Exit 0; one top-level test, six subtests; 4.844 s package runtime | [JSON](checkpoint-first.jsonl), [stderr](checkpoint-first.stderr), [exit](checkpoint-first.exit) |
| `go -C server test ./internal/service -run '^TestCollaborationHandoffTwoMembersAndClaimBoundary$' -count=1 -timeout 3m -json` | Exit 0; one top-level test; 3.602 s package runtime | [JSON](wakeup-first.jsonl), [stderr](wakeup-first.stderr), [exit](wakeup-first.exit) |
| `go -C server test -race ./internal/service -run '^TestCollaboration(CheckpointAuthorityTransactionOrder\|HandoffTwoMembersAndClaimBoundary)$' -count=1 -timeout 5m -json` | Exit 0; two top-level tests, six subtests; 9.544 s package runtime; no race report | [JSON](matrix-race.jsonl), [stderr](matrix-race.stderr), [exit](matrix-race.exit) |
| `go -C server vet ./internal/service` | Exit 0; no diagnostics | [Log](service-vet.log), [exit](service-vet.exit) |
| `gofmt -l server/internal/service/collaboration_checkpoint_authority_matrix_test.go server/internal/service/collaboration_wakeup_boundary_test.go` | Exit 0; no unformatted files | [Log](gofmt.log), [exit](gofmt.exit) |
| `git diff --check` and `git diff --no-index --check /dev/null <each-new-test-file>` | No whitespace diagnostics | [New-file results](diff-check.json) |

All selected tests actually ran: zero skips, zero failures, and exactly the expected `run` and `pass` events. [inventory.json](inventory.json) lists every actual test name. The reproducible [inventory guard](verify-inventory.py) also rejects empty selections, missing/duplicate passes, nonzero command exits, and changed source hashes. Run it from any directory with `python3 <this-directory>/verify-inventory.py`.

The first Go runs passed without source fixes. One evidence-validator attempt incorrectly treated Git's expected no-index exit 1 for added files as a failure; [the original diagnostic and correction](inventory-initial-error.log) are retained. The corrected check requires empty whitespace diagnostics and permits only the documented diff exit statuses. It did not loosen a product assertion or require a test rerun.

## Scope and remaining work

The boundary plan is [boundary-plan.md](boundary-plan.md). Existing dbfx, managed/host fixtures, commit barrier, and polling helpers were reused; no new production abstraction or test hook was introduced. Full Go/TypeScript suites were not repeated because this lane only adds the two targeted tests; their prior evidence remains separate. No production defects or unresolved failures were found in this scope. Independent review and broader parent acceptance remain the coordinator's responsibility. These synthetic service proof payloads do not establish real provider execution, kernel containment, native-client behavior, or model understanding.
