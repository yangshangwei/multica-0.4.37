-- name: LockLifecycleWorkspace :one
-- Take the workspace teardown fence before any child owner locks.
SELECT id FROM workspace WHERE id = $1 FOR KEY SHARE;

-- name: LockLifecycleIssue :one
-- NOWAIT avoids reversing SourceContext's issue-before-counter lock order.
-- NO KEY UPDATE still permits ordinary child creation's parent key-share lock.
SELECT * FROM issue WHERE id = $1 AND workspace_id = $2
FOR NO KEY UPDATE NOWAIT;

-- name: LockLifecycleAgent :one
SELECT * FROM agent WHERE id = $1 AND workspace_id = $2 FOR SHARE NOWAIT;

-- name: LockLifecycleSquad :one
SELECT * FROM squad WHERE id = $1 AND workspace_id = $2 FOR SHARE NOWAIT;

-- name: LockLifecycleRuntime :one
SELECT * FROM agent_runtime WHERE id = $1 AND workspace_id = $2 FOR KEY SHARE NOWAIT;

-- name: LockLifecycleOriginTask :one
SELECT t.* FROM agent_task_queue t JOIN agent a ON a.id = t.agent_id
WHERE t.id = $1 AND a.workspace_id = $2 FOR SHARE OF t NOWAIT;

-- name: LockPendingLifecycleTask :one
SELECT * FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2
  AND (status IN ('queued', 'dispatched')
    OR (status = 'deferred' AND context->>'channel_issue_media_pending' = 'true'))
FOR UPDATE NOWAIT;

-- name: LifecycleMetadataSize :one
SELECT pg_column_size(sqlc.arg(metadata)::jsonb)::integer;

-- name: SetLifecycleMetadata :one
UPDATE issue SET metadata = sqlc.arg(metadata)::jsonb,
  revision = revision + 1, updated_at = now(), last_activity_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
RETURNING *;
