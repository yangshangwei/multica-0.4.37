# Independent S1 / S3a check — 2026-10-21

Status after targeted follow-up: **S1 and S3a source review passed; the confirmed P2 is closed.** The original finding and first-run results below remain historical evidence; the final follow-up records its correction. This report does not close AC-I01, AC-I02, AC-I04 or the parent task.

The check loaded the complete saved native hook, current PRD/design/implementation plan, progress and both S1 plus S3a implementation reports. Comparisons use `/tmp/multica-gap-recovery-20261021-h6phxwo9/files/` for pre-existing dirty files and the recorded HEAD for previously clean files. Earlier permission, nonce/permit, reservation/derived and account-cache fixes are not attributed to this increment. Active S2 work was not edited. No product source was changed by this reviewer.

## Findings (fixed)

None by this reviewer. The S1 implementer independently corrected the FIFO test's source-transaction barrier and preserved the previous cursor position when a changed priority produces an empty page. Those changes are present in the reviewed source and in the latest 34-test log described below.

## Findings (not fixed at initial review)

### P2 — An in-progress scan can recreate a cursor after runtime deletion

- Files: `server/internal/service/collaboration_runtime.go`, `server/pkg/db/queries/collaboration_runtime.sql`, `server/pkg/db/queries/runtime.sql`; the related workspace cleanup is in `workspace_delete.sql`.
- `collaborationRuntimeScanPage` authorizes the runtime and binding through ordinary reads, then calls `EnsureCollaborationRuntimeScanCursor`. It does not participate in the runtime row lock held by actual deletion. `TeardownRuntime` takes `LockAgentRuntime` before cleanup (`runtime_teardown.go:120`), and workspace deletion takes `LockWorkspaceTaskOwnerRuntimes` before its cleanup. The new cursor has deliberately no foreign key, so its insertion obtains no parent-row lock implicitly.
- Deterministic interleaving: scan reads the existing runtime and pauses immediately before Ensure; deletion locks that runtime `FOR UPDATE`, executes the actual `DeleteAgentRuntime` CTE and commits; scan resumes its actual Ensure and commits. One cursor remains with no corresponding runtime. Existing sequential cleanup coverage does not exercise this interleaving.
- Reproduction ran against the task-owned `service_s1` PostgreSQL database using two transactions in a fresh private schema. It applied the actual 730/731 migrations and extracted the current Ensure/Delete statements directly from their production SQL files. The deletion explicitly acquired the same `FOR UPDATE` row fence as teardown before the CTE. Result: `authorized_runtime_read=true`, `runtime_delete_committed_before_cursor_ensure=true`, **`orphan_cursor_count=1`**. The private schema was removed afterward. This is SQL-boundary evidence plus inspected application lock paths, not a full handler test.
- Artifacts: `/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s1-s3a/cursor-delete-repro.py` and `cursor-delete-repro.json`. The JSON records SHA-256 values of both actual SQL statements.
- Command, from `server/`: `python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s1 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/check-s1-s3a/cursor-delete-repro.py` (exit 0; successfully reproduced the defect).
- Recommendation: acquire the runtime row fence before cursor Ensure/read/write and re-authorize after acquiring it. Keep the scan's lock order runtime → cursor, and keep per-object wake/promotion in their existing separate transactions. A deletion or authority change while awaiting the fence must refuse the scan rather than recreate metadata. Add deterministic runtime and workspace deletion competition cases using the real teardown boundaries.
- Disposition: root confirmed the finding and assigned the source fix and regression tests to `s1_server`. This reviewer did not modify shared server files or generated SQL. **The correction has not yet been reviewed in this report.**

No other concrete S1/S3a defect was found within the reviewed increment. Pending product/browser/native acceptance below is a verification limit, not an additional code finding.

## Reviewed behavior

### S1 service and daemon

- Server candidate selection preserves the original `run_ids` response and offers at most one run, matching the daemon's one Claim attempt. Full-scope priority selection is repeated inside the candidate-page SQL snapshot, including after waiting on the cursor lock. Each priority has its own durable position; empty pages now preserve existing progress. No daemon candidate cursor or candidate metadata remains.
- Waiting and wake scans retain their separate 16-item budgets and stable keysets. The derived-run EXISTS predicate matches the unreserved subset of the complete candidate predicate and does not depend on the offered tier/page. Rejected prefixes do not become fabricated holds or execution success.
- Only named object refusals, the ledger sentinel and exhausted rolled-back `collaboration_busy` are isolated. Context/global identity/unclassified database failures propagate. The distinct commit-error wrapper prevents even an embedded ordinary refusal from being swallowed or replayed as a rolled-back mutation.
- Proof recovery uses one private, fsynced sidecar cursor independent of run JSON, writes its position before the attempted lock/request, and bounds a round to eight selections and one inherited ten-second deadline. The retained pre-burn path passes that context into finishing. Foreground post-execution finishing retains its prior shutdown behavior.
- Inventory validation precedes a busy-cursor return. Cursor and ledger damage fail closed; unknown entries are excluded from proof attempts and keep the existing machine/update barriers. Recovery reuses stored identities, attempt nonce and sealed proof/receipt; it never creates a substitute fork or Resume.
- Migration 730 is additive with no foreign key; 731 is a single `CREATE UNIQUE INDEX CONCURRENTLY` statement. Its exact invalid-index cleanup registration exists in `cmd/migrate/main.go`. Down files use a NOWAIT table fence and refuse populated cursor state. Sequential runtime/workspace cursor cleanup exists, and the workspace deletion manifest classifies the table as `workspaceDelete`; the concurrency hole is the P2 above.

### S3a human mentions

- The helper only accepts the special pair `all` / `all` and always emits `[@all](mention://all/all)`. UUID-based member/agent/squad branches keep their issue-projection membership checks. No synthetic account or AI target type was added.
- The shared composer mounts mentions only from the protected issue query and remains absent when commenting is disallowed. People and AI have visible localized labels and distinct accessible groups. `@all` belongs to People and remains independent of AI submission availability; it appears only with loaded mention data.
- Insertion calls the existing change handler, invalidating the old preview while retaining explicit target Markdown. The existing pending/locked guards, source-specific delivery draft, attachment/reply identity, suppression, preview correlation and captured authority versions remain intact. All notification expansion and AI execution decisions stay on the server.
- Source hashes for all four changed core/composer files exactly match the implementation report's successful test runs. No changes to a description editor, shared API schema, directory query or notification service were introduced.

## Verification

Reviewer-run checks:

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/core exec eslint collaboration/comment.ts collaboration/comment.test.ts` | PASS, exit 0 |
| `pnpm --filter @multica/views exec eslint collaboration/comment-composer.tsx collaboration/comment-composer.test.tsx` | PASS, exit 0 |
| `pnpm --filter @multica/core typecheck` | PASS, exit 0 |
| `pnpm --filter @multica/views typecheck` | PASS, exit 0 |
| `GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/collaborationrun` | PASS, exit 0 |
| `gofmt -l` on the six changed/new daemon/proof Go files | PASS, no output |
| Scoped `git diff --check` for daemon/runner and four core/composer files | PASS, no output |
| Guarded race command below, collaborationrun package | PASS: 5 new top-level proof-recovery tests, 0 failing/skipped; private cursor, actual child-process exclusion and damage subcases |
| Same race command, daemon package | BLOCKED at compilation by active S2 definitions; overall command exit 1 |
| `GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon ./internal/daemon/collaborationrun` | BLOCKED by the same active S2 definitions, exit 1 |

The guarded race command was:

```sh
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/daemon ./internal/daemon/collaborationrun -run '^(TestCollaborationProofRecovery|TestCollaborationCandidatePaging|TestProofRecovery)' -count=1 -timeout=90s -v
```

Both blocked commands name only S2's currently undefined `CollaborationOfferResourceParams`, `CollaborationWithdrawResourceOfferParams` and `CollaborationAdoptResourceOfferParams` at `collaboration_queue.go:268–272`. No S1 assertion failure was observed in those runs. Root has been notified to rerun dependent Go checks when S2 compiles; this report does not relabel blocked commands as passing.

Individual logs, command/exit/elapsed JSON records and the reviewed daemon/S3a source hashes are under `/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s1-s3a/`. Standalone vet, gofmt and whitespace checks additionally have their actual successful tool results in this review turn.

Inspected current implementation evidence, without redundantly rerunning the broad suites:

- `s1a-baseline-red.log` records failure of waiting/candidate persistent-prefix progress, object isolation, priority and concurrent cursor cases on the old implementation. `s1a-composed-red.log` records the failure of 16-item page advancement with one actual Claim per List.
- The latest `s1a-race-verified.log` contains **34 top-level PASS, 0 FAIL, 0 SKIP**, 42.151s. It includes the corrected empty-page priority/cursor regression and the FIFO barrier repair. It predates the newly identified deletion-competition fix.
- `/tmp/multica-s3-all-core-red.log`: 2 failures / 15 passes; `/tmp/multica-s3-all-views-red.log`: 1 failure / 5 passes. These fail on canonical all insertion and the missing People group respectively.
- `/tmp/multica-s3-all-core-green.log`: **52 files / 817 tests passed**, and `/tmp/multica-s3-all-views-green.log`: **15 files / 60 tests passed**. All four production/test file hashes were rechecked against the recorded values. These are the implementer's executions, inspected by this reviewer, not newly executed 877-test results.

## Handoff and limits

- Review-generated repository file: this report only. The isolated SQL reproduction and check evidence live outside the checkout. No commits, task archival, product gate changes or real provider calls occurred.
- Root owns the P2 follow-up, executable spec synchronization, generated SQL and any final broad verification. Specs should retain the one-candidate compatibility contract, runtime→cursor deletion fence, proof cursor budget/deadline and human-only all selection once the fix is verified.
- Production Web, actual Electron, visual-verdict, wide/narrow/200% layout, server notification/edit/revocation integration, Linux containment, provider/OS and packaged-client checks are not proven by this review. They retain their separate ownership and original acceptance requirements. S3a source approval is not full AC-I04 or parent acceptance.

## Targeted follow-up — P2 closed

The reviewer inspected the final P2 correction only, including `collaboration_runtime_scan_delete_test.go`, the adjusted candidate-priority race, and the updated S1a implementation record. No S2 code or previously reviewed S3a implementation was changed or re-reviewed.

- `collaborationRuntimeScanPage` now acquires the existing `LockAgentRuntime` fence before authorization and any cursor Ensure/read/write. A runtime removed while it waits returns `runtime_not_found`; current binding/account validation runs after the lock and rejects an obsolete binding with `managed_runtime_required`. Cursor creation cannot occur on either refusal.
- The lock order is runtime → cursor, matching real runtime and workspace deletion. Authorization and page queries introduce no user/namespace write locks. The short scan transaction commits before `ListRuntimeRuns` starts per-object wake/promotion work, so it does not carry the new lock into that separate lock hierarchy. Candidate priority is still rechecked in the page snapshot, and an empty changed-priority page preserves the previous position.
- The new canonical service tests run real `TeardownRuntime` and `LockWorkspaceTaskOwnerRuntimes` paths, rather than only a raw deletion. They cover runtime/workspace deletion in both scan-first and delete-first orders, plus binding change while waiting for the runtime fence. They synchronize on actual `pg_blocking_pids`, assert refusal or serialized completion as appropriate, and verify zero surviving cursor rows.
- `s1a-delete-race-red.log` contains two failing top-level tests: the four deletion interleavings left 1/2/1/2 orphan rows, and the binding case accepted the old identity. `s1a-delete-race-green.log` contains **13 top-level PASS, 0 FAIL, 0 SKIP, 17.286s** after the fix. The selected batch also preserves candidate/waiting fairness, concurrent pollers, priority changes, object isolation, unknown-commit recovery, sequential cleanup and FIFO/cancellation assertions. Both logs were read directly during this follow-up.
- All four SHA-256 values in the updated S1a record match the files read here: `collaboration_runtime.go`, `collaboration_runtime_scan_test.go`, `collaboration_runtime_scan_delete_test.go`, and `queries/collaboration_runtime.sql`. The implementer/root report a successful full service `go vet` in that same stable source window. Reviewer-run focused gofmt and whitespace checks also pass. No broad tests were repeated; the 13-test execution is correctly attributed to the implementer, with log and source-hash verification by this reviewer.

**Final slice verdict:** the runtime-deletion P2 is resolved, with no remaining concrete code findings in the reviewed S1/S3a increments. The fix reuses an existing fence and adds no SQL, migration, public wire field or dependency. The initial SQL-only diagnostic remains historical evidence for the missing-fence implementation; the real service interleavings are the correction's acceptance evidence. S3a production Web/Electron/visual acceptance and all wider task/provider/platform requirements remain pending under the ownership already recorded above.
