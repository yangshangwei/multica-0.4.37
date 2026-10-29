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

-- name: SearchCollaborationRecipients :many
-- A picker page, not authority: invitation, transfer and fields recheck the
-- chosen account. Every availability rule, including the emergency denylist,
-- applies before LIMIT so keyset pages are never short and never skip rows.
SELECT u.id AS user_id, u.name, u.avatar_url, lower(u.name)::text AS sort_name,
       NOT EXISTS (SELECT 1 FROM member m
                   WHERE m.workspace_id = sqlc.arg(source_workspace_id)::uuid AND m.user_id = u.id) AS external
FROM "user" u
LEFT JOIN user_password_credential c ON c.user_id = u.id
WHERE u.id <> sqlc.arg(actor_id)::uuid
  AND u.disabled_at IS NULL AND NOT COALESCE(c.must_change_password, false)
  AND (NOT sqlc.arg(require_password_credential)::boolean OR c.user_id IS NOT NULL)
  AND u.id::text <> ALL(sqlc.arg(temporarily_disabled_user_ids)::text[])
  -- A NULL email must not turn the comparison into NULL and drop the row.
  AND COALESCE(lower(btrim(u.email, sqlc.arg(trim_chars)::text)), '') <> ALL(sqlc.arg(temporarily_disabled_user_emails)::text[])
  AND (sqlc.arg(directory)::boolean
       OR EXISTS (SELECT 1 FROM member mine JOIN member theirs ON theirs.workspace_id = mine.workspace_id
                  WHERE mine.user_id = sqlc.arg(actor_id)::uuid AND theirs.user_id = u.id)
       OR EXISTS (SELECT 1 FROM issue_participant mine JOIN issue_participant theirs ON theirs.issue_id = mine.issue_id
                  WHERE mine.user_id = sqlc.arg(actor_id)::uuid AND theirs.user_id = u.id
                    AND theirs.joined_at <= COALESCE(mine.revoked_at, now())))
  AND (NOT sqlc.arg(members_only)::boolean
       OR EXISTS (SELECT 1 FROM member m
                  WHERE m.workspace_id = sqlc.arg(source_workspace_id)::uuid AND m.user_id = u.id))
  -- Names only. Email, username and ID stay with the exact lookup.
  AND lower(u.name) LIKE '%' || lower(sqlc.arg(name_pattern)::text) || '%' ESCAPE '\'
  AND (sqlc.narg(after_id)::uuid IS NULL
       OR (lower(u.name), u.id) > (sqlc.arg(after_name)::text, sqlc.narg(after_id)::uuid))
ORDER BY lower(u.name), u.id
LIMIT sqlc.arg(page_limit)::int;

-- name: ListRecentCollaborationRecipients :many
-- People who shared a collaboration issue with the actor while the actor was
-- still on it. Someone who joined after the actor left was never seen there.
SELECT u.id AS user_id, u.name, u.avatar_url,
       NOT EXISTS (SELECT 1 FROM member m
                   WHERE m.workspace_id = sqlc.arg(source_workspace_id)::uuid AND m.user_id = u.id) AS external
FROM issue_participant mine
JOIN issue_participant theirs ON theirs.issue_id = mine.issue_id
JOIN "user" u ON u.id = theirs.user_id
LEFT JOIN user_password_credential c ON c.user_id = u.id
WHERE mine.user_id = sqlc.arg(actor_id)::uuid AND u.id <> sqlc.arg(actor_id)::uuid
  AND theirs.joined_at <= COALESCE(mine.revoked_at, now())
  AND u.disabled_at IS NULL AND NOT COALESCE(c.must_change_password, false)
  AND (NOT sqlc.arg(require_password_credential)::boolean OR c.user_id IS NOT NULL)
  AND u.id::text <> ALL(sqlc.arg(temporarily_disabled_user_ids)::text[])
  AND COALESCE(lower(btrim(u.email, sqlc.arg(trim_chars)::text)), '') <> ALL(sqlc.arg(temporarily_disabled_user_emails)::text[])
  AND (NOT sqlc.arg(members_only)::boolean
       OR EXISTS (SELECT 1 FROM member m
                  WHERE m.workspace_id = sqlc.arg(source_workspace_id)::uuid AND m.user_id = u.id))
GROUP BY u.id
ORDER BY max(greatest(mine.joined_at, theirs.joined_at)) DESC, u.id
LIMIT sqlc.arg(recent_limit)::int;
