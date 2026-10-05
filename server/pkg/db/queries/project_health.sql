-- name: ListProjectHealthIssues :many
SELECT id, status, due_date, assignee_type, assignee_id
FROM issue
WHERE workspace_id = sqlc.arg('workspace_id')::uuid
  AND project_id = sqlc.arg('project_id')::uuid
  AND admission_status IN ('not_required', 'accepted')
ORDER BY id ASC;

-- name: GetProjectHealthIssueRows :many
SELECT * FROM issue
WHERE workspace_id = sqlc.arg('workspace_id')::uuid
  AND project_id = sqlc.arg('project_id')::uuid
  AND admission_status IN ('not_required', 'accepted')
  AND id = ANY(sqlc.arg('issue_ids')::uuid[])
ORDER BY id ASC;

-- name: GetProjectHealthLatestUpdate :one
SELECT max(published_at)::timestamptz AS latest_update_at
FROM project_update
WHERE workspace_id = sqlc.arg('workspace_id')::uuid
  AND project_id = sqlc.arg('project_id')::uuid;

-- name: ListProjectHealthAcceptanceSummaries :many
WITH acceptances AS (
  SELECT u.id, u.author_user_id, u.published_at, u.current_revision, r.acceptance
  FROM project_update u
  JOIN project_update_revision r
    ON r.workspace_id = u.workspace_id AND r.project_id = u.project_id
   AND r.update_id = u.id AND r.revision = u.current_revision
  WHERE u.workspace_id = sqlc.arg('workspace_id')::uuid
    AND u.project_id = sqlc.arg('project_id')::uuid AND r.kind = 'acceptance'
)
(SELECT * FROM acceptances ORDER BY published_at DESC, id DESC LIMIT 1)
UNION
(SELECT * FROM acceptances
 WHERE acceptance->>'description_revision' = sqlc.arg('description_revision')::bigint::text
 ORDER BY published_at DESC, id DESC LIMIT 1)
ORDER BY published_at DESC, id DESC;

-- name: ListProjectHealthStatusCounts :many
SELECT project_id, status, count(*)::bigint AS issue_count
FROM issue
WHERE workspace_id = sqlc.arg('workspace_id')::uuid
  AND project_id = ANY(sqlc.arg('project_ids')::uuid[])
  AND admission_status IN ('not_required', 'accepted')
GROUP BY project_id, status;
