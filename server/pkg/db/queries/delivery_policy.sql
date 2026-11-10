-- name: DeliveryPolicySchemaReady :one
SELECT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('project_delivery_policy') AND c.relname='project_delivery_policy_revision_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3 AND i.indoption::text='0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='project_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='revision'
) AND
EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('delivery_evidence_evaluation') AND c.relname='delivery_evaluation_key_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3 AND i.indoption::text='0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='evaluator_user_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='request_key'
) AND
EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('delivery_evidence_evaluation') AND c.relname='delivery_evaluation_id_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=1 AND i.indnatts=1 AND i.indoption::text='0'
 AND pg_get_indexdef(i.indexrelid,1,true)='evaluation_id'
);

-- name: LockDeliveryPolicyProject :one
SELECT * FROM project WHERE id=$1 AND workspace_id=$2 FOR UPDATE NOWAIT;

-- name: GetDeliveryPolicy :one
SELECT * FROM project_delivery_policy WHERE workspace_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1;

-- name: CreateDeliveryPolicy :one
INSERT INTO project_delivery_policy (workspace_id,project_id,revision,policy_version,mode,stage,reason,created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- name: GetDeliveryEvaluation :one
SELECT * FROM delivery_evidence_evaluation WHERE workspace_id=$1 AND evaluator_user_id=$2 AND request_key=$3;

-- name: CreateDeliveryEvaluation :one
INSERT INTO delivery_evidence_evaluation (workspace_id,evaluator_user_id,request_key,evaluation_id,issue_id,project_id,policy_revision,request_digest,request_json,observation)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING RETURNING *;
