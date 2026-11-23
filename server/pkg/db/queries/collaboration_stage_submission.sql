-- name: CreateCollaborationStageSubmission :one
INSERT INTO collaboration_stage_submission (submission_id,issue_id,workspace_id,lifecycle_epoch,policy_revision,role_revision,stage_revision,scope_decision_id,candidate_id,candidate_version,content_digest,release_decision_id,execution_observation_id,operation_id,actor_user_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING *;

-- name: GetCollaborationStageSubmission :one
SELECT * FROM collaboration_stage_submission WHERE issue_id=$1 AND submission_id=$2;

-- name: LockCollaborationStageCompletionCodeHead :one
SELECT * FROM collaboration_stage_candidate
WHERE issue_id=$1 AND workspace_id=$2 AND kind='code'
ORDER BY version DESC LIMIT 1 FOR SHARE NOWAIT;

-- name: LockCollaborationStageSubmission :one
SELECT * FROM collaboration_stage_submission
WHERE issue_id=$1 AND workspace_id=$2 AND submission_id=$3 FOR SHARE NOWAIT;

-- name: LockCollaborationStageOperation :one
SELECT operation.* FROM collaboration_operation operation
WHERE operation.id=$1 AND operation.scope_kind='issue' AND operation.scope_id=$2
 AND EXISTS (SELECT 1 FROM issue WHERE id=operation.scope_id AND workspace_id=$3)
FOR SHARE NOWAIT;

-- name: LockCollaborationStageAdminOperation :one
SELECT operation.* FROM admin_operation operation
WHERE operation.id=$1 AND operation.organization_id=$2 AND operation.actor_id=$3
 AND operation.target_kind='collaboration_issue' AND operation.target_id=$4
 AND EXISTS (SELECT 1 FROM issue WHERE id=operation.target_id AND workspace_id=$5)
FOR SHARE NOWAIT;

-- name: LockCollaborationStageSubmissionHandoff :one
SELECT handoff.* FROM issue_handoff handoff
WHERE handoff.issue_id=$1 AND handoff.id=$2
 AND EXISTS (SELECT 1 FROM issue WHERE id=handoff.issue_id AND workspace_id=$3)
FOR SHARE NOWAIT;

-- name: LockCollaborationStageDeliveryTerminal :one
SELECT * FROM delivery_completion_terminal WHERE authorization_id=$1 FOR SHARE NOWAIT;

-- name: CollaborationStageSubmissionReady :one
SELECT NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('submission_id','uuid'),('issue_id','uuid'),('workspace_id','uuid'),
 ('lifecycle_epoch','bigint'),('policy_revision','bigint'),('role_revision','bigint'),('stage_revision','bigint'),
 ('scope_decision_id','uuid'),('candidate_id','uuid'),('candidate_version','bigint'),('content_digest','text'),
 ('release_decision_id','uuid'),('execution_observation_id','uuid'),('operation_id','uuid'),('actor_user_id','uuid'),('created_at','timestamp with time zone')
 ) AS want(col,typ) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=to_regclass('collaboration_stage_submission') AND a.attname=want.col
 AND a.attnotnull AND NOT a.attisdropped AND format_type(a.atttypid,a.atttypmod)=want.typ
 )) AND NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('collaboration_stage_submission_id_idx',true,'submission_id'),
 ('collaboration_stage_submission_issue_idx',false,'issue_id,lifecycle_epoch,created_at,submission_id')
 ) AS want(idx,uniq,cols) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('collaboration_stage_submission') AND c.relname=want.idx AND a.amname='btree'
 AND i.indisunique=want.uniq AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indexprs IS NULL AND i.indpred IS NULL AND i.indnkeyatts=i.indnatts
 AND (SELECT string_agg(pg_get_indexdef(i.indexrelid,k,true),',' ORDER BY k) FROM generate_series(1,i.indnkeyatts::int) k)=want.cols
 )) AND EXISTS (
 SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 WHERE t.tgrelid=to_regclass('collaboration_stage_submission') AND t.tgname='collaboration_stage_submission_immutable' AND NOT t.tgisinternal
 AND t.tgenabled='O' AND t.tgtype=19 AND p.proname='collaboration_stage_immutable'
 ) AND NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('CHECK ((lifecycle_epoch > 0))'),('CHECK ((policy_revision > 0))'),('CHECK ((role_revision > 0))'),
 ('CHECK ((stage_revision > 0))'),('CHECK ((candidate_version > 0))'),('CHECK ((content_digest ~ ''^[0-9a-f]{64}$''::text))')
 ) AS want(def) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_constraint c WHERE c.conrelid=to_regclass('collaboration_stage_submission') AND c.contype='c' AND c.convalidated AND pg_get_constraintdef(c.oid)=want.def
 ));
