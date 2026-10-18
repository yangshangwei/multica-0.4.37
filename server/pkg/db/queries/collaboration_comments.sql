-- All content writers hold the source issue lock and current actor authority.
-- Dedicated comments remain ineligible for the ordinary dispatch listeners.
-- name: CreateCollaborationComment :one
WITH source AS (
    UPDATE issue SET updated_at = now(), revision = revision + 1,
        last_activity_at = GREATEST(COALESCE(last_activity_at, updated_at), now())
    WHERE id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
    RETURNING id, workspace_id
)
INSERT INTO comment (id, issue_id, workspace_id, author_type, author_id, content, type, parent_id, dispatch_eligible)
SELECT sqlc.arg(comment_id)::uuid, source.id, source.workspace_id, 'member',
       sqlc.arg(author_id)::uuid, sqlc.arg(content)::text, 'comment', sqlc.narg(parent_id)::uuid, false
FROM source
RETURNING *;

-- name: GetCollaborationComment :one
SELECT * FROM comment
WHERE id = sqlc.arg(comment_id)::uuid AND issue_id = sqlc.arg(issue_id)::uuid
  AND workspace_id = sqlc.arg(workspace_id)::uuid;

-- name: ListCollaborationComments :many
SELECT * FROM comment
WHERE issue_id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND (sqlc.narg(before_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: EditCollaborationComment :one
UPDATE comment SET content = sqlc.arg(content)::text,
    revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg(comment_id)::uuid AND issue_id = sqlc.arg(issue_id)::uuid
  AND workspace_id = sqlc.arg(workspace_id)::uuid AND author_type = 'member'
  AND author_id = sqlc.arg(author_id)::uuid AND revision = sqlc.arg(expected_revision)::bigint
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCollaborationComment :one
UPDATE comment SET content = '', deleted_at = now(), revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg(comment_id)::uuid AND issue_id = sqlc.arg(issue_id)::uuid
  AND workspace_id = sqlc.arg(workspace_id)::uuid AND author_type = 'member'
  AND author_id = sqlc.arg(author_id)::uuid AND revision = sqlc.arg(expected_revision)::bigint
  AND deleted_at IS NULL
RETURNING *;

-- name: CreateCollaborationContentEvent :one
INSERT INTO collaboration_event (id, issue_id, event_seq, actor_user_id, kind, payload)
VALUES (sqlc.arg(event_id)::uuid, sqlc.arg(issue_id)::uuid, sqlc.arg(event_seq)::bigint,
        sqlc.arg(actor_id)::uuid, sqlc.arg(kind)::text, sqlc.arg(payload)::jsonb)
RETURNING *;

-- name: ListCollaborationCommentHistory :many
SELECT * FROM collaboration_event
WHERE issue_id = sqlc.arg(issue_id)::uuid
  AND kind IN ('comment_created', 'comment_edited', 'comment_deleted')
  AND payload->>'comment_id' = sqlc.arg(comment_id)::text
ORDER BY event_seq;

-- name: ListCollaborationContentAccounts :many
SELECT u.id AS user_id, u.name, COALESCE(u.avatar_url, '')::text AS avatar_url,
       NOT EXISTS (SELECT 1 FROM member m WHERE m.user_id = u.id AND m.workspace_id = sqlc.arg(workspace_id)::uuid)::boolean AS external
FROM "user" u
WHERE u.id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY u.id;

-- name: ListCollaborationMentionParticipants :many
SELECT u.id AS user_id, u.name, COALESCE(u.avatar_url, '')::text AS avatar_url, u.email,
       NOT EXISTS (SELECT 1 FROM member m WHERE m.user_id = u.id AND m.workspace_id = sqlc.arg(workspace_id)::uuid)::boolean AS external
FROM issue_participant p JOIN "user" u ON u.id = p.user_id
WHERE p.issue_id = sqlc.arg(issue_id)::uuid AND p.revoked_at IS NULL AND u.disabled_at IS NULL
ORDER BY u.id;
