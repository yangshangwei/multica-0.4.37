-- name: CountCollaborationRecallComments :one
SELECT count(*) FROM comment
WHERE issue_id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND (sqlc.arg(include_deleted)::boolean OR deleted_at IS NULL);

-- name: ListCollaborationRecallComments :many
SELECT * FROM comment
WHERE issue_id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND deleted_at IS NULL
  AND (sqlc.narg(before_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;

-- Current author facts are not historical command admission or viewer authority.
-- name: ListCollaborationCommentAuthorFacts :many
SELECT u.id, u.email, (u.disabled_at IS NULL)::boolean AS account_active,
       EXISTS (SELECT 1 FROM member m WHERE m.user_id = u.id AND m.workspace_id = sqlc.arg(workspace_id)::uuid) AS source_member,
       EXISTS (SELECT 1 FROM issue_participant p WHERE p.user_id = u.id AND p.issue_id = sqlc.arg(issue_id)::uuid AND p.revoked_at IS NULL) AS participant
FROM "user" u WHERE u.id = ANY(sqlc.arg(user_ids)::uuid[]);
