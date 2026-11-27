-- name: GetMaterialSettings :one
SELECT * FROM project_material_settings WHERE project_id=$1 AND workspace_id=$2;
-- name: LockMaterialProjectForClaim :one
SELECT * FROM project WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT;
-- name: SaveMaterialSettings :one
WITH pending AS (
 SELECT p.material_id,p.baseline_id,p.task_id FROM git_material_plan p
 JOIN git_material m ON m.id=p.material_id AND m.current_revision_id=p.baseline_id
 LEFT JOIN agent_task_queue t ON t.id=p.task_id
 WHERE p.project_id=$1 AND p.workspace_id=$2 AND NOT p.manual_start AND p.requested
 AND (NOT COALESCE(($4::jsonb->>'auto_start_plan')::boolean,false) OR NOT COALESCE(($4::jsonb->>'enabled')::boolean,false))
 AND (p.task_id IS NULL OR t.status IN ('deferred','queued','dispatched','waiting_local_directory'))
), cancelled AS (
 UPDATE agent_task_queue t SET status='cancelled',completed_at=now(),prepare_lease_expires_at=NULL
 FROM pending p WHERE t.id=p.task_id AND t.status IN ('deferred','queued','dispatched','waiting_local_directory') RETURNING t.id
), held AS (
 UPDATE git_material_plan p SET requested=false,status='waiting_manual',blocker=NULL,task_id=NULL,updated_at=now()
 FROM pending h WHERE p.material_id=h.material_id AND p.baseline_id=h.baseline_id
 AND (h.task_id IS NULL OR h.task_id IN (SELECT id FROM cancelled)) RETURNING p.material_id,p.baseline_id
), reset_material AS (
 UPDATE git_material m SET version=m.version+1,status='baseline_ready',blocker=NULL,updated_at=now()
 FROM held WHERE m.id=held.material_id AND m.current_revision_id=held.baseline_id
)
INSERT INTO project_material_settings (project_id,workspace_id,version,settings,authorized_by,member_id)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (project_id) DO UPDATE SET version=EXCLUDED.version, settings=EXCLUDED.settings,
 authorized_by=EXCLUDED.authorized_by,member_id=EXCLUDED.member_id,updated_at=now()
RETURNING *;
-- name: ListGitMaterials :many
SELECT * FROM git_material WHERE workspace_id=$1 AND project_id=$2 ORDER BY created_at DESC,id;
-- name: GetGitMaterial :one
SELECT * FROM git_material WHERE workspace_id=$1 AND project_id=$2 AND id=$3;
-- name: CreateGitMaterial :one
INSERT INTO git_material (id,workspace_id,project_id,title,path) VALUES ($1,$2,$3,$4,$5) RETURNING *;
-- name: UpdateGitMaterial :one
UPDATE git_material SET version=version+1,status=$4,blocker=$5,current_revision_id=$6,baseline=$7,updated_at=now()
WHERE workspace_id=$1 AND project_id=$2 AND id=$3 RETURNING *;
-- name: ListMaterialRevisions :many
SELECT * FROM git_material_revision WHERE material_id=$1 ORDER BY created_at,id;
-- name: GetMaterialRevision :one
SELECT * FROM git_material_revision WHERE id=$1;
-- name: CreateMaterialRevision :one
INSERT INTO git_material_revision (id,material_id,workspace_id,project_id,kind,payload,settings,config_version,created_by,publication_status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING *;
-- name: ObserveMaterialRevision :exec
UPDATE git_material_revision SET publication_status=$2, review=$3 WHERE id=$1;
-- name: ListMaterialDecisions :many
SELECT * FROM git_material_decision WHERE material_id=$1 ORDER BY created_at,id;
-- name: LatestMaterialDecision :one
SELECT * FROM git_material_decision WHERE revision_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1;
-- name: CreateMaterialDecision :one
INSERT INTO git_material_decision (id,material_id,revision_id,workspace_id,project_id,actor_id,member_id,credential,decision,reason,head_sha)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *;
-- name: ListMaterialComments :many
SELECT * FROM git_material_comment WHERE material_id=$1 ORDER BY created_at,id;
-- name: CreateMaterialComment :one
INSERT INTO git_material_comment (id,material_id,revision_id,workspace_id,project_id,actor_id,body)
VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING *;
-- name: GetMaterialRequest :one
SELECT * FROM git_material_request WHERE workspace_id=$1 AND project_id=$2 AND actor_id=$3 AND request_id=$4;
-- name: CreateMaterialRequest :exec
INSERT INTO git_material_request (workspace_id,project_id,actor_id,request_id,body_hash,response) VALUES ($1,$2,$3,$4,$5,$6);
-- name: EnsureMaterialJob :exec
INSERT INTO git_material_job (id,workspace_id,project_id,material_id,revision_id,kind) VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (revision_id,kind) DO UPDATE SET state='pending',blocker=NULL,next_at=now(),updated_at=now()
WHERE git_material_job.state IN ('blocked','done');
-- name: ClaimMaterialJob :one
WITH candidate AS (SELECT id FROM git_material_job WHERE state IN ('pending','working') AND next_at<=now()
 AND (leased_until IS NULL OR leased_until<now()) ORDER BY next_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
UPDATE git_material_job SET state='working',lease_token=$1,leased_until=now()+interval '2 minutes',attempts=attempts+1,updated_at=now()
WHERE id IN (SELECT id FROM candidate) RETURNING *;
-- name: LockMaterialJob :one
SELECT * FROM git_material_job WHERE id=$1 AND lease_token=$2 AND state='working' AND leased_until>now() FOR UPDATE;
-- name: FinishMaterialJob :exec
UPDATE git_material_job SET state=$3,blocker=$4,next_at=now()+interval '15 seconds',leased_until=NULL,lease_token=NULL,updated_at=now()
WHERE id=$1 AND lease_token=$2;
-- name: GetMaterialPlan :one
SELECT p.* FROM git_material_plan p JOIN git_material m ON m.id=p.material_id
WHERE p.material_id=$1 AND p.baseline_id::text=m.baseline->>'id';
-- name: GetMaterialPlanForIssue :one
SELECT * FROM git_material_plan WHERE issue_id=$1;
-- name: CreateMaterialPlan :exec
INSERT INTO git_material_plan (material_id,workspace_id,project_id,baseline_id,issue_id,status,requested,authorized_by,member_id,agent_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);
-- name: UpdateMaterialPlan :exec
UPDATE git_material_plan SET status=$2,requested=$3,authorized_by=$4,member_id=$5,agent_id=$6,task_id=$7,revision_id=$8,blocker=$9,updated_at=now()
WHERE material_id=$1 AND baseline_id=$10;
-- name: MarkMaterialPlanManualStart :exec
UPDATE git_material_plan SET manual_start=true WHERE material_id=$1 AND baseline_id=$2;
-- name: MaterialProjectHasHistory :one
SELECT EXISTS (SELECT 1 FROM git_material WHERE project_id=$1 AND workspace_id=$2)::boolean;
-- name: DeleteMaterialSettings :exec
DELETE FROM project_material_settings WHERE project_id=$1 AND workspace_id=$2;
-- name: MaterialExecutionBinding :one
SELECT p.*,m.baseline,m.current_revision_id,m.status AS material_status,s.settings,s.authorized_by AS config_authorized_by,s.member_id AS config_member_id
FROM git_material_plan p JOIN git_material m ON m.id=p.material_id JOIN project_material_settings s ON s.project_id=p.project_id
WHERE p.issue_id=$1;
-- name: MaterialWorkspaceHasHistory :one
SELECT EXISTS (SELECT 1 FROM git_material WHERE workspace_id=$1)::boolean;
-- name: DeleteWorkspaceMaterialSettings :exec
DELETE FROM project_material_settings WHERE workspace_id=$1;
-- name: MaterialIssueHasHistory :one
SELECT EXISTS (SELECT 1 FROM git_material_plan WHERE issue_id=$1)::boolean;
-- name: GetMaterialPlanForTask :one
SELECT * FROM git_material_plan WHERE task_id=$1;

-- name: EnsureMaterialCarrier :one
INSERT INTO agent (workspace_id,name,runtime_mode,runtime_config,runtime_id,visibility,permission_mode,max_concurrent_tasks,owner_id,instructions,custom_env,custom_args,kind,system_key)
VALUES (@workspace_id,@system_key::text,@runtime_mode,'{}',@runtime_id,'private','private',1,@owner_id,'Execute only the authorized material stage. Never self-approve.','{}','[]','system',@system_key)
ON CONFLICT (workspace_id,owner_id,runtime_id,system_key) WHERE system_key IS NOT NULL DO UPDATE SET system_key=EXCLUDED.system_key RETURNING *;
-- name: CreateMaterialOperationTask :one
INSERT INTO agent_task_queue (id,agent_id,runtime_id,status,priority,force_fresh_session,max_attempts,context,originator_user_id,accountable_user_id,originator_source,trigger_evidence_kind,trigger_evidence_ref_id)
VALUES (@id,@agent_id,@runtime_id,'queued',0,true,1,@context,@authorizer_id,@authorizer_id,'direct_human','material_operation',@operation_id) RETURNING *;
-- name: CreateMaterialOperation :one
INSERT INTO git_material_operation (id,job_id,task_id,workspace_id,project_id,material_id,revision_id,runtime_id,agent_id,authorizer_id,attempt_no,action,descriptor,expected_digest)
VALUES (@id,@job_id,@task_id,@workspace_id,@project_id,@material_id,@revision_id,@runtime_id,@agent_id,@authorizer_id,@attempt_no,@action,@descriptor,@expected_digest) RETURNING *;
-- name: GetMaterialOperationForTask :one
SELECT * FROM git_material_operation WHERE task_id=$1;
-- name: GetMaterialOperation :one
SELECT * FROM git_material_operation WHERE id=$1;
-- name: LatestMaterialOperation :one
SELECT * FROM git_material_operation WHERE job_id=$1 ORDER BY attempt_no DESC LIMIT 1;
-- name: CurrentMaterialOperation :one
SELECT o.*,t.status AS task_status FROM git_material_operation o JOIN agent_task_queue t ON t.id=o.task_id WHERE o.material_id=$1 ORDER BY o.created_at DESC LIMIT 1;
-- name: BindMaterialOperationClaim :exec
UPDATE git_material_operation SET claim_generation=$2,challenge_hash=$3,state='dispatched',receipt=NULL,receipt_digest='',observation_id='',permit_expires_at=NULL,permit_request_id='',updated_at=now() WHERE task_id=$1 AND claim_generation<$2;
-- name: SaveMaterialObservation :exec
UPDATE git_material_operation SET receipt=$2,receipt_digest=$3,observation_id=$4,state=$5,blocker=$6,receipt_history=jsonb_set(receipt_history,ARRAY[$4::text],to_jsonb($3::text)),updated_at=now() WHERE id=$1;
-- name: SaveMaterialPermit :exec
UPDATE git_material_operation SET permit_request_id=$2,permit_expires_at=$3,updated_at=now() WHERE id=$1;
-- name: CompleteMaterialOperationJob :exec
UPDATE git_material_job SET state=$2,blocker=$3,leased_until=NULL,lease_token=NULL,updated_at=now() WHERE id=$1;
-- name: DetachMaterialCarriers :exec
UPDATE agent SET runtime_id=NULL WHERE runtime_id=$1 AND kind='system' AND system_key LIKE 'material:%';
-- name: DisableMaterialTaskRetries :exec
UPDATE agent_task_queue SET max_attempts=1 WHERE id=$1;
-- name: RequestMaterialOperationRecovery :execrows
UPDATE git_material_operation o SET state='recovery_requested',updated_at=now()
FROM agent_task_queue t WHERE o.task_id=t.id AND o.material_id=$1 AND o.id=(SELECT id FROM git_material_operation WHERE material_id=$1 ORDER BY created_at DESC LIMIT 1)
AND o.state NOT IN ('succeeded','no_effect') AND t.status IN ('completed','failed','cancelled');
-- name: RetryMaterialJob :exec
UPDATE git_material_job SET state='pending',next_at=now(),leased_until=NULL,lease_token=NULL,blocker=NULL WHERE revision_id=$1 AND kind=$2;
-- name: GetMaterialPermittedMerge :one
SELECT * FROM git_material_operation WHERE job_id=$1 AND action='merge' AND permit_request_id<>'' ORDER BY attempt_no DESC LIMIT 1;
-- name: LockMaterialRuntimeProfile :one
-- Claims and observations may already own the task/runtime lock. Never wait
-- behind a configuration writer that could be waiting for that same task.
SELECT * FROM runtime_profile WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT;
