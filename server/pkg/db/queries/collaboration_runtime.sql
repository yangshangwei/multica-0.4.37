-- name: GetCollaborationRuntimeRun :one
SELECT * FROM collaboration_run WHERE id=$1;

-- name: GetCollaborationSessionByID :one
SELECT * FROM collaboration_session WHERE id=$1;

-- name: GetCollaborationGrant :one
SELECT * FROM collaboration_grant WHERE id=$1;

-- name: GetCollaborationBatch :one
SELECT * FROM collaboration_batch WHERE id=$1;

-- name: GetCollaborationChannel :one
SELECT * FROM collaboration_channel WHERE id=$1;

-- name: LockCollaborationRuntimeRun :one
SELECT * FROM collaboration_run WHERE id=$1 FOR UPDATE;

-- name: LockCollaborationRuntimeGroup :one
SELECT * FROM collaboration_execution_group WHERE id=$1 FOR UPDATE;

-- name: GetCollaborationTermination :one
SELECT * FROM collaboration_termination WHERE run_id=$1;

-- name: LockCollaborationTermination :one
SELECT * FROM collaboration_termination WHERE run_id=$1 FOR UPDATE;

-- name: CreateCollaborationTerminationCapability :one
INSERT INTO collaboration_termination(issue_id,run_id,fence,launch_nonce,deployment_id,installation_id,identity_kind,public_key,token_hash,runtime_identity,token_ciphertext)
VALUES($1,$2,$3,$4,$5,$6,'managed',$7,$8,$9,$10) RETURNING *;

-- All three candidate queries share this eligibility predicate. The priority
-- lookup covers the complete scope, independently of any pagination position.
-- name: GetCollaborationRuntimeCandidatePriority :one
SELECT (CASE WHEN EXISTS (
    SELECT 1 FROM collaboration_runtime_reservation reserve WHERE reserve.run_id=r.id AND reserve.released_at IS NULL
) THEN 0 ELSE 2 END + CASE WHEN r.step_kind='continuation' THEN 1 ELSE 0 END)::integer AS priority
FROM collaboration_run r JOIN collaboration_grant g ON g.id=r.grant_id AND g.revoked_at IS NULL
JOIN collaboration_target t ON t.id=r.target_id AND t.session_id=r.session_id AND t.hold_reason IS NULL
JOIN issue_participant p ON p.issue_id=g.issue_id AND p.user_id=g.owner_user_id AND p.revoked_at IS NULL AND p.epoch=g.owner_participant_epoch
JOIN collaboration_account_fence f ON f.user_id=g.owner_user_id AND f.epoch=g.owner_auth_epoch
WHERE r.runtime_id=$1 AND r.binding_id=$2 AND r.settled_at IS NULL AND r.forced_disposition IS NULL
  AND NOT EXISTS (SELECT 1 FROM collaboration_command c
      LEFT JOIN collaboration_target dependency ON dependency.id=c.target_id AND dependency.session_id=c.session_id
      WHERE c.batch_id=r.batch_id AND c.state='in_flight' AND (dependency.id IS NULL OR dependency.hold_reason IS NOT NULL))
ORDER BY priority LIMIT 1;

-- Recheck the highest tier in the page snapshot too: a reservation may appear
-- while a poller is waiting for the selected tier's cursor lock. An empty old
-- tier must not be filled with lower-priority rows, including during wrap. A
-- daemon attempts one run per dispatch, so persist only one offered position;
-- advancing a larger page would repeatedly skip its unattempted suffix.
-- name: ListCollaborationRuntimeCandidates :many
WITH candidates AS (
SELECT r.id,r.created_at,(CASE WHEN EXISTS (
    SELECT 1 FROM collaboration_runtime_reservation reserve WHERE reserve.run_id=r.id AND reserve.released_at IS NULL
) THEN 0 ELSE 2 END + CASE WHEN r.step_kind='continuation' THEN 1 ELSE 0 END)::integer AS priority
FROM collaboration_run r JOIN collaboration_grant g ON g.id=r.grant_id AND g.revoked_at IS NULL
JOIN collaboration_target t ON t.id=r.target_id AND t.session_id=r.session_id AND t.hold_reason IS NULL
JOIN issue_participant p ON p.issue_id=g.issue_id AND p.user_id=g.owner_user_id AND p.revoked_at IS NULL AND p.epoch=g.owner_participant_epoch
JOIN collaboration_account_fence f ON f.user_id=g.owner_user_id AND f.epoch=g.owner_auth_epoch
WHERE r.runtime_id=sqlc.arg(runtime_id)::uuid AND r.binding_id=sqlc.arg(binding_id)::uuid AND r.settled_at IS NULL AND r.forced_disposition IS NULL
  AND NOT EXISTS (SELECT 1 FROM collaboration_command c
      LEFT JOIN collaboration_target dependency ON dependency.id=c.target_id AND dependency.session_id=c.session_id
      WHERE c.batch_id=r.batch_id AND c.state='in_flight' AND (dependency.id IS NULL OR dependency.hold_reason IS NOT NULL))
)
SELECT id,created_at,priority FROM candidates
WHERE priority=sqlc.arg(priority)::integer AND priority=(SELECT min(priority) FROM candidates)
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (created_at,id)>(sqlc.narg(after_created_at)::timestamptz,sqlc.narg(after_id)::uuid))
ORDER BY created_at,id LIMIT 1;

-- The existence of pending unreserved work is independent of candidate pages
-- and priority cursors. Revoked, held, forced and settled rows stay excluded.
-- name: HasCollaborationRuntimePendingDerivedRun :one
SELECT EXISTS (
SELECT 1
FROM collaboration_run r JOIN collaboration_grant g ON g.id=r.grant_id AND g.revoked_at IS NULL
JOIN collaboration_target t ON t.id=r.target_id AND t.session_id=r.session_id AND t.hold_reason IS NULL
JOIN issue_participant p ON p.issue_id=g.issue_id AND p.user_id=g.owner_user_id AND p.revoked_at IS NULL AND p.epoch=g.owner_participant_epoch
JOIN collaboration_account_fence f ON f.user_id=g.owner_user_id AND f.epoch=g.owner_auth_epoch
WHERE r.runtime_id=$1 AND r.binding_id=$2 AND r.settled_at IS NULL AND r.forced_disposition IS NULL
  AND NOT EXISTS (SELECT 1 FROM collaboration_command c
      LEFT JOIN collaboration_target dependency ON dependency.id=c.target_id AND dependency.session_id=c.session_id
      WHERE c.batch_id=r.batch_id AND c.state='in_flight' AND (dependency.id IS NULL OR dependency.hold_reason IS NOT NULL))
  AND NOT EXISTS (SELECT 1 FROM collaboration_runtime_reservation reserve WHERE reserve.run_id=r.id AND reserve.released_at IS NULL)
)::boolean;

-- name: ListCollaborationRuntimeWaitingTargets :many
SELECT DISTINCT t.id,t.session_id,s.issue_id FROM collaboration_target t
JOIN collaboration_session s ON s.id=t.session_id
JOIN collaboration_channel ch ON ch.id=t.channel_id
JOIN collaboration_command c ON c.channel_id=ch.id AND c.target_id=t.id AND c.state='waiting' AND c.group_id IS NULL
JOIN collaboration_grant g ON g.target_id=t.id AND g.config_version=t.config_version AND g.target_epoch=t.target_epoch AND g.revoked_at IS NULL
WHERE g.role IN ('agent','leader') AND t.hold_reason IS NULL AND g.runtime_id=sqlc.arg(runtime_id)::uuid AND g.binding_id=sqlc.arg(binding_id)::uuid AND s.queue_state='active' AND ch.queue_state='active'
  AND (sqlc.narg(after_issue_id)::uuid IS NULL OR (s.issue_id,t.id)>(sqlc.narg(after_issue_id)::uuid,sqlc.narg(after_id)::uuid))
ORDER BY s.issue_id,t.id LIMIT 16;

-- name: ListCollaborationRuntimeWakeups :many
SELECT DISTINCT g.id,g.created_at,g.wake_parent_run_id FROM collaboration_execution_group g
JOIN collaboration_target t ON t.id=g.target_id AND t.kind='squad' AND t.hold_reason IS NULL
JOIN collaboration_grant grant_row ON grant_row.target_id=t.id AND grant_row.role='leader'
  AND grant_row.config_version=g.config_version AND grant_row.target_epoch=g.target_epoch AND grant_row.revoked_at IS NULL
WHERE g.wake_pending AND g.settled_at IS NULL AND g.expansion_sealed_at IS NULL
  AND grant_row.runtime_id=sqlc.arg(runtime_id)::uuid AND grant_row.binding_id=sqlc.arg(binding_id)::uuid
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (g.created_at,g.id)>(sqlc.narg(after_created_at)::timestamptz,sqlc.narg(after_id)::uuid))
ORDER BY g.created_at,g.id LIMIT 16;

-- name: EnsureCollaborationRuntimeScanCursor :exec
INSERT INTO collaboration_runtime_scan_cursor(runtime_id,binding_id,scan_kind) VALUES($1,$2,$3)
ON CONFLICT(runtime_id,binding_id,scan_kind) DO NOTHING;

-- name: LockCollaborationRuntimeScanCursor :one
SELECT * FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1 AND binding_id=$2 AND scan_kind=$3 FOR UPDATE;

-- name: AdvanceCollaborationRuntimeScanCursor :exec
UPDATE collaboration_runtime_scan_cursor
SET after_issue_id=sqlc.narg(after_issue_id)::uuid,after_created_at=sqlc.narg(after_created_at)::timestamptz,after_id=sqlc.narg(after_id)::uuid
WHERE runtime_id=sqlc.arg(runtime_id)::uuid AND binding_id=sqlc.arg(binding_id)::uuid AND scan_kind=sqlc.arg(scan_kind)::text;

-- M09 uses exclusive namespace occupancy because no authoritative server-side
-- machine capacity exists yet. It is not a multi-target capacity declaration.
-- name: CollaborationRuntimeNamespaceOccupied :one
SELECT (EXISTS (
    SELECT 1 FROM agent_task_queue task JOIN agent_runtime r ON r.id=task.runtime_id
    WHERE r.workspace_id=sqlc.arg(workspace_id)::uuid AND r.daemon_id=sqlc.arg(daemon_id)::text
      AND task.status IN ('preparing','dispatched','running','waiting','waiting_local_directory')
) OR EXISTS (
    SELECT 1 FROM collaboration_runtime_reservation reserve
    WHERE reserve.execution_workspace_id=sqlc.arg(workspace_id)::uuid AND reserve.daemon_id=sqlc.arg(daemon_id)::text AND reserve.released_at IS NULL
))::boolean AS occupied;

-- name: CreateCollaborationRuntimeGroup :one
INSERT INTO collaboration_execution_group(session_id,target_id,command_id,generation,target_epoch,config_version,roster,channel_id,batch_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;

-- name: CreateCollaborationRuntimeRun :one
INSERT INTO collaboration_run(session_id,target_id,group_id,command_id,step_key,step_kind,agent_id,target_epoch,config_version,
    grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch,
    channel_id,batch_id,deployment_id,resource_owner_user_id,progress_state)
VALUES(sqlc.arg(session_id)::uuid,sqlc.arg(target_id)::uuid,sqlc.arg(group_id)::uuid,sqlc.arg(command_id)::uuid,'root',sqlc.arg(step_kind)::text,
    sqlc.arg(agent_id)::uuid,sqlc.arg(target_epoch)::bigint,sqlc.arg(config_version)::bigint,sqlc.arg(grant_id)::uuid,sqlc.arg(grant_epoch)::bigint,
    sqlc.arg(resource_epoch)::bigint,sqlc.arg(execution_workspace_id)::uuid,sqlc.arg(runtime_id)::uuid,sqlc.arg(installation_id)::uuid,
    sqlc.arg(binding_id)::uuid,sqlc.arg(binding_epoch)::bigint,sqlc.arg(channel_id)::uuid,sqlc.arg(batch_id)::uuid,
    sqlc.arg(deployment_id)::uuid,sqlc.arg(resource_owner_user_id)::uuid,'prepared') RETURNING *;

-- name: CreateCollaborationRuntimeReservation :exec
INSERT INTO collaboration_runtime_reservation(run_id,execution_workspace_id,agent_id,runtime_id,daemon_id,installation_id,binding_id)
VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: StartCollaborationBatch :one
UPDATE collaboration_batch SET state='in_flight',root_command_id=sqlc.arg(command_id)::uuid,group_id=sqlc.arg(group_id)::uuid,started_at=now()
WHERE id=sqlc.arg(batch_id)::uuid AND state='waiting' RETURNING *;

-- name: StartCollaborationBatchInputs :exec
WITH consumed AS (
    UPDATE collaboration_command c SET state='in_flight',group_id=b.group_id
    FROM collaboration_batch b WHERE c.batch_id=$1 AND b.id=c.batch_id AND b.state='in_flight'
      AND c.state='waiting' AND c.group_id IS NULL
    RETURNING c.*
)
INSERT INTO collaboration_group_input(group_id,command_id,channel_id,batch_id,seq,consuming_run_id)
SELECT c.group_id,c.id,c.channel_id,c.batch_id,c.seq,r.id FROM consumed c
JOIN collaboration_run r ON r.group_id=c.group_id AND r.parent_run_id IS NULL AND r.step_key='root';

-- name: AssignCollaborationChannelGroup :one
UPDATE collaboration_channel SET active_group_id=sqlc.arg(group_id)::uuid,group_generation=group_generation+1
WHERE id=sqlc.arg(channel_id)::uuid AND active_group_id IS NULL RETURNING *;

-- name: AssignCollaborationChannelTargetGroups :exec
UPDATE collaboration_target SET active_group_id=sqlc.arg(group_id)::uuid,group_generation=sqlc.arg(group_generation)::bigint
WHERE channel_id=sqlc.arg(channel_id)::uuid;

-- name: ClaimCollaborationRuntimeRun :one
UPDATE collaboration_run SET progress_state=CASE WHEN progress_state='prepared' THEN 'preparing' ELSE progress_state END,
    containment=sqlc.arg(containment)::text
WHERE id=sqlc.arg(run_id)::uuid AND settled_at IS NULL AND forced_disposition IS NULL RETURNING *;

-- name: GetCollaborationAttempt :one
SELECT * FROM collaboration_attempt WHERE run_id=$1 AND attempt_seq=$2;

-- name: CreateCollaborationAttempt :exec
INSERT INTO collaboration_attempt(run_id,attempt_seq,attempt_nonce,claim_generation,containment) VALUES($1,$2,$3,$4,$5);

-- name: CreateCollaborationHostAttempt :exec
INSERT INTO collaboration_attempt(run_id,attempt_seq,attempt_nonce,claim_generation,containment,launch_descriptor,terminal_observation,authorized_at)
VALUES(sqlc.arg(run_id)::uuid,sqlc.arg(attempt_seq)::bigint,sqlc.arg(attempt_nonce)::text,sqlc.arg(claim_generation)::bigint,
    'host_process_supervision_v1',sqlc.narg(launch_descriptor)::jsonb,sqlc.narg(terminal_observation)::jsonb,
    CASE WHEN sqlc.narg(launch_descriptor)::jsonb IS NULL THEN NULL ELSE now() END);

-- name: ListCollaborationAttempts :many
SELECT * FROM collaboration_attempt WHERE run_id=$1 ORDER BY attempt_seq;

-- name: RecordCollaborationLaunchTermination :one
UPDATE collaboration_attempt SET terminal_observation=sqlc.arg(observation)::jsonb
WHERE run_id=sqlc.arg(run_id)::uuid AND attempt_seq=sqlc.arg(attempt_seq)::bigint AND terminal_observation IS NULL RETURNING *;

-- name: AuthorizeCollaborationAttempt :one
UPDATE collaboration_run SET authorized_attempt_seq=sqlc.arg(attempt_seq)::bigint,progress_state='launch_authorized'
WHERE id=sqlc.arg(run_id)::uuid AND authorized_attempt_seq=0 AND settled_at IS NULL AND forced_disposition IS NULL RETURNING *;

-- name: AuthorizeCollaborationHostAttempt :one
UPDATE collaboration_run SET authorized_attempt_seq=sqlc.arg(attempt_seq)::bigint,progress_state='launch_authorized'
WHERE id=sqlc.arg(run_id)::uuid AND containment='host_process_supervision_v1'
    AND authorized_attempt_seq=sqlc.arg(attempt_seq)::bigint-1 AND settled_at IS NULL AND sealed_attempt_seq IS NULL
    AND forced_disposition IS NULL RETURNING *;

-- name: GetCollaborationBusinessCapability :one
SELECT * FROM collaboration_business_capability WHERE run_id=$1 AND attempt_seq=$2;

-- name: GetCollaborationBusinessCapabilityByHash :one
SELECT * FROM collaboration_business_capability WHERE run_id=$1 AND token_hash=$2;

-- name: CreateCollaborationBusinessCapability :exec
INSERT INTO collaboration_business_capability(run_id,attempt_seq,token_hash,token_ciphertext) VALUES($1,$2,$3,$4);

-- name: RevokeCollaborationRunBusinessCapabilities :exec
UPDATE collaboration_business_capability SET revoked_at=COALESCE(revoked_at,now()) WHERE run_id=$1;

-- name: GetCollaborationRuntimeEvent :one
SELECT * FROM collaboration_run_event WHERE run_id=$1 AND producer_event_id=$2;

-- name: CreateCollaborationRuntimeOutcomeEvent :one
INSERT INTO collaboration_run_event(issue_id,run_id,producer_event_id,seq,kind,payload)
VALUES(sqlc.arg(issue_id)::uuid,sqlc.arg(run_id)::uuid,sqlc.arg(event_id)::text,
    (SELECT COALESCE(max(seq),0)+1 FROM collaboration_run_event WHERE run_id=sqlc.arg(run_id)::uuid),'outcome',sqlc.arg(payload)::jsonb) RETURNING *;

-- name: RecordCollaborationRuntimeOutcome :one
UPDATE collaboration_run SET business_outcome=sqlc.arg(outcome)::text
WHERE id=sqlc.arg(run_id)::uuid AND business_outcome IS NULL AND settled_at IS NULL RETURNING *;

-- name: RecordCollaborationRuntimeProof :one
UPDATE collaboration_run SET proof_state=sqlc.arg(proof_state)::text,sealed_attempt_seq=sqlc.arg(sealed_attempt_seq)::bigint,
    recording_incomplete=(recording_incomplete OR business_outcome IS NULL),progress_state='settled',settled_at=now()
WHERE id=sqlc.arg(run_id)::uuid AND settled_at IS NULL RETURNING *;

-- name: RecordCollaborationTerminationProof :one
UPDATE collaboration_termination SET receipt_id=sqlc.arg(receipt_id)::text,receipt_digest=sqlc.arg(receipt_digest)::text,
    proof=sqlc.arg(proof)::jsonb,confirmed_at=now()
WHERE run_id=sqlc.arg(run_id)::uuid AND confirmed_at IS NULL RETURNING *;

-- name: SealCollaborationCompletedGroup :one
UPDATE collaboration_execution_group SET expansion_sealed_at=COALESCE(expansion_sealed_at,now()),state='ending'
WHERE id=$1 AND settled_at IS NULL RETURNING *;

-- name: ResumeCollaborationSession :one
UPDATE collaboration_session SET queue_state='active',pause_reason='',control_version=control_version+CASE WHEN queue_state='paused' THEN 1 ELSE 0 END
WHERE id=$1 AND queue_state<>'closed' RETURNING *;

-- name: SettleCollaborationProvenGroup :one
UPDATE collaboration_execution_group g SET settled_at=now(),
    state=COALESCE(g.forced_disposition,CASE WHEN (
        SELECT r.business_outcome FROM collaboration_run r WHERE r.group_id=g.id AND r.step_kind IN ('agent','leader','continuation')
        ORDER BY r.leader_turn DESC NULLS LAST LIMIT 1
    )='success' THEN 'succeeded' ELSE 'failed' END)
WHERE g.id=$1 AND g.settled_at IS NULL AND g.expansion_sealed_at IS NOT NULL
  AND EXISTS(SELECT 1 FROM collaboration_run r WHERE r.group_id=g.id)
  AND NOT EXISTS(SELECT 1 FROM collaboration_run r WHERE r.group_id=g.id AND (
      NOT collaboration_managed_execution_ended(r.containment,r.proof_state,r.progress_state,r.settled_at,r.authorized_attempt_seq,r.sealed_attempt_seq,r.started_at,r.business_outcome)
      OR r.step_kind IN ('agent','leader','continuation') AND r.leader_turn IS NULL
      OR r.session_id<>g.session_id OR r.target_id<>g.target_id OR r.command_id<>g.command_id
      OR r.channel_id IS DISTINCT FROM g.channel_id OR r.batch_id IS DISTINCT FROM g.batch_id))
  AND NOT EXISTS(SELECT 1 FROM collaboration_delegation d WHERE d.group_id=g.id AND d.state NOT IN ('settled','cancelled'))
RETURNING *;

-- name: ReleaseCollaborationRunReservation :exec
UPDATE collaboration_runtime_reservation reserve SET released_at=COALESCE(reserve.released_at,now()) FROM collaboration_run r
WHERE reserve.run_id=$1 AND r.id=reserve.run_id
    AND collaboration_managed_execution_ended(r.containment,r.proof_state,r.progress_state,r.settled_at,r.authorized_attempt_seq,r.sealed_attempt_seq,r.started_at,r.business_outcome);

-- name: ClearCollaborationChannelTargetGroups :many
UPDATE collaboration_target SET active_group_id=NULL WHERE channel_id=$1 AND active_group_id=$2 RETURNING *;
