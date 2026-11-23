-- name: GetCommentDelegationReceipt :one
SELECT * FROM comment_delegation_receipt WHERE workspace_id=$1 AND actor_type=$2 AND actor_id=$3 AND originator_id=$4 AND request_key=$5;

-- name: ClaimCommentDelegationReceipt :execrows
INSERT INTO comment_delegation_receipt (workspace_id,actor_type,actor_id,originator_id,request_key,request_digest,issue_id,comment_id,source_task_id,state,targets,response)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'unknown',$10,$11) ON CONFLICT DO NOTHING;

-- name: RecordCommentDelegationReceipt :execrows
UPDATE comment_delegation_receipt SET state='recorded', targets=$6, response=$7, recorded_at=clock_timestamp()
WHERE workspace_id=$1 AND actor_type=$2 AND actor_id=$3 AND originator_id=$4 AND request_key=$5 AND state='unknown' AND comment_id=$8;

-- name: LockDelegationIssue :one
SELECT id FROM issue WHERE id=$1 AND workspace_id=$2 FOR NO KEY UPDATE NOWAIT;

-- name: SnapshotCommentDelegationReceipt :execrows
UPDATE comment_delegation_receipt SET response=$6 WHERE workspace_id=$1 AND actor_type=$2 AND actor_id=$3 AND originator_id=$4 AND request_key=$5 AND state='unknown';

-- name: DelegationReceiptSchemaReady :one
SELECT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('comment_delegation_receipt')
 AND c.relname='comment_delegation_receipt_key_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=5 AND i.indnatts=5 AND i.indoption::text='0 0 0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='actor_type'
 AND pg_get_indexdef(i.indexrelid,3,true)='actor_id'
 AND pg_get_indexdef(i.indexrelid,4,true)='originator_id'
 AND pg_get_indexdef(i.indexrelid,5,true)='request_key'
 );

-- name: CommentDelegationReplayAllowed :one
-- Automatic reconciliation may continue only an originally recorded admission
-- for this exact executing agent. Unknown and refused targets never gain a new
-- dispatch through completion or cancellation repair.
SELECT NOT EXISTS (
 SELECT 1 FROM comment_delegation_receipt r
 WHERE r.workspace_id=$1 AND r.issue_id=$2 AND r.comment_id=$3
 AND (r.state <> 'recorded' OR NOT EXISTS (
  SELECT 1 FROM jsonb_array_elements(r.targets) target
  WHERE target->>'executing_agent_id'=sqlc.arg('executing_agent_id')::text
    AND target->>'target_type'=sqlc.arg('target_type')::text
    AND target->>'target_id'=sqlc.arg('target_id')::text
    AND target->>'status' IN ('queued','coalesced','deferred')
    AND COALESCE(target->>'task_id','') <> ''
 ))
);
