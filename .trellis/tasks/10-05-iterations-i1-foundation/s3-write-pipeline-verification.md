# W01 bounded read-pipeline optimization

## Diagnosis and plan before production edits

The failed capacity run is retained in `s3-capacity-verification.md`. Thresholds
remain unchanged. Inspection against `e683c08d9` confirms the member/title-only
W01 path and ordinary benchmark have no new behavior: current authorization
changes execute only on create or touched assignee fields. SQL counts stay eight.

Four-writer ordinary mean handler latency implied by throughput is about
7.31 ms; its median lock-statement span is 5.222 ms (approximately 71%). The
populated current-iteration/participation/latest-event queries execute in
0.022/0.010/0.012 ms through indexes. PostgreSQL reports no deadlocks or temp
files; after fixture cleanup/autovacuum issue/event dead tuples are zero. This
supports reducing round trips while holding serialization locks, not removing
locks or weakening authorization. The measurements do not attribute the entire
regression to new code or establish an exact environmental cause.

The official installed pgx 5.9.2 source confirms pipeline query messages are
ordered Bind/Describe/Execute per statement. Its RowToStructByName documentation
supports strict struct/column matching; CollectExactlyOneRow closes rows and
returns ErrNoRows for an empty result. No new mapper/dependency is needed.

Real PostgreSQL protocol proof passed under race detection:
`TestIterationPipelinePostFenceSnapshot`. Seven separate statements queued in
one batch saw membership/title committed by a fence holder after the batch was
sent. The holder could still lock the issue NOWAIT before releasing the fence.
Log: `/tmp/multica-i1-capacity-pipeline-proof.log`.

Approved narrower production plan:

1. Keep the initial five-lock batch, its current authorization handling, and
   status validation unchanged.
2. With no attachments, send the current-iteration locking lookup followed by
   the issue locking lookup as two separate statements in one subsequent pgx
   batch. This saves one round trip under the held fences.
3. With attachments, retain iteration -> attachment -> issue sequencing and
   the existing code path. Preserve error classification, transaction owner,
   isolation, retry budget, and all business mutation/recorder code.
4. Reuse sqlc model structs through pgx's documented row mapper; mirror exact
   authoritative SQL and add equivalence checks so schema/query edits cannot
   silently drift. Consume and close every batch result.
5. Verify actual HTTP no-attachment join and lock ordering, existing attached
   join/CAS/rollback/authorization/P1 regressions, then obtain an exclusive
   acceptance benchmark window. No thresholds are changed and no passing
   capacity claim is made until those runs complete.

The initial broader seven-statement protocol proof is not permission to move
protected reads ahead of current member-result handling. The production change
uses the narrower post-fence batch described above.

## Implementation and correctness result

`lockIssueReadRows` in `handler/issue.go` uses two ordered statements and strict
pgx mapping into existing sqlc `Iteration` and `Issue` structs. Missing current
iteration remains the unassociated case; missing issue remains a wrapped
ErrNoRows. SQL statements are unchanged and equivalence-checked. No query/index
or dependency was added, and no authorization/recorder/attachment behavior was
removed.

The guarded race regression run passed **83 test entries, zero failures/skips**:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 \
  ./internal/handler \
  -run 'Test(IssueReadLockSQL|IterationPipelinePostFenceSnapshot|UpdateIssueIteration|IssueWriteFenceSQL|RevisionConflictsPreserveLatestIssueAndComment|TextBaselinesIgnoreUnrelatedAggregateRevisionChanges|ConcurrentRevisionWritesHaveExactlyOneWinner|NoOpIssue|IssueUpdateAndAttachmentBindingExcludeInterleavingMutation|IssueUpdateRollsBackWhenAttachmentBindingFails|ProjectAssociation|UpdateIssueReassign|BatchUpdateIssueReassign|UpdateIssueCancelStatus|BatchUpdateIssueCancelStatus)' \
  -count=1 -json
```

Log: `/tmp/multica-i1-read-pipeline-regressions.jsonl`. New actual HTTP tests
`TestUpdateIssueIterationReadPipelineLockOrder` cover a join committed while
waiting on I1 and an iteration-row blocker; both prove issue NOWAIT remains
available until the preceding lock releases, then verify the event carries
the holder's committed before-title. Existing attached join, rollback, CAS,
current authorization and P1 deletion/association tests also passed.

`go -C server vet -p 2 ./internal/handler` exited zero; log
`/tmp/multica-i1-read-pipeline-vet.log`. Formatting and `git diff --check` passed.

## Acceptance after the specific code change

The exclusive post-change runs passed every frozen ordinary and populated
writer threshold; complete raw numbers/comparisons are in
`s3-capacity-verification.md`. Populated unassociated four-writer medians improved
from 445.7/s and P95 12.59 ms to 644.8/s and P95 9.963 ms; original small
four-writer medians improved from 547.1/s and P95 11.30 ms to 696.7/s and
P95 9.168 ms. Transaction statement counts stayed eight/twelve and both HTTP
and durable-event assertions passed. The populated P95 margin is narrow;
retain that limitation when reporting readiness. The initial failed runs were
not discarded and no threshold changed. This completes the bounded writer
performance correction, not the rest of FG or later lifecycle capacity gates.
