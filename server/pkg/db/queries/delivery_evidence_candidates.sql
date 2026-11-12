-- The service applies the shared artifact authority to every row before paging.
-- The raw scan is bounded; hidden rows are never returned as cursor anchors.
-- name: ListDeliveryEvidenceCandidateScan :many
SELECT * FROM agent_artifact_execution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND resource_id = sqlc.arg(issue_id)
  AND resource_kind = 'issue'
  AND action = 'package_release_candidate'
  AND created_at <= sqlc.arg(upper_bound)::timestamptz
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL
       OR (created_at, approval_id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_approval_id)::uuid))
ORDER BY created_at DESC, approval_id DESC
LIMIT sqlc.arg(scan_limit);
