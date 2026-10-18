-- Channels are locked after the stable target set, before participants/grants.
-- A channel belongs to the actual root agent, not a target's display role.
-- name: LockCollaborationChannels :many
SELECT * FROM collaboration_channel WHERE session_id=$1 ORDER BY id FOR UPDATE;

-- name: ListCollaborationChannels :many
SELECT * FROM collaboration_channel WHERE session_id=$1 ORDER BY id;

-- name: EnsureCollaborationChannel :one
INSERT INTO collaboration_channel (session_id,root_agent_id)
VALUES ($1,$2) ON CONFLICT(session_id,root_agent_id) DO UPDATE SET root_agent_id=EXCLUDED.root_agent_id RETURNING *;

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

-- Direct-agent batches use their target's enqueue sequence, not transaction time.
-- Mixed/squad batches retain their existing order until channel-wide input
-- sequencing is implemented; their per-target seq values are not comparable.
-- name: ListCollaborationBatchInputs :many
SELECT * FROM collaboration_command WHERE batch_id=$1
ORDER BY CASE WHEN EXISTS (
    SELECT 1 FROM collaboration_batch b
    JOIN collaboration_target t ON t.id=b.effective_target_id
    WHERE b.id=$1 AND t.kind='agent'
) THEN seq END,created_at,seq,id;

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
