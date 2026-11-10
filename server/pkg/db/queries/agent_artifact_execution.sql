-- name: ArtifactExecutionSchemaReady :one
SELECT count(*) = 3 AS ready
FROM pg_catalog.pg_index AS idx
JOIN pg_catalog.pg_class AS index_relation ON index_relation.oid = idx.indexrelid
JOIN pg_catalog.pg_am AS method ON method.oid = index_relation.relam
WHERE idx.indrelid = to_regclass('agent_artifact_execution')
  AND index_relation.relnamespace = (SELECT relnamespace FROM pg_catalog.pg_class WHERE oid = to_regclass('agent_artifact_execution'))
  AND idx.indisunique AND idx.indisvalid AND idx.indisready AND idx.indislive
  AND NOT idx.indisexclusion AND idx.indpred IS NULL AND idx.indexprs IS NULL
  AND method.amname = 'btree'
  AND ((index_relation.relname = 'agent_artifact_execution_id_idx' AND idx.indnkeyatts = 1 AND idx.indnatts = 1 AND idx.indoption::text = '0'
        AND pg_get_indexdef(idx.indexrelid, 1, true) = 'execution_id')
    OR (index_relation.relname = 'agent_artifact_execution_approval_idx' AND idx.indnkeyatts = 1 AND idx.indnatts = 1 AND idx.indoption::text = '0'
        AND pg_get_indexdef(idx.indexrelid, 1, true) = 'approval_id')
    OR (index_relation.relname = 'agent_artifact_execution_idempotency_idx' AND idx.indnkeyatts = 4 AND idx.indnatts = 4 AND idx.indoption::text = '0 0 0 0'
        AND pg_get_indexdef(idx.indexrelid, 1, true) = 'workspace_id' AND pg_get_indexdef(idx.indexrelid, 2, true) = 'initiator_kind' AND pg_get_indexdef(idx.indexrelid, 3, true) = 'initiator_id' AND pg_get_indexdef(idx.indexrelid, 4, true) = 'idempotency_key'));

-- name: ArtifactExecutionDatabaseTime :one
SELECT clock_timestamp()::timestamptz AS database_time;

-- name: CreateAgentArtifactExecution :one
INSERT INTO agent_artifact_execution (
    execution_id, approval_id, workspace_id, initiator_kind, initiator_id, originator_user_id, agent_id, task_id, runtime_id, task_generation, resource_kind, resource_id, action, idempotency_key, request_digest, canonical_spec, spec_digest, source_revision, staging_id, staging_manifest_digest, input_manifest_digest, invocation_digest, policy_version, target_environment, root_config_identity, created_at, expires_at, updated_at
) VALUES (
    sqlc.arg(execution_id), sqlc.arg(approval_id), sqlc.arg(workspace_id), sqlc.arg(initiator_kind), sqlc.arg(initiator_id), sqlc.arg(originator_user_id), sqlc.arg(agent_id), sqlc.arg(task_id), sqlc.arg(runtime_id), sqlc.arg(task_generation), sqlc.arg(resource_kind), sqlc.arg(resource_id), sqlc.arg(action), sqlc.arg(idempotency_key), sqlc.arg(request_digest), sqlc.arg(canonical_spec), sqlc.arg(spec_digest), sqlc.arg(source_revision), sqlc.arg(staging_id), sqlc.arg(staging_manifest_digest), sqlc.arg(input_manifest_digest), sqlc.arg(invocation_digest), sqlc.arg(policy_version), sqlc.arg(target_environment), sqlc.arg(root_config_identity), sqlc.arg(created_at), sqlc.arg(expires_at), sqlc.arg(updated_at)
)
ON CONFLICT (workspace_id, initiator_kind, initiator_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetAgentArtifactExecutionByApproval :one
SELECT * FROM agent_artifact_execution WHERE approval_id = $1;

-- name: GetAgentArtifactExecutionByIdempotency :one
SELECT * FROM agent_artifact_execution WHERE workspace_id = $1 AND initiator_kind = $2 AND initiator_id = $3 AND idempotency_key = $4;

-- name: LockAgentArtifactExecutionByApproval :one
SELECT * FROM agent_artifact_execution WHERE approval_id = $1 FOR UPDATE NOWAIT;

-- Caller holds approval then execution locks and revalidates current authority.
-- name: ClaimAgentArtifactExecution :one
UPDATE agent_artifact_execution SET
    claimed_kind = sqlc.arg(claimed_kind), claimed_by = sqlc.arg(claimed_by), claim_nonce = sqlc.arg(claim_nonce),
    claimed_at = statement_timestamp(), started_at = statement_timestamp(), result_status = 'unknown',
    result_code = 'claimed', updated_at = statement_timestamp()
WHERE execution_id = sqlc.arg(execution_id) AND spec_digest = sqlc.arg(spec_digest)
    AND claimed_at IS NULL AND result_status IS NULL AND expires_at > clock_timestamp()
RETURNING *;

-- The original nonce owner records one outcome; terminal unknown is not retryable.
-- name: FinalizeAgentArtifactExecution :one
UPDATE agent_artifact_execution SET
    result_status = sqlc.arg(result_status), result_code = sqlc.arg(result_code),
    receipt_json = sqlc.arg(receipt_json), receipt_digest = sqlc.arg(receipt_digest),
    output_artifact_id = sqlc.narg(output_artifact_id), output_artifact_digest = sqlc.arg(output_artifact_digest),
    output_size = sqlc.narg(output_size), finished_at = clock_timestamp(), updated_at = clock_timestamp()
WHERE execution_id = sqlc.arg(execution_id) AND spec_digest = sqlc.arg(spec_digest)
    AND claim_nonce = sqlc.arg(claim_nonce) AND result_status = 'unknown'
    AND claimed_at IS NOT NULL AND finished_at IS NULL
    AND sqlc.arg(result_status)::text IN ('succeeded', 'failed', 'unknown')
RETURNING *;

-- SHARE rather than KEY SHARE also fences non-key role/account updates.
-- name: LockArtifactUsers :many
SELECT * FROM "user" WHERE id = ANY(sqlc.arg(user_ids)::uuid[]) ORDER BY id FOR SHARE NOWAIT;

-- name: LockArtifactMembers :many
SELECT * FROM member WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY user_id, id FOR SHARE NOWAIT;

-- name: LockArtifactPasswordCredential :one
SELECT * FROM user_password_credential WHERE user_id = $1 FOR SHARE NOWAIT;

-- name: LockArtifactPAT :one
SELECT * FROM personal_access_token WHERE token_hash = $1 AND user_id = $2
    AND NOT revoked AND (expires_at IS NULL OR expires_at > clock_timestamp()) FOR SHARE NOWAIT;
