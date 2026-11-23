-- name: CreateCollaborationGroupDependency :exec
INSERT INTO collaboration_group_dependency(group_id,command_id,target_id,target_epoch,config_version,configuration_fingerprint,grants)
VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: ListCollaborationGroupDependencies :many
SELECT * FROM collaboration_group_dependency WHERE group_id=$1 ORDER BY command_id;

-- name: FreezeCollaborationRunPrompt :one
UPDATE collaboration_run SET prompt_snapshot=sqlc.arg(prompt_snapshot)::jsonb
WHERE id=sqlc.arg(run_id)::uuid AND prompt_snapshot IS NULL AND settled_at IS NULL RETURNING *;
