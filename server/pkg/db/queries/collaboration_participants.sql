-- Management includes disabled accounts with an unrevoked participation row.
-- Mention and recipient queries intentionally retain their separate active filter.
-- name: ListCollaborationParticipantUsers :many
SELECT user_id FROM issue_participant
WHERE issue_id = $1 AND revoked_at IS NULL
ORDER BY joined_at, user_id;
