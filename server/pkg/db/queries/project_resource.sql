-- name: ListProjectResources :many
SELECT * FROM project_resource
WHERE project_id = $1
ORDER BY position ASC, created_at ASC;

-- name: ListProjectResourcesInWorkspace :many
-- Workspace-scoped read for the daemon claim path. project_resource carries its
-- own workspace_id, so a corrupt project reference cannot pull another tenant's
-- repository URLs or local paths into a claim response.
SELECT * FROM project_resource
WHERE project_id = $1 AND workspace_id = $2
ORDER BY position ASC, created_at ASC;

-- name: ListProjectResourcesForProjects :many
SELECT * FROM project_resource
WHERE project_id = ANY(sqlc.arg('project_ids')::uuid[])
ORDER BY project_id, position ASC, created_at ASC;

-- name: GetProjectResource :one
SELECT * FROM project_resource
WHERE id = $1;

-- name: GetProjectResourceInWorkspace :one
SELECT * FROM project_resource
WHERE id = $1 AND workspace_id = $2;

-- name: CreateProjectResource :one
-- Serialize the whole resource set with an execution fingerprint's FOR SHARE
-- project lock. Consume the materialized owner before inserting the child.
WITH project_fence AS MATERIALIZED (
    SELECT p.id, p.workspace_id FROM project p
    WHERE p.id = sqlc.arg(project_id) AND p.workspace_id = sqlc.arg(workspace_id)
    FOR NO KEY UPDATE
)
INSERT INTO project_resource (
    project_id, workspace_id, resource_type, resource_ref, label, position, created_by
)
SELECT project_fence.id, project_fence.workspace_id, sqlc.arg(resource_type), sqlc.arg(resource_ref), sqlc.narg(label), sqlc.arg(position), sqlc.narg(created_by)
FROM project_fence
RETURNING *;

-- name: UpdateProjectResource :one
WITH project_fence AS MATERIALIZED (
    SELECT p.id FROM project p
    JOIN project_resource r ON r.project_id = p.id AND r.workspace_id = p.workspace_id
    WHERE r.id = $1
    FOR NO KEY UPDATE OF p
)
UPDATE project_resource
SET resource_ref = $2,
    label        = $3,
    position     = $4
WHERE project_resource.id = $1 AND project_resource.project_id IN (SELECT project_fence.id FROM project_fence)
RETURNING *;

-- name: DeleteProjectResource :exec
WITH project_fence AS MATERIALIZED (
    SELECT p.id FROM project p
    JOIN project_resource r ON r.project_id = p.id AND r.workspace_id = p.workspace_id
    WHERE r.id = $1
    FOR NO KEY UPDATE OF p
)
DELETE FROM project_resource
WHERE project_resource.id = $1 AND project_resource.project_id IN (SELECT project_fence.id FROM project_fence);

-- name: CountProjectResources :one
SELECT count(*) FROM project_resource WHERE project_id = $1;

-- name: GetProjectResourceCounts :many
SELECT project_id, count(*)::bigint AS resource_count
FROM project_resource
WHERE project_id = ANY(sqlc.arg('project_ids')::uuid[])
GROUP BY project_id;
