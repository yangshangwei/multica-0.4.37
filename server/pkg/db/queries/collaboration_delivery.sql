-- The caller owns a repeatable read or the issue-collaboration writer lock.
-- No timestamp join or current comment text can reconstruct a submission.
-- name: ListCollaborationDeliveryEvents :many
SELECT * FROM collaboration_event
WHERE issue_id = sqlc.arg(issue_id)::uuid AND kind = 'acceptance_submitted'
  AND event_seq <= sqlc.arg(through_seq)::bigint
  AND (sqlc.narg(before_seq)::bigint IS NULL OR event_seq < sqlc.narg(before_seq)::bigint)
ORDER BY event_seq DESC LIMIT sqlc.arg(row_limit)::int;

-- name: ListCollaborationDeliveryHandoffs :many
SELECT * FROM issue_handoff
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = ANY(sqlc.arg(handoff_ids)::uuid[])
  AND purpose = 'acceptance';

-- name: ListCollaborationDeliveryDecisions :many
SELECT * FROM collaboration_event
WHERE issue_id = sqlc.arg(issue_id)::uuid
  AND kind IN ('completed', 'cancelled', 'acceptance_returned')
  AND payload->'delivery_decision'->>'submission_id' = ANY(sqlc.arg(submission_ids)::text[])
ORDER BY event_seq DESC LIMIT sqlc.arg(row_limit)::int;

-- name: GetCollaborationDeliveryLifecycleBoundary :one
SELECT COALESCE(max(event_seq), 0)::bigint FROM collaboration_event
WHERE issue_id = $1 AND kind IN ('completed', 'cancelled', 'reopened');

-- name: ListCollaborationDeliveryAttachments :many
SELECT * FROM collaboration_attachment
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = ANY(sqlc.arg(attachment_ids)::uuid[])
  AND discarded_at IS NULL;

-- name: UpdateCollaborationAIPolicy :exec
UPDATE issue_collaboration SET ai_policy = sqlc.arg(ai_policy)::text
WHERE issue_id = sqlc.arg(issue_id)::uuid;
