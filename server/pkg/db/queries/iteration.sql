-- name: GetIterationSettings :one
SELECT * FROM workspace_iteration_settings WHERE workspace_id = $1;

-- name: EnsureIterationSettings :exec
INSERT INTO workspace_iteration_settings (workspace_id) VALUES ($1)
ON CONFLICT (workspace_id) DO NOTHING;

-- name: LockIterationSettings :one
SELECT * FROM workspace_iteration_settings WHERE workspace_id = $1 FOR UPDATE;

-- name: GetIteration :one
SELECT * FROM iteration WHERE workspace_id = $1 AND id = $2;

-- name: LockIterations :many
SELECT * FROM iteration WHERE workspace_id = $1 AND id = ANY($2::uuid[])
ORDER BY id FOR UPDATE;

-- name: ListIterationParticipations :many
SELECT * FROM iteration_participation WHERE workspace_id = $1 AND iteration_id = $2 ORDER BY issue_id;

-- name: IssueHasIterationHistory :one
SELECT EXISTS (SELECT 1 FROM iteration_participation WHERE workspace_id = $1 AND issue_id = $2);

-- name: ListIterationEvents :many
SELECT * FROM iteration_event WHERE workspace_id = $1 AND iteration_id = $2
AND sequence > $3 ORDER BY sequence LIMIT $4;

-- name: GetIterationSnapshot :one
SELECT * FROM iteration_snapshot WHERE workspace_id = $1 AND iteration_id = $2;

-- name: GetIterationOperation :one
SELECT * FROM iteration_operation WHERE workspace_id = $1 AND actor_user_id = $2 AND request_id = $3;

-- name: InsertIterationOperation :one
INSERT INTO iteration_operation (id, workspace_id, actor_user_id, request_id, operation, payload_hash, result, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: GetWorkspacePlanningTimezone :one
SELECT planning_timezone FROM workspace WHERE id = $1;

-- name: ListIterationCurrentIssues :many
SELECT * FROM issue WHERE workspace_id = $1 AND current_iteration_id = $2 ORDER BY id;

-- name: LockIterationCurrentIssues :many
SELECT * FROM issue WHERE workspace_id = $1 AND current_iteration_id = $2 ORDER BY id FOR UPDATE;

-- name: SetIssueCurrentIteration :one
-- Caller holds the workspace iteration fence and the issue row lock. Admission,
-- target eligibility, participation and event writes belong to that transaction.
UPDATE issue SET current_iteration_id = sqlc.narg('current_iteration_id'),
    iteration_rollover_count = sqlc.arg('rollover_count'), revision = revision + 1,
    updated_at = sqlc.arg('business_at'), last_activity_at = sqlc.arg('business_at')
WHERE workspace_id = sqlc.arg('workspace_id') AND id = sqlc.arg('issue_id')
    AND revision = sqlc.arg('expected_revision')
    AND admission_status IN ('not_required', 'accepted')
RETURNING *;
