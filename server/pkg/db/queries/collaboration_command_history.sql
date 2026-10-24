-- Every page borrows the current issue-authorized content transaction. Neither
-- editable comments nor live agent/runtime configuration supplies input history.
-- name: ListCollaborationCommandHistory :many
SELECT sqlc.embed(c), t.kind AS target_kind, t.resource_id,
    source.event_seq AS input_event_seq, source.payload AS input_payload,
    cancelled.id AS cancellation_id, cancelled.event_seq AS cancellation_seq,
    cancelled.actor_user_id AS cancellation_actor_id, cancelled.created_at AS cancellation_time,
    cancelled.payload AS cancellation_payload
FROM collaboration_command c
LEFT JOIN collaboration_target t ON t.id=c.target_id AND t.session_id=c.session_id
LEFT JOIN collaboration_event source ON source.id=c.input_event_id AND source.issue_id=sqlc.arg(issue_id)::uuid
    AND source.kind='comment_created'
LEFT JOIN collaboration_event cancelled ON cancelled.id=c.cancellation_event_id AND cancelled.issue_id=sqlc.arg(issue_id)::uuid
    AND cancelled.kind='command_cancelled'
WHERE c.session_id=sqlc.arg(session_id)::uuid
    AND (sqlc.narg(before_created_at)::timestamptz IS NULL OR (c.created_at,c.id)<(sqlc.narg(before_created_at)::timestamptz,sqlc.narg(before_id)::uuid))
ORDER BY c.created_at DESC,c.id DESC LIMIT sqlc.arg(row_limit)::int;

-- name: CollaborationCommandHistoryCursorExists :one
SELECT EXISTS(SELECT 1 FROM collaboration_command
    WHERE session_id=sqlc.arg(session_id)::uuid AND id=sqlc.arg(command_id)::uuid AND created_at=sqlc.arg(created_at)::timestamptz)::boolean;

-- State, reason, event and linkage are one caller-owned transaction. The exact
-- payload check prevents linking a cancelled command to another input's event.
-- name: LinkCollaborationCommandCancellation :one
UPDATE collaboration_command c SET cancellation_event_id=e.id
FROM collaboration_session s, collaboration_event e
WHERE c.session_id=sqlc.arg(session_id)::uuid AND c.id=sqlc.arg(command_id)::uuid
    AND c.state='cancelled' AND c.cancellation_event_id IS NULL
    AND s.id=c.session_id AND e.id=sqlc.arg(event_id)::uuid AND e.issue_id=s.issue_id AND e.kind='command_cancelled'
    AND e.payload->>'command_id'=c.id::text AND e.payload->>'target_id'=c.target_id::text
    AND e.payload->>'channel_id'=c.channel_id::text AND e.payload->>'batch_id'=c.batch_id::text
    AND e.payload->>'command_sequence'=c.seq::text AND e.payload->>'reason'=c.reason
    AND e.payload->>'cancellation_kind' IN ('manual','automatic')
RETURNING c.*;
