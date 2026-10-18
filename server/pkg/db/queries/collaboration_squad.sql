-- name: CreateCollaborationDerivedRun :one
INSERT INTO collaboration_run(session_id,target_id,group_id,command_id,parent_run_id,step_key,step_kind,agent_id,target_epoch,config_version,
    grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch,
    channel_id,batch_id,deployment_id,resource_owner_user_id,progress_state)
VALUES(sqlc.arg(session_id)::uuid,sqlc.arg(target_id)::uuid,sqlc.arg(group_id)::uuid,sqlc.arg(command_id)::uuid,
    sqlc.arg(parent_run_id)::uuid,sqlc.arg(step_key)::text,sqlc.arg(step_kind)::text,
    sqlc.arg(agent_id)::uuid,sqlc.arg(target_epoch)::bigint,sqlc.arg(config_version)::bigint,sqlc.arg(grant_id)::uuid,sqlc.arg(grant_epoch)::bigint,
    sqlc.arg(resource_epoch)::bigint,sqlc.arg(execution_workspace_id)::uuid,sqlc.arg(runtime_id)::uuid,sqlc.arg(installation_id)::uuid,
    sqlc.arg(binding_id)::uuid,sqlc.arg(binding_epoch)::bigint,sqlc.arg(channel_id)::uuid,sqlc.arg(batch_id)::uuid,
    sqlc.arg(deployment_id)::uuid,sqlc.arg(resource_owner_user_id)::uuid,'prepared') RETURNING *;

-- name: GetCollaborationQueuedContinuation :one
SELECT * FROM collaboration_run WHERE group_id=$1 AND step_kind='continuation' AND progress_state='prepared'
    AND settled_at IS NULL AND forced_disposition IS NULL;

-- name: CreateCollaborationCommentDelegation :exec
INSERT INTO collaboration_delegation(group_id,parent_run_id,request_key,payload_hash,phase_key,members,continuation_key,
    source_comment_id,member_agent_id,run_id,state)
VALUES(sqlc.arg(group_id)::uuid,sqlc.arg(parent_run_id)::uuid,sqlc.arg(request_key)::text,sqlc.arg(payload_hash)::text,
    'comment',jsonb_build_array(sqlc.arg(member_agent_id)::uuid),'leader',
    sqlc.arg(source_comment_id)::uuid,sqlc.arg(member_agent_id)::uuid,sqlc.arg(run_id)::uuid,'waiting');

-- name: SettleCollaborationRunDelegations :exec
UPDATE collaboration_delegation SET state='settled' WHERE run_id=$1 AND state='waiting';

-- name: GetCollaborationRuntimeReservation :one
SELECT * FROM collaboration_runtime_reservation WHERE run_id=$1;

-- name: GetCollaborationBusinessRunByHash :one
SELECT run_id FROM collaboration_business_capability WHERE token_hash=$1 AND revoked_at IS NULL;

-- name: CreateCollaborationRuntimeComment :one
INSERT INTO comment(id,issue_id,workspace_id,author_type,author_id,parent_id,content,dispatch_eligible)
VALUES($1,$2,$3,'agent',$4,$5,$6,false) RETURNING *;

-- name: CreateCollaborationBusinessEvent :one
INSERT INTO collaboration_run_event(issue_id,run_id,producer_event_id,seq,kind,payload)
VALUES(sqlc.arg(issue_id)::uuid,sqlc.arg(run_id)::uuid,sqlc.arg(event_id)::text,
    (SELECT COALESCE(max(seq),0)+1 FROM collaboration_run_event WHERE run_id=sqlc.arg(run_id)::uuid),
    sqlc.arg(kind)::text,sqlc.arg(payload)::jsonb) RETURNING *;

-- name: ListCollaborationRuntimeComments :many
SELECT c.id,c.author_type,c.author_id,c.content,c.parent_id,c.created_at FROM comment c
WHERE c.id IN (
    SELECT recent.id FROM comment recent
    WHERE recent.issue_id=$1 AND recent.deleted_at IS NULL
    ORDER BY recent.created_at DESC,recent.id DESC LIMIT 200
)
ORDER BY c.created_at,c.id;

-- name: ScheduleCollaborationSquadWakeup :exec
UPDATE collaboration_execution_group SET wake_pending=true,wake_parent_run_id=$2
WHERE id=$1 AND settled_at IS NULL AND expansion_sealed_at IS NULL;

-- name: ClearCollaborationSquadWakeup :exec
UPDATE collaboration_execution_group SET wake_pending=false WHERE id=$1;

-- name: ListCollaborationSquadWakeups :many
SELECT g.wake_parent_run_id FROM collaboration_execution_group g
JOIN collaboration_target t ON t.id=g.target_id AND t.kind='squad' AND t.hold_reason IS NULL
JOIN collaboration_grant grant_row ON grant_row.target_id=t.id AND grant_row.role='leader'
  AND grant_row.config_version=g.config_version AND grant_row.target_epoch=g.target_epoch AND grant_row.revoked_at IS NULL
WHERE g.wake_pending AND g.settled_at IS NULL AND g.expansion_sealed_at IS NULL
  AND grant_row.runtime_id=$1 AND grant_row.binding_id=$2
ORDER BY g.created_at,g.id LIMIT 16;

-- The source and channel locks serialize joining against FIFO and settlement.
-- name: JoinCollaborationUnreadInput :one
UPDATE collaboration_command SET group_id=sqlc.arg(group_id)::uuid,batch_id=sqlc.arg(batch_id)::uuid,reason='joined_squad'
WHERE id=sqlc.arg(command_id)::uuid AND state='waiting' AND group_id IS NULL RETURNING *;

-- name: DetachCollaborationGroupDependency :exec
UPDATE collaboration_group_dependency SET detached_at=now(),detach_reason=sqlc.arg(reason)::text
WHERE group_id=sqlc.arg(group_id)::uuid AND command_id=sqlc.arg(command_id)::uuid AND detached_at IS NULL;

-- name: DetachCollaborationUnreadInput :one
UPDATE collaboration_command SET group_id=NULL,batch_id=sqlc.arg(batch_id)::uuid,state=sqlc.arg(state)::text,reason=sqlc.arg(reason)::text
WHERE id=sqlc.arg(command_id)::uuid AND group_id=sqlc.arg(group_id)::uuid AND state='waiting' RETURNING *;

-- name: ConsumeCollaborationJoinedInputs :exec
WITH consumed AS (
    UPDATE collaboration_command SET state='in_flight'
    WHERE group_id=sqlc.arg(group_id)::uuid AND state='waiting' RETURNING *
)
INSERT INTO collaboration_group_input(group_id,command_id,channel_id,batch_id,seq,consuming_run_id)
SELECT group_id,id,channel_id,batch_id,seq,sqlc.arg(run_id)::uuid FROM consumed;

-- name: ListCollaborationGroupCommands :many
SELECT * FROM collaboration_command WHERE group_id=$1 ORDER BY seq,id;
