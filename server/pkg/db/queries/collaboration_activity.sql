-- Reads run inside the same repeatable-read snapshot as object authorization.
-- The service projects allowed fields; stored JSON is never an HTTP DTO.
-- name: ListCollaborationActivityEvents :many
SELECT id, issue_id, event_seq, actor_user_id, kind, payload, created_at
FROM collaboration_event
WHERE issue_id = sqlc.arg(issue_id)::uuid
  AND (sqlc.narg(before_seq)::bigint IS NULL OR event_seq < sqlc.narg(before_seq)::bigint)
ORDER BY event_seq DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: ListCollaborationActivityHandoffs :many
SELECT id, actor_user_id, from_user_id, to_user_id, purpose, note, created_at
FROM issue_handoff
WHERE issue_id = sqlc.arg(issue_id)::uuid
  AND id = ANY(sqlc.arg(handoff_ids)::uuid[])
ORDER BY id;
