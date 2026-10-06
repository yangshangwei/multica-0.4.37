# Iteration facts in existing issue transactions

I1 is under implementation. W01 ordinary updates, W02 public content updates
and W03/W04 system status updates record facts; W15 workspace deletion explicitly
purges history. Do not enable
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

## System status writers

`service.WriteIssueStatus` owns a fresh, explicitly READ COMMITTED transaction
for the formerly autocommit webhook completion and failed-task reset paths. Its
first SQL selects isolation. Do not change W01/P1 isolation or nest this owner
inside an existing transaction. Current production callers use pool-backed
starters. No new retry loop or autocommit fallback is provided.

After workspace/catalog/I1/iteration locks, take the issue FOR UPDATE before
the read-only policy callback. NO KEY UPDATE would not exclude execution's KEY
SHARE locks. Read the active-task predicate in a separate statement after this
lock: an enqueue can leave the issue tuple unchanged, so an old RR snapshot
could miss its committed task. `lock_issue_execution` refuses contention, but
`CreateAgentTask` may first wait inside `lock_task_owner_rows`; do not assume all
enqueue contention immediately returns ErrNoRows or 55P03.

GitHub, Forgejo and GitLab completion recheck terminal state and the combined
close aggregate. Failed-task reset preserves preceding retry/delegated recovery,
then requires current effective in_progress and no active task. Callers publish
only after a committed change, using the locked before state. A system actor
has null id/user_id and a nonempty source, never a fabricated human identity.

Regression sources: `internal/handler/issue_iteration_test.go`,
`issue_revision_test.go`, `project_association_concurrency_test.go` and
`iteration_write_benchmark_test.go`. Performance thresholds and actual gate
status are in `.trellis/tasks/10-05-iterations-i1-foundation/s0-s1-verification.md`.
Additional real-path coverage: `plugin_iteration_test.go`,
`workspace_iteration_delete_test.go`, service `issue_public_test.go` and the
foundation `s2-verification.md` ledger.

## Additional S2 owners and protected recovery

Squad archive and project deletion prepare all participation records before one
batch clock sample and record changed assignee/project facts in the cleanup
transaction. Project deletion refuses malformed cross-workspace legacy FK
references (issue, autopilot and project_resource) before invoking any cleanup.

Ordinary issue deletion locks the selected/direct-child set globally, writes a
single delete/leave event per current participation and retains original facts.
Participant leave time follows the event's clamped time. Pre-existing task
cancellation before issue deletion is a separate effect, not rolled back by a
failed deletion recorder.

Actual StartTask now commits its running transition together with first-start
participation evidence. It takes the I1 fence before issue and task locks,
checks changed task ownership under a NOWAIT lock, and uses database wall time
for task.started_at after the wait. Task enqueue/claim/completion remain distinct.

CreateInTx callers must arrange catalog/I1 fences before source, attachment or
owner locks. Ordinary create also locks its current member before those fences;
agent creator transports explicitly carry ActorUserID, never infer an agent's
owner as the current human. T1 preserves its existing settings-before-member
order; a damaged pending item's iteration association fails closed.

GET iteration-operations holds current workspace/member authorization through
its RC read and looks up only the actor's own request. No client hash or live
entity is required to recover a stored result. Stored references and numeric
counters are validated before returning them to clients. Member revocation
removes protected I1 notifications atomically under the recipient fence.

Generic HTTP/Plugin writes with explicit iteration fields currently return428,
including null; omission preserves current membership. This restriction must
be replaced only by the confirmed lifecycle writer, never silent field dropping.
Full capability and lifecycle gates remain pending; the latest foundation
verification ledgers, not the existence of these helpers, establish readiness.

## FG operation boundary and read pipeline

FG acceptance is recorded in the foundation task's `fg-verification.md`.
`iteration.RunOperation` reauthorizes each bounded RC attempt, serializes under
member/catalog/I1 fences, loads durable requests before mutable-state checks,
and persists the mutation and validated result in one commit. Only the exact
request uniqueness conflict is recoverable; unknown commit responses retain
the old request ID for explicit reconciliation. Settings enable is the current
production integration. The release flag stays off and atomic_handoff remains
false until its later implementation/acceptance.

For W01 with no attachments, current-iteration and issue locking reads share a
second pgx batch only after the existing fence batch and status validation.
They remain separate ordered statements with fresh snapshots. Strict sqlc-model
row mapping and authoritative-query equivalence tests prevent drift. The
attachment path keeps iteration -> attachment -> issue order.

Ordinary HTTP creation fixes its actor/task identity before entering the owner
transaction and revalidates authority inside it. The callback receives a fresh
params copy per retry and derives live provenance only from locked references.
Create, update and lifecycle invocation checks use the actual returned locked
grant set, not a later allow-list read that could observe an unheld new grant.
