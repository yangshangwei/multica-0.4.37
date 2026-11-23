-- The workflow owner holds the current account, source, issue and session
-- locks and merges only these fields from the locked issue. Ordinary writers
-- cannot reach this path or trigger execution after it commits.
-- name: UpdateCollaborationIssueFields :one
UPDATE issue SET title = sqlc.arg(title)::text,
    description = sqlc.narg(description)::text,
    start_date = sqlc.narg(start_date)::date,
    due_date = sqlc.narg(due_date)::date,
    priority = sqlc.arg(priority)::text,
    properties = sqlc.arg(properties)::jsonb,
    revision = revision + 1, updated_at = clock_timestamp()
WHERE id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id = issue.id)
RETURNING *;

-- Content/date changes advance the edit version without revoking authority or
-- changing execution generations, lifecycle or the information request chain.
-- name: AdvanceCollaborationEditRevision :one
UPDATE issue_collaboration SET revision = revision + 1
WHERE issue_id = $1 RETURNING revision;

-- Metadata replaces the complete label set under the source I1 and issue
-- fences. The issue revision is advanced once by UpdateCollaborationIssueFields.
-- name: ClearCollaborationIssueLabels :exec
DELETE FROM issue_to_label il
USING issue i, issue_collaboration c
WHERE il.issue_id = i.id AND c.issue_id = i.id
  AND i.id = sqlc.arg(issue_id)::uuid
  AND i.workspace_id = sqlc.arg(workspace_id)::uuid;

-- name: InsertCollaborationIssueLabels :execrows
INSERT INTO issue_to_label (issue_id, label_id)
SELECT i.id, l.id
FROM issue i
JOIN issue_collaboration c ON c.issue_id = i.id
JOIN issue_label l ON l.workspace_id = i.workspace_id AND l.resource_type = 'issue'
WHERE i.id = sqlc.arg(issue_id)::uuid
  AND i.workspace_id = sqlc.arg(workspace_id)::uuid
  AND l.id = ANY(sqlc.arg(label_ids)::uuid[]);
