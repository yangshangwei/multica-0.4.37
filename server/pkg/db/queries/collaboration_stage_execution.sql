-- These private rows are not response DTOs. All writes borrow the caller's
-- transaction and are followed by its real commit boundary.
-- name: CreateStageExecutionIntent :one
INSERT INTO collaboration_stage_execution_intent
(intent_id,workspace_id,issue_id,candidate_id,candidate_version,candidate_digest,lifecycle_epoch,role_revision,policy_revision,
 approval_decision_id,request_id,request_digest,consumer_subject,physical_target_key,target_digest,artifact_digest,kind,invocation,dependencies,admitted_at,dispatch_expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING *;

-- name: GetStageExecutionIntent :one
SELECT * FROM collaboration_stage_execution_intent WHERE intent_id=$1;

-- name: LockStageExecutionIntent :one
SELECT * FROM collaboration_stage_execution_intent WHERE intent_id=$1 FOR UPDATE;

-- name: GetStageExecutionIntentByApproval :one
SELECT * FROM collaboration_stage_execution_intent WHERE issue_id=$1 AND approval_decision_id=$2;

-- name: GetStageExecutionIntentByRequest :one
SELECT * FROM collaboration_stage_execution_intent WHERE workspace_id=$1 AND consumer_subject=$2 AND request_id=$3;

-- The physical concurrency domain is global across local workspace/binding
-- identities. Outward refusals must not expose the matching private row.
-- name: HasUnresolvedStageExecution :one
SELECT EXISTS (SELECT 1 FROM collaboration_stage_execution_intent i WHERE physical_target_key=$1
 AND NOT EXISTS (SELECT 1 FROM collaboration_stage_execution_observation o WHERE o.intent_id=i.intent_id
 AND o.result IN ('succeeded','failed','not_started')))::boolean;

-- name: ListStageExecutionIntents :many
SELECT * FROM collaboration_stage_execution_intent WHERE issue_id=$1 AND candidate_id=$2
ORDER BY admitted_at DESC,intent_id DESC LIMIT $3;

-- name: CreateStageExecutionObservation :one
INSERT INTO collaboration_stage_execution_observation
(observation_id,intent_id,workspace_id,issue_id,origin,native_id,native_version,observation_digest,result,external_operation_id,observation)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *;

-- name: GetStageExecutionObservationByNative :one
SELECT * FROM collaboration_stage_execution_observation WHERE intent_id=$1 AND origin=$2 AND native_id=$3 AND native_version=$4;

-- name: GetStageExecutionTerminal :one
SELECT * FROM collaboration_stage_execution_observation WHERE intent_id=$1 AND result IN ('succeeded','failed','not_started');

-- name: GetStageExecutionLatestObservation :one
SELECT * FROM collaboration_stage_execution_observation WHERE intent_id=$1
ORDER BY created_at DESC,observation_id DESC LIMIT 1;

-- name: GetStageExecutionFact :one
SELECT * FROM collaboration_stage_execution_observation WHERE observation_id=$1 AND result IN ('succeeded','failed');

-- name: CreateStageRollbackObservation :one
INSERT INTO collaboration_stage_rollback_observation
(observation_id,workspace_id,issue_id,candidate_id,deployment_id,stream_key,version,native_version,observation_digest,metric)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING *;

-- name: GetStageRollbackObservation :one
SELECT * FROM collaboration_stage_rollback_observation WHERE observation_id=$1;

-- name: GetStageRollbackObservationByContent :one
SELECT * FROM collaboration_stage_rollback_observation WHERE stream_key=$1 AND native_version=$2 AND observation_digest=$3;

-- name: StageRollbackNextVersion :one
SELECT (COALESCE(max(version),0)+1)::bigint FROM collaboration_stage_rollback_observation WHERE stream_key=$1;

-- name: StageRollbackNativeHead :one
SELECT COALESCE(max(native_version),0)::bigint AS native_version FROM collaboration_stage_rollback_observation WHERE stream_key=$1;

-- Equal native versions with distinct canonical contents remain durable
-- ambiguity; a later arrival can never select a winner for that version.
-- name: StageRollbackNativeAmbiguous :one
SELECT (count(DISTINCT observation_digest)>1)::boolean FROM collaboration_stage_rollback_observation WHERE stream_key=$1 AND native_version=$2;

-- name: ListStageRollbackObservations :many
SELECT * FROM collaboration_stage_rollback_observation WHERE issue_id=$1 AND candidate_id=$2
ORDER BY created_at DESC,observation_id DESC LIMIT $3;

-- name: StageExecutionSchemaReady :one
SELECT NOT EXISTS (
 SELECT 1 FROM (VALUES
  ('collaboration_stage_execution_intent','collaboration_stage_execution_intent_id_idx',true,'intent_id',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_approval_idx',true,'approval_decision_id',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_request_idx',true,'workspace_id,consumer_subject,request_id',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_target_idx',false,'physical_target_key,admitted_at',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_candidate_idx',false,'issue_id,candidate_id,admitted_at,intent_id',''),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_observation_id_idx',true,'observation_id',''),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_observation_native_idx',true,'intent_id,origin,native_id,native_version',''),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_terminal_idx',true,'intent_id','(result = ANY (ARRAY[''succeeded''::text, ''failed''::text, ''not_started''::text]))'),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_observation_order_idx',false,'intent_id,created_at,observation_id',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_id_idx',true,'observation_id',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_version_idx',true,'stream_key,version',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_content_idx',true,'stream_key,native_version,observation_digest',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_order_idx',false,'issue_id,candidate_id,created_at,observation_id','')
 ) AS want(tbl,idx,uniq,cols,pred)
 WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass(want.tbl) AND c.relname=want.idx AND a.amname='btree'
 AND i.indisunique=want.uniq AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indexprs IS NULL AND i.indnkeyatts=i.indnatts
 AND (SELECT string_agg(pg_get_indexdef(i.indexrelid,k,true),',' ORDER BY k) FROM generate_series(1,i.indnkeyatts::int) k)=want.cols
 AND COALESCE(pg_get_expr(i.indpred,i.indrelid),'')=want.pred
 )) AND NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('collaboration_stage_execution_intent'),('collaboration_stage_execution_observation'),('collaboration_stage_rollback_observation')
 ) AS want(tbl) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 WHERE t.tgrelid=to_regclass(want.tbl) AND t.tgname=want.tbl||'_immutable' AND NOT t.tgisinternal
 AND t.tgenabled='O' AND t.tgtype=19 AND p.proname='collaboration_stage_immutable'
 ));

-- name: ListStageExecutionFacts :many
SELECT o.* FROM collaboration_stage_execution_observation o
JOIN collaboration_stage_execution_intent i ON i.intent_id=o.intent_id
WHERE i.issue_id=$1 AND i.candidate_id=$2 AND o.issue_id=i.issue_id AND o.workspace_id=i.workspace_id
 AND o.result IN ('succeeded','failed')
ORDER BY o.created_at DESC,o.observation_id DESC LIMIT $3;
