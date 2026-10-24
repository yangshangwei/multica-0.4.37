-- Catalog create/archive take the shared advisory fence's exclusive counterpart.
-- Existing metadata updates do not, so retain row locks as well. NOWAIT avoids
-- reverse waits against a multi-row catalog reorder while source locks are held.
-- name: LockCollaborationStatusEntries :many
SELECT * FROM issue_status WHERE workspace_id = $1 ORDER BY key FOR SHARE NOWAIT;

-- name: AdvanceCollaborationLifecycle :one
UPDATE issue_collaboration SET revision = revision + 1,
    authority_epoch = authority_epoch + 1, lifecycle_epoch = lifecycle_epoch + 1
WHERE issue_id = $1 RETURNING *;

-- Manual nonterminal status editing changes no authorization or execution epoch.
-- name: AdvanceCollaborationContentRevision :one
UPDATE issue_collaboration SET revision = revision + 1 WHERE issue_id = $1 RETURNING revision;

-- Both operations retain the permanent session, grant binding and old run barrier.
-- name: CloseCollaborationSession :one
UPDATE collaboration_session SET queue_state = 'closed',
    pause_reason = sqlc.arg(reason)::text, control_version = control_version + 1
WHERE id = sqlc.arg(session_id)::uuid RETURNING *;

-- name: ReopenCollaborationSession :one
UPDATE collaboration_session SET queue_state = 'paused',
    pause_reason = 'issue_reopened', control_version = control_version + 1
WHERE id = $1 AND queue_state = 'closed' RETURNING *;

-- Only waiting work is cancelled. An in-flight command is settled exclusively
-- by the stop/proof protocol, never by the business issue's terminal status.
-- name: CancelCollaborationWaitingCommands :many
UPDATE collaboration_command SET state = 'cancelled', reason = sqlc.arg(reason)::text
WHERE session_id = sqlc.arg(session_id)::uuid AND state = 'waiting' AND group_id IS NULL
RETURNING *;
