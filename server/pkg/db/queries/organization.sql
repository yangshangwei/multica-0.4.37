-- name: GetInternalOrganization :one
SELECT * FROM organization WHERE internal = true AND state = 'active';

-- name: EnsureInternalOrganization :one
INSERT INTO organization (name, internal) VALUES ('Internal organization', true)
ON CONFLICT (internal) DO UPDATE SET internal = EXCLUDED.internal
RETURNING *;

-- name: AssignWorkspaceOrganization :one
INSERT INTO organization_workspace (organization_id, workspace_id)
SELECT o.id, w.id FROM organization o CROSS JOIN workspace w
WHERE o.internal = true AND o.state = 'active' AND w.id = $1
ON CONFLICT (workspace_id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id
WHERE organization_workspace.organization_id = EXCLUDED.organization_id
RETURNING organization_id;

-- name: BackfillWorkspaceOrganizations :many
INSERT INTO organization_workspace (organization_id, workspace_id)
SELECT o.id, w.id FROM organization o CROSS JOIN workspace w
WHERE o.internal = true AND o.state = 'active'
AND NOT EXISTS (SELECT 1 FROM organization_workspace ow WHERE ow.workspace_id = w.id)
ORDER BY w.id LIMIT 500
ON CONFLICT (workspace_id) DO NOTHING
RETURNING workspace_id;

-- name: CountUnassignedWorkspaces :one
SELECT count(*) FROM workspace w WHERE NOT EXISTS (SELECT 1 FROM organization_workspace ow WHERE ow.workspace_id = w.id);

-- name: DeleteWorkspaceOrganization :exec
DELETE FROM organization_workspace WHERE workspace_id = $1;
