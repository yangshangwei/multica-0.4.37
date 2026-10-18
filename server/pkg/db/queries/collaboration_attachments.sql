-- name: CreateCollaborationAttachment :one
INSERT INTO collaboration_attachment (
    id, draft_id, uploader_user_id, source_workspace_id, intended_issue_id,
    upload_request_id, storage_key, content_sha256, encryption_key,
    name, content_type, size_bytes
) VALUES (
    sqlc.arg(id), sqlc.arg(draft_id), sqlc.arg(uploader_user_id), sqlc.arg(source_workspace_id), sqlc.narg(intended_issue_id),
    sqlc.arg(upload_request_id), sqlc.arg(storage_key), sqlc.arg(content_sha256), sqlc.arg(encryption_key),
    sqlc.arg(name), sqlc.arg(content_type), sqlc.arg(size_bytes)
) RETURNING *;

-- name: GetCollaborationAttachmentUpload :one
SELECT * FROM collaboration_attachment
WHERE uploader_user_id = sqlc.arg(uploader_user_id)
  AND draft_id = sqlc.arg(draft_id) AND upload_request_id = sqlc.arg(upload_request_id);

-- name: GetCollaborationAttachment :one
SELECT * FROM collaboration_attachment WHERE id = $1;

-- name: LockCollaborationAttachments :many
SELECT * FROM collaboration_attachment WHERE id = ANY($1::uuid[]) ORDER BY id FOR UPDATE;

-- name: ListCollaborationAttachments :many
SELECT * FROM collaboration_attachment
WHERE issue_id = $1 AND delivered_at IS NOT NULL AND discarded_at IS NULL
ORDER BY created_at, id;

-- name: ListCollaborationDraftAttachments :many
SELECT * FROM collaboration_attachment
WHERE draft_id = sqlc.arg(draft_id) AND uploader_user_id = sqlc.arg(uploader_user_id)
  AND issue_id IS NULL AND discarded_at IS NULL
ORDER BY created_at, id;

-- Every delivery row retains its upload identity, including discarded files and
-- delivered files. A delivered row is the permanent whole-draft consumption
-- marker. Writers hold the uploader's user-row lock before reading this state,
-- so another upload or binding cannot insert behind this snapshot.
-- name: GetCollaborationAttachmentDraftState :one
SELECT count(*) FILTER (WHERE issue_id IS NULL AND discarded_at IS NULL)::bigint AS undelivered_count,
       COALESCE(bool_or(issue_id IS NOT NULL), false)::boolean AS consumed,
       COALESCE(bool_and(source_workspace_id IS NOT DISTINCT FROM sqlc.arg(source_workspace_id)::uuid
           AND intended_issue_id IS NOT DISTINCT FROM sqlc.narg(intended_issue_id)::uuid), true)::boolean AS context_matches
FROM collaboration_attachment
WHERE draft_id = sqlc.arg(draft_id)::uuid AND uploader_user_id = sqlc.arg(uploader_user_id)::uuid
  AND upload_request_id IS NOT NULL;

-- name: DeliverCollaborationAttachment :one
UPDATE collaboration_attachment SET issue_id = sqlc.arg(issue_id), delivered_at = clock_timestamp()
WHERE id = sqlc.arg(id) AND issue_id IS NULL AND discarded_at IS NULL
RETURNING *;

-- name: DiscardCollaborationDraftAttachment :one
UPDATE collaboration_attachment SET discarded_at = COALESCE(discarded_at, clock_timestamp())
WHERE id = sqlc.arg(id) AND draft_id = sqlc.arg(draft_id)
  AND uploader_user_id = sqlc.arg(uploader_user_id) AND issue_id IS NULL
RETURNING *;

-- name: CreateCollaborationAttachmentReference :exec
INSERT INTO collaboration_attachment_reference (attachment_id, reference_kind, reference_id, reference_revision)
VALUES (sqlc.arg(attachment_id), sqlc.arg(reference_kind), sqlc.arg(reference_id), sqlc.arg(reference_revision));

-- name: RemoveCollaborationCommentAttachmentReferences :exec
UPDATE collaboration_attachment_reference SET removed_at = clock_timestamp()
WHERE reference_kind = 'comment' AND reference_id = $1 AND removed_at IS NULL;

-- name: ListCollaborationCommentAttachmentReferences :many
SELECT a.id AS attachment_id, r.reference_id AS comment_id
FROM collaboration_attachment_reference r
JOIN collaboration_attachment a ON a.id = r.attachment_id
WHERE r.reference_kind = 'comment' AND r.reference_id = ANY(sqlc.arg(comment_ids)::uuid[])
  AND r.removed_at IS NULL AND a.issue_id = sqlc.arg(issue_id)
  AND a.delivered_at IS NOT NULL AND a.discarded_at IS NULL
ORDER BY r.reference_id, a.created_at, a.id;

-- name: GetCollaborationAttachmentReadBasis :one
SELECT f.epoch AS account_epoch,
       (SELECT m.id FROM member m WHERE m.workspace_id = sqlc.arg(source_workspace_id) AND m.user_id = sqlc.arg(viewer_id))::uuid AS member_id,
       COALESCE((SELECT p.epoch FROM issue_participant p WHERE p.issue_id = sqlc.arg(issue_id) AND p.user_id = sqlc.arg(viewer_id) AND p.revoked_at IS NULL), 0)::bigint AS participant_epoch
FROM collaboration_account_fence f WHERE f.user_id = sqlc.arg(viewer_id);

-- name: ListCollaborationCreationSources :many
SELECT w.id, w.name, w.slug FROM member m
JOIN workspace w ON w.id = m.workspace_id
WHERE m.user_id = $1 ORDER BY lower(w.name), w.id;
