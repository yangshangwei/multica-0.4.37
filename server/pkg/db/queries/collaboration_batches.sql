-- Channels are locked before the stable target set and participants/grants.
-- A channel belongs to the actual root agent, not a target's display role.
-- name: LockCollaborationChannels :many
SELECT * FROM collaboration_channel WHERE session_id=$1 ORDER BY id FOR UPDATE;

-- name: ListCollaborationChannels :many
SELECT * FROM collaboration_channel WHERE session_id=$1 ORDER BY id;

-- name: EnsureCollaborationChannel :one
INSERT INTO collaboration_channel (session_id,root_agent_id)
VALUES ($1,$2) ON CONFLICT(session_id,root_agent_id) DO UPDATE SET root_agent_id=EXCLUDED.root_agent_id RETURNING *;

-- Existing channels are already locked; insert only a missing destination.
-- The source issue/session fence serializes legitimate channel creators.
-- name: CreateCollaborationChannel :one
INSERT INTO collaboration_channel (session_id,root_agent_id) VALUES ($1,$2) RETURNING *;

-- name: BindCollaborationTargetChannel :one
UPDATE collaboration_target t SET channel_id=ch.id,queue_state=ch.queue_state,pause_reason=ch.pause_reason,
    control_version=ch.control_version,group_generation=ch.group_generation,active_group_id=ch.active_group_id
FROM collaboration_channel ch WHERE ch.id=sqlc.arg(channel_id)::uuid AND ch.session_id=sqlc.arg(session_id)::uuid
    AND t.id=sqlc.arg(target_id)::uuid AND t.session_id=ch.session_id RETURNING t.*;

-- name: AllocateCollaborationBatchSequence :one
UPDATE collaboration_channel SET next_batch_seq=next_batch_seq+1 WHERE id=$1 RETURNING (next_batch_seq-1)::bigint AS batch_seq;

-- name: LockCollaborationWaitingBatches :many
SELECT * FROM collaboration_batch WHERE channel_id=$1 AND state='waiting' ORDER BY seq FOR UPDATE;

-- name: CreateCollaborationBatch :one
INSERT INTO collaboration_batch(session_id,channel_id,effective_target_id,seq) VALUES($1,$2,$3,$4) RETURNING *;

-- name: CoalesceCollaborationBatchTarget :one
UPDATE collaboration_batch SET effective_target_id=sqlc.arg(target_id)::uuid
WHERE id=sqlc.arg(batch_id)::uuid AND state='waiting' RETURNING *;

-- Empty waiting batches disappear from the runnable/read projections. Retain
-- their tombstones so removed inputs keep an immutable batch identity.
-- name: CloseEmptyCollaborationBatches :exec
UPDATE collaboration_batch b SET state='cancelled',settled_at=now()
WHERE b.session_id=$1 AND b.state='waiting' AND NOT EXISTS (
    SELECT 1 FROM collaboration_command c WHERE c.batch_id=b.id AND c.state='waiting'
);

-- name: ListCollaborationBatches :many
SELECT * FROM collaboration_batch WHERE session_id=$1 ORDER BY channel_id,seq;

-- name: LockCollaborationBatches :many
SELECT * FROM collaboration_batch WHERE session_id=$1 ORDER BY id FOR UPDATE;

-- A batch is a staging container until promotion. Only channel seq orders input.
-- name: ListCollaborationBatchInputs :many
SELECT * FROM collaboration_command WHERE batch_id=$1
ORDER BY seq,id;

-- name: ListCollaborationChannelWaitingInputs :many
SELECT * FROM collaboration_command WHERE channel_id=$1 AND state='waiting' ORDER BY seq,id;

-- Only unread, unjoined inputs may be moved between staging batches.
-- name: RebatchCollaborationWaitingInputs :execrows
UPDATE collaboration_command c SET batch_id=sqlc.arg(batch_id)::uuid
FROM collaboration_batch old_batch, collaboration_batch new_batch
WHERE c.id=ANY(sqlc.arg(command_ids)::uuid[]) AND c.state='waiting' AND c.group_id IS NULL
  AND old_batch.id=c.batch_id AND old_batch.state='waiting' AND old_batch.started_at IS NULL
  AND new_batch.id=sqlc.arg(batch_id)::uuid AND new_batch.state='waiting' AND new_batch.started_at IS NULL
  AND new_batch.session_id=c.session_id AND new_batch.channel_id=c.channel_id;

-- name: MoveCollaborationWaitingInput :one
UPDATE collaboration_command c SET channel_id=sqlc.arg(channel_id)::uuid,batch_id=sqlc.arg(batch_id)::uuid,seq=sqlc.arg(seq)::bigint
WHERE c.id=sqlc.arg(command_id)::uuid AND c.state='waiting' AND c.group_id IS NULL
RETURNING c.*;

-- Every consumed association is retained independently of a command's next queue placement.
-- name: ListCollaborationGroupInputs :many
SELECT * FROM collaboration_group_input WHERE group_id=$1 ORDER BY seq,command_id;

-- name: ListCollaborationSessionGroupInputs :many
SELECT i.* FROM collaboration_group_input i JOIN collaboration_execution_group g ON g.id=i.group_id
WHERE g.session_id=$1 ORDER BY i.group_id,i.seq,i.command_id;

-- name: ReturnNeverLaunchedCollaborationInput :one
UPDATE collaboration_command c SET state='waiting',reason='never_launched',group_id=NULL,batch_id=sqlc.arg(batch_id)::uuid
WHERE c.id=sqlc.arg(command_id)::uuid AND c.state='in_flight' AND c.group_id=sqlc.arg(group_id)::uuid
RETURNING c.*;

-- name: PauseCollaborationChannel :one
UPDATE collaboration_channel SET queue_state='paused',control_version=control_version+1,pause_reason=sqlc.arg(reason)::text
WHERE id=sqlc.arg(channel_id)::uuid RETURNING *;

-- name: ResumeCollaborationChannel :one
UPDATE collaboration_channel SET queue_state='active',control_version=control_version+1,pause_reason=''
WHERE id=$1 RETURNING *;

-- name: MirrorCollaborationChannelPause :exec
UPDATE collaboration_target SET queue_state=sqlc.arg(queue_state)::text,pause_reason=sqlc.arg(reason)::text,control_version=control_version+1
WHERE channel_id=sqlc.arg(channel_id)::uuid;

-- name: MirrorCollaborationChannelControl :many
UPDATE collaboration_target t SET queue_state=ch.queue_state,pause_reason=ch.pause_reason,
    control_version=ch.control_version,group_generation=ch.group_generation,active_group_id=ch.active_group_id
FROM collaboration_channel ch WHERE ch.id=$1 AND t.channel_id=ch.id RETURNING t.*;

-- name: FinishCollaborationBatch :one
UPDATE collaboration_batch SET state=sqlc.arg(state)::text,settled_at=now()
WHERE id=sqlc.arg(batch_id)::uuid AND state='in_flight' RETURNING *;

-- name: FinishCollaborationBatchInputs :exec
UPDATE collaboration_command SET state=sqlc.arg(state)::text,reason=sqlc.arg(reason)::text
WHERE batch_id=sqlc.arg(batch_id)::uuid AND state='in_flight';

-- name: ClearCollaborationChannelGroup :exec
UPDATE collaboration_channel SET active_group_id=NULL WHERE id=$1 AND active_group_id=$2;
