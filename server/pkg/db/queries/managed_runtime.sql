-- name: CountForeignInstallationDaemonRuntimes :one
SELECT count(*) FROM agent_runtime
WHERE workspace_id=sqlc.arg(workspace_id) AND daemon_id=sqlc.arg(daemon_id)
AND owner_id IS DISTINCT FROM sqlc.arg(principal_user_id)::uuid;

-- name: CaptureTaskExecutionBinding :one
UPDATE agent_task_queue SET execution_installation_id=sqlc.arg(installation_id),
 execution_binding_id=sqlc.arg(binding_id),execution_binding_epoch=sqlc.arg(binding_epoch)
WHERE id=sqlc.arg(task_id) AND runtime_id=sqlc.arg(runtime_id)
 AND dispatched_at=sqlc.arg(dispatched_at) AND execution_binding_id IS NULL
RETURNING *;

-- name: GetRuntimeRegistrationOwner :one
SELECT id, owner_id FROM agent_runtime
WHERE workspace_id=sqlc.arg(workspace_id) AND daemon_id=sqlc.arg(daemon_id)
AND ((profile_id IS NULL AND sqlc.narg(profile_id)::uuid IS NULL AND provider=sqlc.arg(provider))
 OR profile_id=sqlc.narg(profile_id)::uuid)
FOR UPDATE;
