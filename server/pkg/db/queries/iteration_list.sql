-- name: GetWorkspaceIterationListVersion :one
-- A coherent collection fingerprint invalidates cursors when period metadata
-- or scope changes, without coupling collection edits to settings revision.
SELECT md5(COALESCE(string_agg(id::text || ':' || revision::text || ':' || scope_revision::text,
 ',' ORDER BY id),''))::text AS version FROM iteration WHERE workspace_id=$1;

-- name: ListWorkspaceIterations :many
SELECT * FROM iteration i WHERE i.workspace_id=sqlc.arg('workspace_id')
 AND (sqlc.narg('issue_id')::uuid IS NULL OR EXISTS (SELECT 1 FROM iteration_participation ip WHERE ip.workspace_id=i.workspace_id AND ip.iteration_id=i.id AND ip.issue_id=sqlc.narg('issue_id')))
 AND (sqlc.arg('status')::text='' OR i.status=sqlc.arg('status'))
 AND (sqlc.arg('search')::text='' OR strpos(lower(i.name),lower(sqlc.arg('search')))>0)
 AND (sqlc.narg('from_date')::date IS NULL OR i.end_date>=sqlc.narg('from_date'))
 AND (sqlc.narg('to_date')::date IS NULL OR i.start_date<=sqlc.narg('to_date'))
 AND (NOT sqlc.arg('has_after')::boolean OR (i.start_date,i.id)>(sqlc.narg('after_date')::date,sqlc.narg('after_id')::uuid))
ORDER BY i.start_date,i.id LIMIT sqlc.arg('page_limit');
