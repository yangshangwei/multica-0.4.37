-- The workflow owner holds the current account, source, issue and session
-- locks and merges only these fields from the locked issue. Ordinary writers
-- cannot reach this path or trigger execution after it commits.
-- name: UpdateCollaborationIssueFields :one
UPDATE issue SET title = sqlc.arg(title)::text,
    description = sqlc.narg(description)::text,
    start_date = sqlc.narg(start_date)::date,
    due_date = sqlc.narg(due_date)::date,
    revision = revision + 1, updated_at = clock_timestamp()
WHERE id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id = issue.id)
RETURNING *;

-- Content/date changes advance the edit version without revoking authority or
-- changing execution generations, lifecycle or the information request chain.
-- name: AdvanceCollaborationEditRevision :one
UPDATE issue_collaboration SET revision = revision + 1
WHERE issue_id = $1 RETURNING revision;
