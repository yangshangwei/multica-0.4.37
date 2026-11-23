# FIFO / Switch schema handoff

Implementation owner: `fifo_switch`. Migration, generated sqlc and projection owners
remain the main session / `core_fifo_switch`. This is a design handoff, not executed SQL.

## Persistence contract

```sql
ALTER TABLE collaboration_channel ADD COLUMN next_command_seq bigint NOT NULL DEFAULT 1
    CHECK (next_command_seq > 0);
ALTER TABLE collaboration_command ADD COLUMN group_id uuid;
CREATE TABLE collaboration_group_input (
    group_id uuid NOT NULL,
    command_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    batch_id uuid NOT NULL,
    seq bigint NOT NULL CHECK (seq > 0),
    consuming_run_id uuid NOT NULL
);
```

The new ledger is append-only for this slice. Each row records a command actually
consumed by the group's root run, in frozen order. Its immutable command row remains
the authority for target, input event, content/attachments, author and enqueue epochs.
The retained row survives command return to waiting, new batching and Switch. No
unread/M11 insertion or continuation launch is implemented by this schema alone.

Concurrent, single-statement indexes:

- UNIQUE command `(channel_id, seq)` replaces the old UNIQUE `(target_id, seq)`.
- command `(channel_id, state, seq)` supports ordered waiting selection.
- UNIQUE group_input `(group_id, command_id)` prevents duplicate consumption in a group.
- group_input `(command_id, group_id)` supports retained history/requeue validation.
- Remove the partial UNIQUE waiting batch `(channel_id, effective_target_id)`:
  multiple same-target waiting segments can exist behind a second distinct squad.

Keep group/run/batch identity and existing unsettled-per-channel uniqueness. Do not
delete the historical ledger while applying a forward migration. Guarded down must
refuse retained new state, including group_input rows and transferred commands.

## Guard changes

`collaboration_preserve_execution_identity` must continue rejecting edits to command
id/session/target/input_event/author/participant/account/lifecycle/content/attachments.
It may change command seq only together with a waiting-to-waiting Switch channel
change, when both old/new group_id are NULL, the old batch has never started, the
destination batch is waiting/unstarted, and the target now belongs to the destination
channel with switch_hold. Target/session, root-agent binding and destination allocated
seq are validated. Rebatching within the channel never changes seq.

`collaboration_preserve_batch_input` keeps group/run channel and batch immutable.
For commands, allow only:

1. Unjoined waiting -> waiting rebatching within the same channel, old/new batch
   waiting and never started, new batch matching command session/channel. This can
   split an old mixed batch without moving the held input or rewriting any content.
2. The Switch movement described above. Target update, binding and ordered input
   movement happen in one source transaction; old lane state is unchanged.
3. Proven-never-launched in_flight -> waiting (or cancelled for invalid author/closed
   session), clearing group_id and moving to a new waiting batch at the same seq.
   Require a retained row for the old group/command/batch/channel/seq, a settled and
   sealed old group, and valid settled never_launched_proven consuming run with
   sealed_attempt_seq >= authorized_attempt_seq, no started_at/success contradiction.
   For the current direct-root slice also require every group run to be proven never
   launched; one unstarted descendant cannot release a consumed parent input.
   A stale account/participant/lifecycle author or closed session cannot return waiting.

Current group_id can be assigned only while the waiting input transitions to
in_flight and belongs to the new active batch/group; keep that association on normal
terminal settlement. Clear it only in rule 3. No waiting -> in_flight assignment to
a second existing group. Retained ledger insertion checks group/run/command identity,
source session, batch/channel and sequence under the shared source locks. No UPDATE
or DELETE of an existing ledger row in normal writers.

## Migration backfill

Freeze existing consumed group membership before enabling any mutable batch guards.
Current real runtime execution is direct-agent only, so its existing claimed order is
the target seq, not transaction-start created_at. Capture all consumed inputs, not
only group.command_id; exclude cancelled waiting tombstones that were never started.
Do not infer consumption solely from batch_id for a cancelled tombstone. Preserve
actual group/run IDs, forced dispositions and every proof/capability identity.

For channel seq, retained per-target sequences are a partial order; old transaction
timestamps cannot reconstruct mixed channel lock acquisition. Choose and document a
deterministic linear extension preserving each target's seq and each real consumed
batch order. Initialize next_command_seq beyond every assigned seq. Historical
group_input seq must reflect the preserved consumed ordering; later Switch only
changes current command position. The previous target.next_command_seq may remain
as unused storage initially, but the projection reads channel.next_command_seq.

## Query ownership / intended calls

The implementation owner will add queries for all waiting commands ordered by
channel seq (including non-null group_id so the selector can explicitly skip joins),
selected command rebatching, channel seq allocation, root group-input capture,
retained group inputs, and proven-never-launched input return. `StartBatchInputs`
sets the current command.group_id from the started batch and captures membership in
the same transaction as root run creation. Promotion uses individual eligibility;
waiting staging batches are never an authority for which commands start.

Switch needs target rebind plus per-command move queries. The queue entry ensures a
missing destination channel before taking all channels in UUID order, then rechecks
under source/target/resource locks. New-channel uniqueness conflicts become a full
transaction retry; uncertain commits continue to require original-key recovery.

## Wire handoff

Proposed required fields: channel.next_command_seq, command.group_id (nullable), and
groups[].inputs[] = {command_id, channel_id, batch_id, sequence, consuming_run_id}.
The current command placement validates against its current channel. A historical
group validates against its retained input ledger and run proof, never against the
current command's queue placement after requeue/Switch. Current root representative
continues to be an input of the effective target; mixed squad launch remains gated.

## Final implementation alignment

The handoff above preceded implementation. Migrations 699–708 and the current
server spec are the executable contract. The obsolete target counter was
removed; channel sequence is the only queue authority. Existing channels are
locked before inserting a missing destination. A group-input row retains the
original consuming root; updates and deletion while its group exists are refused.
The old group-command unique index was replaced so complete nonlaunch can return
the same input and later create a different group. E03 checks complete settled
proof for every run, including progress and seal/attempt boundaries.

`ReconfigureTarget` remains a held pending-configuration stage; completed,
consent-ready atomic Switch and unread M11 membership remain separate work.
