-- name: CreateAgentApprovalRequest :one
INSERT INTO agent_approval_request (
    workspace_id, agent_id, task_id, issue_id,
    risk_class, summary, plan
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAgentApprovalRequest :one
SELECT * FROM agent_approval_request WHERE id = $1;

-- name: ListAgentApprovalRequests :many
-- Workspace review queue. `status` is optional so one query serves both the
-- "what needs me" filter and the full audit trail.
SELECT * FROM agent_approval_request
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC
LIMIT $2;

-- name: ListAgentApprovalRequestsByAgent :many
-- What one agent has asked for. This is the read an agent itself performs to
-- find out whether the human decided yet.
SELECT * FROM agent_approval_request
WHERE agent_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC
LIMIT $2;

-- name: DecideAgentApprovalRequest :one
-- The `status = 'pending'` predicate is the concurrency guard: two reviewers
-- deciding at once means the second one updates zero rows and the handler
-- reports the conflict instead of overwriting the first decision.
UPDATE agent_approval_request SET
    status = $2,
    decided_by = $3,
    decided_at = now(),
    decision_note = $4,
    updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkAgentApprovalRequestExecuted :one
-- Only an approved request can be executed, enforced here rather than in Go so
-- a retry or a second runtime cannot walk a rejected request into 'executed'.
UPDATE agent_approval_request SET
    status = 'executed',
    executed_at = now(),
    execution_note = $2,
    updated_at = now()
WHERE id = $1 AND status = 'approved'
RETURNING *;

-- name: CancelAgentApprovalRequest :one
UPDATE agent_approval_request SET
    status = 'cancelled',
    updated_at = now()
WHERE id = $1 AND status IN ('pending', 'approved')
RETURNING *;

-- name: CountPendingAgentApprovalRequests :one
SELECT COUNT(*) FROM agent_approval_request
WHERE workspace_id = $1 AND status = 'pending';
