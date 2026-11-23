-- The caller already holds the deleting parent. Do not join or lock sources,
-- candidates or task rows while cleaning up independent target counters.
-- name: LockKnowledgeTargetsForDeletion :many
SELECT * FROM collaboration_knowledge_target
WHERE workspace_id = sqlc.arg(workspace_id)::uuid
  AND (sqlc.narg(project_id)::uuid IS NULL
    OR target_kind = 'project' AND target_id = sqlc.narg(project_id)::uuid)
ORDER BY target_kind, target_id FOR UPDATE NOWAIT;

-- Role writes already acquire this row through UPDATE. Read its actual old
-- role under the same lock so a concurrent change cannot hide a real transition.
-- name: LockKnowledgeMemberRoleChange :one
SELECT * FROM member WHERE id = $1 AND workspace_id = $2 FOR UPDATE;

-- Operational metadata only; never select source/target identities or content.
-- name: GetKnowledgeInvalidationPendingAge :one
SELECT count(*)::bigint AS pending_jobs,
  COALESCE(EXTRACT(EPOCH FROM (clock_timestamp() - min(created_at))), 0)::bigint AS oldest_seconds
FROM collaboration_knowledge_invalidation WHERE state = 'pending';
