-- Source catalogs are only read after current source-member/editor authority.
-- Return a bounded, searchable reference list; never project issue content.
-- name: ListCollaborationSourceParents :many
SELECT id, title, number, parent_issue_id FROM issue
WHERE workspace_id = sqlc.arg(workspace_id)::uuid
  AND admission_status IN ('not_required', 'accepted')
  AND id IS DISTINCT FROM sqlc.narg(excluded_issue_id)::uuid
  AND (sqlc.arg(search)::text = ''
    OR strpos(lower(title), lower(sqlc.arg(search)::text)) > 0
    OR number::text = sqlc.arg(search)::text
    OR id::text = lower(sqlc.arg(search)::text))
ORDER BY number DESC, id
LIMIT 201;

-- Source/I1 locks precede this ancestor walk. A reference writer with the
-- opposite issue lock order must retry the whole transaction on contention.
-- name: LockCollaborationSourceParent :one
SELECT * FROM issue
WHERE workspace_id = sqlc.arg(workspace_id)::uuid AND id = sqlc.arg(issue_id)::uuid
FOR SHARE NOWAIT;

-- name: UpdateCollaborationIssueStructure :one
-- The owner also writes UpdateCollaborationIssueFields in this transaction,
-- which advances the ordinary issue revision exactly once for the whole edit.
UPDATE issue SET project_id = sqlc.narg(project_id)::uuid,
    parent_issue_id = sqlc.narg(parent_issue_id)::uuid,
    stage = sqlc.narg(stage)::integer
WHERE id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id = issue.id)
RETURNING *;
