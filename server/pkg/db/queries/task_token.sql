-- name: CreateTaskToken :one
INSERT INTO task_token (token_hash, task_id, agent_id, workspace_id, user_id, expires_at, id, auth_version, installation_binding_id, installation_binding_epoch, api_capability)
VALUES ($1, $2, $3, $4, $5, $6, COALESCE(sqlc.narg('id')::uuid, gen_random_uuid()), sqlc.arg('auth_version'), sqlc.narg('installation_binding_id')::uuid, sqlc.narg('installation_binding_epoch')::bigint, sqlc.narg('api_capability')::jsonb)
RETURNING *;

-- name: GetTaskTokenByHash :one
SELECT * FROM task_token
WHERE token_hash = $1 AND expires_at > now();

-- name: DeleteTaskTokensByTask :exec
DELETE FROM task_token WHERE task_id = $1;

-- name: DeleteExpiredTaskTokens :exec
DELETE FROM task_token WHERE expires_at <= now();

-- name: LockTaskCapabilityPrincipals :one
-- NOWAIT avoids introducing a lock-order wait into existing task/issue paths.
-- The same transaction performs the authority check and comment/token write.
SELECT t.id FROM agent_task_queue t
JOIN agent a ON a.id=t.agent_id
JOIN agent_runtime r ON r.id=t.runtime_id
JOIN issue i ON i.id=t.issue_id
JOIN member origin_member ON origin_member.user_id=t.originator_user_id AND origin_member.workspace_id=i.workspace_id
JOIN member owner_member ON owner_member.user_id=r.owner_id AND owner_member.workspace_id=i.workspace_id
JOIN "user" origin_user ON origin_user.id=origin_member.user_id
JOIN "user" owner_user ON owner_user.id=owner_member.user_id
WHERE t.id=$1
FOR NO KEY UPDATE OF i NOWAIT
FOR SHARE OF t,a,r,origin_member,owner_member,origin_user,owner_user NOWAIT;

-- name: LockTaskCapabilityInvocationTargets :many
SELECT * FROM agent_invocation_target WHERE agent_id=$1 FOR SHARE NOWAIT;

-- name: LockTaskCapabilityToken :one
SELECT id FROM task_token WHERE id=$1 AND expires_at>now() FOR SHARE NOWAIT;
