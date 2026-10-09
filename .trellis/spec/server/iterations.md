# Iteration facts in existing issue transactions

I1 implements manual iteration management, history and workspace-owned settings.
W01 ordinary updates, W02 public content updates and W03/W04 system status
updates record facts; W15 workspace deletion explicitly purges history. Keep
writer coverage tied to real-path regression evidence, not schema presence.
The original foundation task retains the historical gate evidence; current
availability follows the workspace contract at the end of this document.

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

Explicit generic batch/Plugin iteration writes return428, including null;
omission preserves current membership. Workspace-disabled explicit assignment
returns409 under the existing transaction checks.
Confirmed ordinary HTTP create/update now use the LG borrowed membership
writer described below. The former deployment rollout gate has been retired;
workspace settings and current authorization remain authoritative.

## FG operation boundary and read pipeline

FG acceptance is recorded in the foundation task's `fg-verification.md`.
`iteration.RunOperation` reauthorizes each bounded RC attempt, serializes under
member/catalog/I1 fences, loads durable requests before mutable-state checks,
and persists the mutation and validated result in one commit. Only the exact
request uniqueness conflict is recoverable; unknown commit responses retain
the old request ID for explicit reconciliation. Settings enable and the full lifecycle now share this production integration.
Capabilities advertise manual/atomic_handoff support independently of the
workspace enabled state.

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

## LG/HG lifecycle and history contract

### Scope and trigger

These rules apply when changing period management, ordinary issue assignment,
T1 acceptance or historical reads. CG owns actual close/handoff/disable and
outbox delivery. I1 capability is implemented; workspace enablement remains
default-off. No deployment flag may suppress history, operation recovery or
completed notification outbox work.

### Signatures

- `IterationService.Create`, `Edit`, `Preview`, `Apply` own period operations.
  Writes reuse `iteration.RunOperation`; preview uses bounded fresh RR attempts.
- `PrepareMembershipChange(ctx, tx, before, source, target, allowCompleted)`
  prepares a borrowed write; `CommitMembershipChange(ctx, tx, change, actor,
  operationID, businessAt)` records it in the same caller transaction.
- `CaptureHistoricalIssues`, `LoadHistory`, `BuildSnapshot`, `DecodeSnapshot`
  share the canonical statistics/chart implementation. Capture locks display
  references for writers; reads use one authorized RR view.
- Workspace routes: `GET/POST iterations`, `GET/PUT iterations/{id}`,
  `GET iterations/{id}/issues`, `GET iterations/{id}/events`,
  `POST iteration-previews`, `POST iteration-operations`.

### Request, transaction and response contracts

Period maintenance requires a current human workspace member; settings enable
requires owner/admin. A machine's issue assignment rights do not confer period
maintenance rights. All operation attempts reauthorize under existing fences.
Preview includes the whole affected set and hashes the captured identifier and
live assignee/squad/leader availability. Start emits its marker before baseline
events, fixing original commitment by sequence even at equal timestamps.

Confirmed `POST /issues` requires `expected_iteration_revision` with a nonnull
`current_iteration_id`. A done issue joining active additionally requires
`allow_completed=true`. It preserves ordinary duplicate409 and enqueue behavior.
Membership-only `PUT /issues/{id}` requires `expected_revision`; actual leave or
switch needs `iteration_reason`. It accepts only those fields and optional
`allow_completed`; it cannot mix ordinary status/title/assignee edits.
Rollover is server-owned. Response builders AND handwritten SELECT/Scan paths
must expose real nullable `current_iteration_id` and integer rollover.

T1 accepts optional `fields.current_iteration_id` only for accept and
accept-and-execute. Lock settings/target before pending issue rows. Validate
and lock all final references/grants/labels, project the accepted issue with
its next revision, prepare membership, then sample once before label/admission
mutations. Reuse one action identity across the existing four-attempt owner.
Plain acceptance does not enqueue; execution retains its own idempotent action.
Capability reads inside settings writes must use the transaction-bound queries;
borrowing a second pool connection can deadlock a single-connection pool.

Closed statistics/scope come exclusively from a validated stored snapshot.
`Snapshot.Events` is frozen; `/events` may include later metadata audit.
Destinations must cover all scope items, contain explicit nullable target and
integer counters, and exclude terminal/self rollover. Baseline issue identity
must match its event. Historical names use the stored legacy prefix resolver;
live availability is separate. Never repair bad snapshots from live joins.

### CG closure, bulk writes and notifications

CG evidence is in the I1 parent `cg-verification.md`. End, active cancel,
handoff and whole-workspace disable use the same persistent operation owner.
New workspaces remain disabled until an administrator enables them;
`atomic_handoff` describes implemented support independently of that choice.

Capture frozen history before releasing membership, but stage its payload in
memory until all writes finish. Insert the immutable snapshot exactly once
with final processed time. Preserve the original sampled clock on events;
their occurred time is clamped independently. Normalize injected clocks to PG
microsecond precision so JSON timestamps agree with persisted timestamptz.

Compare the confirmed hash before projecting history: a changed catalog fact
must return stale409 rather than mask the conflict with a projection failure.
Canonical decoding must allocate a fresh draft, never unmarshal into caller-
owned pointers/slices that may alias source or target IDs.

Bulk membership uses ordered pgx batches with affected-row checks. Keep source
leaves before target joins and return every SQL error to the transaction owner.
Transport failure injection must cover SendBatch as well as Exec/Query paths.
The borrowed single-issue writer retains the original caller transaction.

Notifications use the outbox ID as inbox ID; insertion, delivered marking and
current-recipient authorization share one transaction. Revoke removes inbox
content but keeps suppressed outbox tombstones so rejoin cannot reconstruct
old messages or duplicate the same local-day reminder. Whole disable emits
one workspace summary rather than a random first-plan title. Date-edit
recipients are locked and resolved before sampling and writing. Publish
realtime only after committed inbox insertion; reminders never advance periods
or invoke agents.

### Validation and error matrix

| Condition | Result |
| --- | --- |
| Explicit generic batch/plugin iteration writes or client-supplied rollover | 428 |
| Confirmed ordinary or triage assignment while workspace disabled | 409 |
| Missing create target revision or update issue revision | 428 |
| Mixed membership/ordinary edits, malformed acknowledgment | 400 |
| Duplicate raw management keys, Unicode-fold aliases, unknown fields | 400 |
| Stale preview/revision, changed request payload, stale cursor | 409 |
| Affected set exceeds 2,000, or management body exceeds 4 MiB | 413; no partial mutation |
| Membership or current invocation grant revoked | 403; no business mutation |
| NOWAIT lock conflict / RR serialization conflict | Preserve PG error for the owning bounded retry |

### Good, base and bad cases

- Good: preview all 1,000 tasks, confirm its hash, move/start atomically and
  capture every original. Same request returns its stored result without events.
- Base: ordinary issue edit omits iteration fields and preserves membership.
- Bad: treat archived agent display data as current authority, silently truncate
  a preview, or use a later unlocked grant to authorize acceptance.

### Required regression evidence

Use real isolated PostgreSQL with the agent CLI guard. Lifecycle tests cover
single active, complete sets, stale/unauthorized rollback, terminal choices,
reentry, midnight/backclock, replay and unchanged running execution. HTTP/T1
tests cover current grants, one enqueue, duplicate/Unicode envelopes, JWT scope
and workspace-disabled routes. History A–J asserts O8/current9/effective8/completed5/
original_completed4 and 62.5%/50%, including deleted originals and frozen reads.
See lifecycle/history task `verification.md` for commands and gate limits.

### Wrong versus correct

Wrong: decode a management request directly into a Go struct before checking
duplicates; `start_date` and `ſtart_date` can overwrite the same field.
Correct: `ValidateObjectJSON(raw)` before typed `DisallowUnknownFields` decoding.

Wrong: validate the T1 iteration after acceptance SQL and sample again.
Correct: prepare from the final accepted projection and sample before the first
business mutation; use that one sample and transaction for admission and join.

### I1 final acceptance regression boundaries

For partial-up/invalid-index recovery, use the real 550–566 files and registered
`hooksForDirection` through `runMigrations`, not only the cleanup-name map or a
recovery test for another feature. `cmd/migrate/migrate_iteration_recovery_test.go`
forces 553 to fail with two active rows, verifies the INVALID index and three-row
ledger, resolves only its isolated fixture conflict, and proves hook-assisted
recovery to 16 valid/ready/live indexes and 17 ledger rows. A hook-free negative
control must fail; replay must preserve index identities and applied timestamps.
Partial empty down preserves shared `planning_timezone` and ordinary issues;
used I1 down must fail before changing any ledger or index. Run only against an
explicit isolated `DATABASE_URL` via `scripts/go-test-with-agent-cli-guard.sh`.
This is a recovery rehearsal, not authorization to rewrite production history.

`TestIterationHistoryCanonicalThroughLifecycleAndIssueWriters` owns the real
A–J proof, including `parent_issue_id` for the A/B relationship and F leaving
before an ordinary HTTP completion outside the period. Verify A and B each
appear once, D/OD remain 5/4 after F completes, and no source-period event is
added. A comment omitting F from synthetic input is not evidence that a real
external writer leaves the source commitment unchanged.


## Workspace-owned availability

### Scope and trigger

Apply this contract when changing iteration discovery, workspace enable/disable,
ordinary or triage assignment, or the notification runner. The deployment gate
`iterations_i1` and `IterationService.Available` have been removed. Capability,
workspace choice and current authorization are separate facts.

### Signatures

- `GET /api/workspaces/{id}/iteration-capabilities` retains schema version 1.
- `GET /api/workspaces/{id}/iteration-settings` reads the persisted choice and
  effective planning timezone.
- `POST /api/workspaces/{id}/iteration-settings/enable` carries `request_id`,
  `expected_revision` and `confirmed_timezone`.
- Disable uses the existing `iteration-previews` and `iteration-operations`
  routes, complete preview hash and original durable request identity.

### Request, response and environment contracts

Capabilities return implemented `supported/manual/atomic_handoff=true`, with
`enabled` read from workspace settings. Missing settings means disabled/revision
1. There is no migration that changes an existing workspace choice; a leftover
`FF_ITERATIONS_I1` environment value no longer affects these capabilities.

`EnableIterationSettings` retains stable human actor and owner/admin checks in
the transaction. It publishes through `writeIterationResult` only for a newly
committed operation. A stored enable/disable receipt is replayed before mutable
settings checks and never republishes the original event.

Whole-workspace disable retains its complete-set transaction: freeze active
history, cancel planned periods, clear current memberships and persist settings,
receipt and notification outbox atomically. It does not rewrite issue status,
project, assignee, cumulative rollover or running executions. Re-enable never
resurrects closed periods.

### Validation and error matrix

| Condition | Contract |
| --- | --- |
| Missing/false old deployment environment variable | No effect on supported capability or saved workspace choice |
| New workspace / missing settings row | Disabled until explicit administrator enable |
| Ordinary or triage assignment while workspace disabled | Existing workspace-disabled conflict |
| Enable by non-admin or machine credential | Forbidden |
| Stale settings revision/timezone or confirmation hash | 409; require a fresh explicit intent/preview |
| Disable affects more than 2,000 issues | 413; no partial operation |
| Lost response after enable/disable | Query original request, replay identical identity only if absent |
| Old enabled=true row formerly hidden by rollout | Becomes usable; pending outbox delivery may resume |

### Good, base and bad cases

- Good: an administrator explicitly enables the saved timezone; only after the
  transaction commits do other clients refresh their iteration capability.
- Base: a new workspace is supported but disabled, with a discoverable settings
  entry and no generated periods or executions.
- Bad: treat a replayed old enable receipt as a new enable after another client
  disabled the workspace, or infer a missing request's success from live state.

### Tests required

Handler `iteration_settings_test.go` covers absent/false old flags, preserved
choices, current authority, stale revisions/timezones and receipt replay without
repeating a post-commit event. Service lifecycle/closure suites cover the whole
disable transaction, limit rejection, frozen history and unchanged execution.
`iteration_notifications_test.go` covers retained outbox delivery. Real settings
E2E checks sibling-client refresh and recovery using the original request ID.

### Wrong versus correct

Wrong: replace the old deployment flag with a client-owned boolean, weaken human
authorization, or stop delivering already committed outbox work when disabled.
Correct: derive support from the implementation, enabled state from workspace
settings and each operation's authority from its existing transactional fences.
