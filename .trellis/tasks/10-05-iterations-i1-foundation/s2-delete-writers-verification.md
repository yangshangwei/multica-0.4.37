# S2 W12 issue deletion facts

W12 extends the existing single/batch deletion owner. It does not enable I1 or
establish FG; other writer and foundation gates remain separate.

## Behavioral evidence

The assigned isolated database is `multica_i1_s1_20261006`. A live SELECT proved
the database name, `multica` user and migration ledger maximum
`566_iteration_notification_day` before testing. No development database was used.

`/tmp/i1-w12-red.log` is a prerequisite failure: another lane's pending
`LockAgentTaskForStart` generated query prevented compilation. After root's
coordinated generation, `/tmp/i1-w12-red2.log` is valid behavioral RED: active and
planned single deletes returned204 with zero deletion facts; batch deletion
returned200/deleted4 with zero facts. The first focused GREEN is
`/tmp/i1-w12-green.log`.

Additional authority tests initially replaced the request's Chi context with a
new background context and never reached the transaction. This fixture problem
is recorded in `/tmp/i1-w12-green2.log`; deriving the timeout from the request
context fixed it. `/tmp/i1-w12-green3.log` passes the seven focused top-level
tests then present, including member/agent/task reauthorization and wall-clock
rollback. The final run also covers sampling after the last participation lock.

## Transaction and retained data

- Keep the existing Begin/commit owner and retry policy. Reuse
  `lockIssueWriteFences`: workspace KEY SHARE, subscriber fence, active member
  SHARE, status catalogue, I1 workspace fence.
- Read current iteration IDs after the fence, lock iterations by ID, then lock
  selected issues and their direct children in a global issue-ID order. Preserve
  the batch's excluded IDs so selected descendants do not produce survivor events.
- Revalidate current agent autonomy, archive state and task identity with NOWAIT
  reference locks. Hold member and actor locks through commit; persist/broadcast
  the same resolved actor.
- Prepare the selected issues' participation facts as one batch after locks;
  sample database wall time once. Child parent/stage detachment is not a
  statistical scope event.
- Append one `delete` event with before facts and JSON-null after facts before
  deleting each current participant. The single event represents deletion plus
  leave, avoiding a doubled removed count. Release current participation and
  started state; retain original facts, participation identity and prior events.
  `last_left_at` follows the event's clamped occurred_at while sampled_at keeps
  actual wall time. Deleting an issue with only former participation does not
  rewrite its old history. Frozen snapshots remain unchanged.
- Source-context/attachment cleanup, child detachment, issue deletion, event,
  participation and scope revision commit or roll back together. A failure at
  the second issue deletion proves the first deletion and its cleanup roll back.

## Verification status and boundaries

Root owns shared SQL generation and final integration. Earlier broad race/vet
attempts were blocked by in-progress sibling test edits (`iteration_revoke_test.go`
imports, then `lifecycle_handoff_atomic_test.go` releaseOnce). The intermediate
`/tmp/i1-w12-regression.jsonl` and `/tmp/i1-w12-regression2.jsonl` are not passes.

Final verification after the coordinated tree stabilized:

- `go test -race ./internal/handler -run '(DeleteIssue|BatchDelete|CommentSourceContextLifecycle|CommentMutationsFollowIssueTeardownLockOrder)' -count=1 -json`
  exited0 on the assigned database. `/tmp/i1-w12-regression-final.jsonl` contains
  23 passing entries / 14 top-level tests, zero failures and zero skips. All eight
  W12 top-level tests ran, together with identifier deletion, source-context
  lifecycle/attachments, selected-child survivor events, comment teardown lock
  ordering and workspace-scoped VCS cleanup.
- `go vet ./internal/handler ./internal/iteration` exited0;
  `/tmp/i1-w12-vet-final.log` is empty.
- `go test ./internal/iteration` and `go vet ./internal/iteration` exited0.
  `git diff --check` passed for the W12 files.
- Root's independent production review found no W12 blocker. No commits were
  created by this lane; root retains integration and final gate ownership.

Existing `CancelTasksForIssue` and `FailAutopilotRunsByIssue` calls still occur
before the deletion transaction. Their established execution effects are not
part of the rollback claim. This change adds no automatic retry after uncertain
commit results and does not alter execution cancellation policy.

The current fact recorder retains IDs/title/status facts. LG/HG must freeze full
historical identifiers and display names in original_facts at start, and CG must
freeze current summaries in snapshots. Historical readers must not fall back to
live joins after an issue is deleted. W12 does not certify HG or snapshot assembly.
