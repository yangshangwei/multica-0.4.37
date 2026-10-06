# Iteration facts in existing issue transactions

I1 is under implementation. W01 ordinary updates and W02 public content updates
record facts; W15 workspace deletion explicitly purges history. Do not enable
capabilities or infer full writer coverage from the schema. Gate evidence lives
in the I1 foundation task.

## Transaction ownership and locks

- Keep the caller's isolation and retry budget. W01 uses its existing `Begin`
  default and four attempts for 55P03; P1 project writes use RR and project
  deletion uses RC. Never nest `iteration.RunTransaction` in those owners.
- Ordinary unassociated issues also acquire the I1 workspace fence before any
  attachment/issue lock. Otherwise a concurrent join can commit unnoticed.
- W01 orders workspace KEY SHARE, subscriber fence, active member SHARE,
  status catalog shared and I1 advisory locks. It pipelines those separate
  SQL statements with `SendBatch`, then consumes and closes every result.
  The subsequent current-iteration lookup is a new statement after the fence
  wait; do not fold it into a pre-wait snapshot or use preflight membership.
- `iteration.WorkspaceFenceSQL` is shared by the individual and batched lock
  paths. The other batched statements mirror authoritative sqlc queries and
  have an equivalence test. Preserve UUID-based lock keys and parameter order.
- Lock the iteration before attachment/issue rows, then participation. Related
  project/member/agent/squad/task references retain SHARE NOWAIT to avoid
  inverse lock waits. A task lending originator invocation permission must
  stay locked through the authorized write; terminal tasks may still edit
  ordinary content but cannot lend new invocation rights.
- The batch consumes queued locks even after a missing member result, so a
  forbidden response may wait until those locks resolve or its context ends.
  No business write follows failed authorization.

## Recorder boundary

`PrepareIssueRecord` takes the locked before issue and locked iteration, resolves
the real status category (including archived custom statuses), locks current
participation and samples database wall time before the business mutation.
`RecordIssueChange` uses the same borrowed `pgx.Tx`. It never begins/commits,
broadcasts or invokes execution.

The current recorder supports unchanged membership in planned/active iterations.
Do not use it as deletion/join/leave/execution-start support without extending
its contract and real-path tests. Ordinary edits retain original facts, current
pointer and cumulative rollover. Description/attachments/revision-only changes
do not manufacture statistical events. Started is sticky only within the
current participation. Event sequence follows the last persisted event;
occurred_at is clamped against that event while sampled_at retains wall time.

Create the event operation identity once outside the owning retry loop; reuse
it and actor identity for every rolled-back attempt. Reauthorize each attempt.
Issue mutation, attachment binding, participation, event and scope revision
must share one commit. No post-commit listener may repair missing facts.

## Public content and workspace deletion

`IssueService.UpdateContent` requires a transaction starter and authorization
callback; it never falls back to autocommit. The callback validates the existing
transport principal under applicable member/installation locks before the I1
fence, then rechecks time-sensitive grants after the issue lock. Keep actor
identity stable. A plugin actor is the installation with null user_id, never a
fabricated member; a member action includes via_plugin_id. The in-memory callback
token check is point-in-time and does not acquire a hold-through-commit grant.

Preserve the public conditional-write contract at both workspace and issue
locking reads: missing rows with ExpectedRevision yield ErrIssueRevisionConflict,
as the former conditional update did. Keep membership-revocation errors distinct.

Only complete workspace deletion calls `DeleteWorkspaceIterationData`. It purges
all seven I1 tables in the existing teardown transaction, after protected inbox
leaf data and before issue roots, under the workspace FOR UPDATE lock. That lock
must wait for current fact writers; subsequent delete statements see their
committed facts. Ordinary issue/project deletion retains iteration history.

The runtime sweeper must use `TaskService.HandleFailedTasks`; its former
test-only fallback was removed because it bypassed real retry eligibility and
the shared writer. Final-failure fixtures explicitly exhaust their retry budget.
`MarkIssueFirstExecuted` remains a lifetime analytics marker, not a participation
start: it must not create iteration events or set current participation started.
Only the actual successful StartAgentTask path can supply execution-start facts.

Regression sources: `internal/handler/issue_iteration_test.go`,
`issue_revision_test.go`, `project_association_concurrency_test.go` and
`iteration_write_benchmark_test.go`. Performance thresholds and actual gate
status are in `.trellis/tasks/10-05-iterations-i1-foundation/s0-s1-verification.md`.
Additional real-path coverage: `plugin_iteration_test.go`,
`workspace_iteration_delete_test.go`, service `issue_public_test.go` and the
foundation `s2-verification.md` ledger.
