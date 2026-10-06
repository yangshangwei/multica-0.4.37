# S2 W03/W04 system status writers

Prior verified commits: S1 `837e1abaf`; W15 `7d53c9ed0`; W02 `c6980d5e5`;
W05 fallback removal and W17 narrow exemption `e3cee069c`. W03/W04 are now
verified and committed with this record; this is not an FG pass. No production
capability, lifecycle, closure or UI work.

## Real database RED

- W03: `TestPullRequestWebhookIterationFacts/github` and `/forgejo` drove real
  webhook handlers. Both completed the issue but left events=0/scope_revision=1
  instead of events=1/scope_revision=2. `/tmp/i1-s2-w03-red.log`, exit 1.
- W04: `TestHandleFailedTasksIterationFacts` drove the real failure-settlement
  service with an exhausted retry budget. Issue became todo/revision=2 but
  events=0/scope_revision=1. `/tmp/i1-s2-w04-red.log`, exit 1.

The task-only local PostgreSQL database remains `multica_i1_s1_20261006`; tests
use the repository agent CLI guard. No shared development database is a target.

## Implementation and review boundary

The new service-owned status transaction returns locked before/after/changed;
the callers keep notification/publication after commit. W03 is shared by GitHub
and Forgejo/GitLab VCS, so its policy rechecks both terminal state and the current
combined PR close aggregate. W04 keeps retry and delegated recovery outside the
issue-reset transaction and rechecks effective in_progress plus no active task.

The issue must be locked FOR UPDATE, not NO KEY UPDATE: the execution guard
takes KEY SHARE NOWAIT. An active-task count is a separate READ COMMITTED
statement after that lock. Enqueue-first therefore commits before the count;
reset-first excludes enqueue/start/promotion during the mutation. Explicit RC
is required for this new transaction owner; inheriting a database RR default
could retain a pre-wait snapshot even when an enqueue did not change the issue
tuple. Existing W01/P1 isolation and retry budgets remain unchanged.

System actors have an explicit source and null id/user_id. No fallback to
autocommit or new retry budget was added. Production call-site audit found W03
using the handler's pool-backed starter and W04 reached from the runtime sweeper
and RecoverOrphanedTasks using the same production TaskService. No borrowed
production pgx.Tx starter/copy was found. The new owner does not nest a transaction
inside the preceding retry/delegated-recovery operation.

## Failed verification attempts retained

The first reset-first test paused the first Commit on TaskService, which belonged
to delegated recovery, not the later issue reset. It did not prove reset locking:
`/tmp/i1-s2-w04-expanded.log`. The barrier now arms only after UpdateIssueStatus.

The next test incorrectly expected CreateAgentTask to immediately return no row.
The actual statement first waits in the older `lock_task_owner_rows`, then reaches
the NOWAIT `lock_issue_execution`. Its timeout is preserved in
`/tmp/i1-s2-w04-final.log`. Production locking was not weakened or changed to make
this expectation pass. The final test proves direct execution admission false,
actual enqueue blocked (pg_blocking_pids), no publication before reset commit,
then successful enqueue after release. Enqueue-first also proves actual server
waiting and forces an RR initial starter to catch a missing explicit RC choice.

## Final evidence

- W03 executor run: 19 passing entries (13 top-level), no skips/failures;
  `/tmp/i1-s2-w03-final.log`. Real GitHub/Forgejo/GitLab and duplicate deliveries
  produce one event with system/source/null identity. Cancellation/open-PR changes
  before the lock prevent stale completion; event failure restores issue/version.
- W04 executor run: 11 entries (4 top-level), no skips/failures;
  `/tmp/i1-s2-w04-final2.log`. Includes original/started retention, active/retry and
  custom status guards, rollback, both enqueue orders and publication timing.
- Final independent handler run: 154 entries (91 top-level), no skips/failures;
  `/tmp/multica-i1-s2-system-handler-final.jsonl`.
- Root strengthened cancellation to call the real W01 UpdateIssue handler rather
  than raw SQL. Its own cancel event/scope advance remain; the later webhook adds
  nothing and preserves cancellation. The strengthened three-case test passed
  4 entries including its parent; `/tmp/multica-i1-s2-system-live-cancel.jsonl`.
- Final independent service run: 15 entries (8 top-level), no skips/failures;
  `/tmp/multica-i1-s2-system-service-final.jsonl`. It includes latest repeated
  settlement assertions, the existing final delegated-failure/T1 contracts and
  W02 stale-content/deleted-workspace regressions.
- Existing production sweeper/start regressions: 10/10 passed, no skips/failures;
  `/tmp/multica-i1-s2-system-sweeper-final.jsonl`. This also checks the W05 tests
  through the newly integrated W04 path.
- Iteration pure/domain tests: 69 entries (30 top-level), no skips/failures;
  `/tmp/multica-i1-s2-system-iteration-final.jsonl`.
- `go -C server build -p 2 ./...` and `go -C server vet -p 2 ./...` both exited 0;
  `/tmp/multica-i1-s2-system-build.log`, `/tmp/multica-i1-s2-system-vet.log`.
  gofmt and diff checks are clean. No SQL/migration changed in W03/W04, so sqlc
  stability evidence remains the preceding W02/W15 run, not a claimed new run.
- Final independent read-only review found no remaining code blocker.

Independent DB commands (same isolated DATABASE_URL; packages run sequentially):

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/handler -run 'Test(PullRequestWebhookIteration|Webhook_|VCSWebhook_|CombinedCloseAggregateSpansProviders|UpdateIssueIteration|PatchPluginIssueIteration|IssueWriteFenceSQL|ProjectAssociation|DeleteWorkspace|FirstCompletionAnalytics|MarkIssueFirstExecuted|ConcurrentRevisionWritesHaveExactlyOneWinner|NoOpIssue|IssueUpdateAndAttachmentBindingExcludeInterleavingMutation|IssueUpdateRollsBackWhenAttachmentBindingFails|UpdateIssueReassign|BatchUpdateIssueReassign|UpdateIssueCancelStatus|BatchUpdateIssueCancelStatus)' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/handler -run '^TestPullRequestWebhookIterationRechecksAndRollsBack$' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/service -run 'Test(HandleFailedTasksIteration|HandleFailedTasksFinalDelegatedFailureWakesCoordinator|TriageFailureSettlementSurvivesRetryRefusal|IssueContentDeletedWorkspacePreservesConditionalConflict|TriageStaleContentEditPreservesAcceptedAssignment)' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./cmd/server -run 'TestSweep|TestStartTaskSkipsUnchangedAgentStatusWriteAndBroadcast' -count=1 -json
```

## Limits and continuation

Only issue reset/completion plus iteration facts is atomic here. The failed task
already existed, and preceding automatic retry/delegated recovery keeps its
existing independent effects. Webhook PR metadata keeps its existing persistence
and best-effort failure response contract. Do not claim rollback of those prior
operations or complete execution atomicity.

Next remains S2: W06–W14 and W16, operation/read authorization, capability/settings,
explicit-field compatibility, member notification cleanup and complete performance
evidence. W14 can reuse same-membership recording but needs all affected issues,
sorted iteration/issue locks, a single lock-time sample for the owning batch, and
unchanged P1 RC/retry/CAS behavior. W12 needs a deletion/leave recorder extension
that retains original facts; its pre-existing external task cancellation is a
separate contract. W13 needs a tenant-scoped atomic transfer/archive transaction.
No full Go repository suite, browser/Electron E2E, remote CI or FG pass is claimed.
