# Collaboration squad dependencies, input readers and leader order

## 1. Scope / Trigger

Use with [collaboration targets](collaboration-targets.md) when changing M08
member delegation, M11 joined input, run admission, nonlaunch return or final
squad settlement. The implemented service and static Linux tests do not enable
general provider, file/Git, other-OS or complete Web/Desktop delivery.

## 2. Signatures

- `CollaborationService.RuntimeContext`, `CreateRuntimeComment`,
  `RuntimeSquadActivity` use a current run's narrow business capability.
- `GET /api/collaboration/runtime/runs/{runId}/context`
- `POST .../comments` carries a stable `comment_id`, optional `parent_id` and
  `content`; `POST .../squad-activity` carries `event_id`, `outcome`, `reason`.
- `collaboration_group_dependency` freezes each input's target/configuration
  and complete grant set. `collaboration_run.prompt_snapshot` freezes claim input.
- UI execution protocol 5 adds `reader_run_ids` to retained group inputs;
  `unread_inputs` and a queued continuation ID describe unconsumed joins.
- Migration 724 guards input returns. Migrations 725/726 add immutable
  `leader_turn`, group `next_leader_turn` and its concurrent unique index.

## 3. Contracts

Admission freezes each command's own target, configuration and grants. Claim,
permit and business calls validate that frozen set; current replacement grants
cannot authorize an older input. Input authors keep their independent account,
participation and lifecycle epochs. Claim saves its exact prompt once and replay
returns the same prompt.

Only a live run may delegate through a comment to its frozen roster. The comment
and member identify the durable delegation; retries do not create a second run.
Each member and continuation gets independent admission, capacity, nonce and
proof. Replies and completion coalesce into at most one queued leader turn.
The leader ends its turn; waiting group state alone never releases machine
capacity. Context returns the newest 200 comments in chronological order.

An M11 join is waiting until the queued continuation claims it. Before that
boundary, cancellation or invalid authority detaches that input and only its
added dependencies. Once claimed, every run whose immutable prompt contains the
command is a reader, including later members and continuations. A primary
consumer's nonlaunch proof alone is insufficient if another reader may have run.

A continuation input may return while other members continue only after every
reader has settled with sealed `never_launched_proven`, sufficient attempt
coverage and no contradictory started/success fact. Root batch return still
requires the whole group to prove nonlaunch. Returned input retains sequence,
author and content, but moves to an unstarted batch with no current group; an
obsolete author or closed session makes it cancelled. Original memberships,
dependencies, prompts and proof rows remain history. The v5 parser checks the
same complete reader set before accepting an early returned input.

The database allocates `leader_turn` under the group row lock during insertion.
Callers cannot supply or mutate it. Final settlement chooses the last leader
turn, not transaction-start `created_at` or random UUID order. A transaction
begun earlier can acquire the lock and create the final continuation later.

Migration 725 reconstructs active legacy turns only from a unique parent
ancestry chain. Ambiguous active sibling continuations refuse upgrade without
changing state. Settled groups preserve their original result and nullable
historical ordinals. Never invent an order, alter old proofs or clear barriers
to make an upgrade pass. Each new index remains a separate concurrent migration
with an exact invalid-index cleanup entry in the runner.

Private fixtures that clone with `LIKE INCLUDING ALL` must install the real
allocator trigger explicitly and clone all dependency tables. PostgreSQL does
not copy triggers; a missing private table can silently resolve to public and
produce evidence for a different dataset.

## 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Frozen input grant replaced or author epoch obsolete | Deny new admission; preserve historical identity |
| Duplicate comment/member dispatch or queued wakeup | Reuse recorded work; no second run |
| Joined input never claimed | Waiting cancellation/detachment without stopping unrelated members |
| First consumer never launched, later reader may have | Input stays consumed; retain its dependencies |
| All readers proved nonlaunch | Return only eligible input; retain old history and other members |
| Missing/duplicate reader or insufficient proof in v5 | Unavailable execution projection; no inferred authority |
| Older BEGIN creates the last continuation | Its group-allocated turn determines final outcome |
| Caller supplies or edits a leader ordinal | Database rejects the write |
| Active legacy leader order is ambiguous | Atomic migration refusal |
| Settled historical run lacks an ordinal | Retain readable history; never recompute its result |

## 5. Good / Base / Bad Cases

- Good: a never-launched continuation returns one joined input to waiting while
  an unrelated member still runs; the author can cancel it through R45.
- Good: a later member has already read that same input, so the first
  continuation's nonlaunch proof cannot make it waiting again.
- Base: direct-agent FIFO, independent target holds and immutable root history
  keep their existing semantics.
- Bad: reading only `consuming_run_id`, dropping old memberships on return, or
  choosing the final leader with `ORDER BY created_at DESC`.

## 6. Tests Required

- `collaboration_group_dependencies_test.go`: complete frozen grants, replacement
  authority, separate authors and preserved claim prompts.
- `collaboration_squad_boundary_test.go`: later-reader duplication regression,
  early return/cancel, closed/departed authors and unchanged historical rows.
- `collaboration_squad_order_test.go`: a delayed-BEGIN transaction without clock
  edits/sleeps; final outcome follows lock-serialized leader order.
- `collaboration_leader_turn_test.go`: causal active backfill, settled ambiguous
  preservation, active ambiguity refusal and forward-only downgrade boundary.
- Core `execution-squad.test.ts`: every referenced reader exists in the same
  group and all required proofs cover an early return; malformed data fails closed.
- `TestCollaborationSquadCapacityOneRoutesLinux`: actual router/PostgreSQL,
  leader/member/continuation, separate namespace proofs, joined/cancelled input.
- `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux` proves real
  `MaxConcurrentTasks=1` polling with the existing commands, registration,
  router/database and each independent proof. Keep this combined test; a
  sequential Runner harness and an isolated slot test cannot replace it.

## 7. Wrong vs Correct

Wrong: return an input when its primary continuation has nonlaunch proof.

Correct: enumerate every immutable prompt reader and require each reader's
sealed nonlaunch evidence before detaching that input.

Wrong: use a transaction timestamp to identify the last leader, or relax a
production validator because a private fixture omitted its trigger.

Correct: allocate order under the group lock and install the actual database
contracts in the isolated fixture used to verify it.


## Complete catalogue and derived-owner current matrix

`TestCollaborationCatalogueAuthorityMatrix` connects three actual participating
resource owners' consent to one leader and two independently bound member runs.
Registration never substitutes for consent; incomplete or proxy confirmation
never admits the typed squad. A newly invited participant reuses identical
existing per-issue grants without source membership or a new owner signature.
Valid issue-pause, offline, occupied namespace and target-pause selections remain
available and wait; one fresh Resume consumes original inputs in FIFO.

Every dependency freezes all actual grant/owner/configuration/installation/binding
facts. Each derived run retains its own exact provenance and rechecks the entire
frozen resource set. Leader business/daemon credentials do not authorize members.
Changing one member configuration or binding invalidates old selections and
credentials; restoring old values does not restore old consent. Owner Leave
withdraws every direct/squad grant while retaining truthful reservations,
original termination capability, permitted Stop and readable execution/input/run
history. This is database/service evidence; synthetic proof payloads do not
establish a provider process or kernel containment.
