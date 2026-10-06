# Iteration facts in existing issue transactions

I1 is under implementation. Only the W01 ordinary update family currently
records facts; do not enable capabilities or infer full writer coverage from
the schema. Gate evidence lives in the I1 foundation task.

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

Regression sources: `internal/handler/issue_iteration_test.go`,
`issue_revision_test.go`, `project_association_concurrency_test.go` and
`iteration_write_benchmark_test.go`. Performance thresholds and actual gate
status are in `.trellis/tasks/10-05-iterations-i1-foundation/s0-s1-verification.md`.
