-- Recipient identity and mention status are frozen in the business transaction.
-- A retry never re-expands a later participant list.
-- name: CreateCollaborationContentNotification :one
INSERT INTO collaboration_notification
    (recipient_user_id, issue_id, event_seq, kind, is_mention, source_kind, source_id)
VALUES (sqlc.arg(recipient_id)::uuid, sqlc.arg(issue_id)::uuid, sqlc.arg(event_seq)::bigint,
        sqlc.arg(kind)::text, sqlc.arg(is_mention)::boolean, sqlc.arg(source_kind)::text, sqlc.narg(source_id)::uuid)
ON CONFLICT (recipient_user_id, issue_id, event_seq) DO NOTHING
RETURNING *;

-- name: ListCollaborationNotifications :many
SELECT sqlc.embed(n), COALESCE(i.title, '')::text AS issue_title,
       (n.kind <> 'removed' AND i.id IS NOT NULL AND (
           EXISTS (SELECT 1 FROM issue_participant p WHERE p.issue_id = n.issue_id
                   AND p.user_id = n.recipient_user_id AND p.revoked_at IS NULL)
           OR EXISTS (SELECT 1 FROM member m WHERE m.workspace_id = i.workspace_id
                      AND m.user_id = n.recipient_user_id)
       ))::boolean AS can_open
FROM collaboration_notification n
LEFT JOIN issue i ON i.id = n.issue_id
WHERE n.recipient_user_id = sqlc.arg(recipient_id)::uuid
  AND (sqlc.arg(include_archived)::boolean OR n.archived_at IS NULL)
  AND (sqlc.narg(before_at)::timestamptz IS NULL
       OR (n.created_at, n.id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY n.created_at DESC, n.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountUnreadCollaborationNotifications :one
SELECT count(*)::bigint FROM collaboration_notification
WHERE recipient_user_id = sqlc.arg(recipient_id)::uuid AND read_at IS NULL AND archived_at IS NULL;

-- name: MarkCollaborationNotificationRead :one
UPDATE collaboration_notification SET read_at = COALESCE(read_at, now())
WHERE id = sqlc.arg(notification_id)::uuid AND recipient_user_id = sqlc.arg(recipient_id)::uuid
RETURNING *;

-- name: ArchiveCollaborationNotification :one
UPDATE collaboration_notification SET archived_at = COALESCE(archived_at, now())
WHERE id = sqlc.arg(notification_id)::uuid AND recipient_user_id = sqlc.arg(recipient_id)::uuid
RETURNING *;
