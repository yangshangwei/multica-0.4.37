-- name: GetCollaborationRecipientSource :one
-- Ordinary source members may create issues. Do not infer a creation context
-- from an ambient workspace header or from a dangling membership alone.
SELECT w.id
FROM workspace w
JOIN member m ON m.workspace_id = w.id AND m.user_id = sqlc.arg(actor_id)::uuid
WHERE w.id = sqlc.arg(source_workspace_id)::uuid;

-- name: FindCollaborationRecipients :many
-- This is an exact confirmation lookup, never a directory. Keep unavailable
-- accounts in the match set so historical normalized duplicates are ambiguous
-- rather than silently selecting whichever account remains enabled.
SELECT u.id AS user_id, u.name, u.avatar_url, u.email,
       (u.disabled_at IS NULL AND NOT COALESCE(c.must_change_password, false)
        AND (NOT sqlc.arg(require_password_credential)::boolean OR c.user_id IS NOT NULL))::boolean AS available,
       NOT EXISTS (SELECT 1 FROM member m
                   WHERE m.workspace_id = sqlc.arg(source_workspace_id)::uuid AND m.user_id = u.id) AS external
FROM "user" u
LEFT JOIN user_password_credential c ON c.user_id = u.id
WHERE (sqlc.arg(kind)::text = 'username' AND lower(btrim(c.username, sqlc.arg(trim_chars)::text)) = sqlc.arg(value)::text)
   OR (sqlc.arg(kind)::text = 'email' AND lower(btrim(u.email, sqlc.arg(trim_chars)::text)) = sqlc.arg(value)::text)
   OR (sqlc.arg(kind)::text = 'user_id' AND u.id = sqlc.narg(user_id)::uuid)
ORDER BY u.id
LIMIT 2;
