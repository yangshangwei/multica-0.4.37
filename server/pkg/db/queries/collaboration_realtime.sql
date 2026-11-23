-- The retained event is the outbox. Claim one committed record at a time so
-- a failed relay publish leaves it available to another worker after rollback.
-- name: LockNextCollaborationRealtimeEvent :one
SELECT e.id, e.issue_id, e.event_seq, e.kind, e.actor_user_id,
       c.revision, c.authority_epoch,
       CASE WHEN e.kind IN ('participant_removed', 'participant_left')
            THEN COALESCE(e.payload->>'target_user_id', '') ELSE '' END::text AS revoked_user_id
FROM collaboration_event e
JOIN issue_collaboration c ON c.issue_id=e.issue_id
WHERE e.outbox_state='pending'
ORDER BY e.created_at, e.id
LIMIT 1 FOR UPDATE OF e SKIP LOCKED;

-- Account invalidations contain no issue data. The original notification
-- recipients include a removed viewer who must clear their old workbench.
-- name: ListCollaborationRealtimeAccounts :many
SELECT p.user_id FROM issue_participant p
WHERE p.issue_id=sqlc.arg(issue_id)::uuid AND p.revoked_at IS NULL
UNION
SELECT n.recipient_user_id FROM collaboration_notification n
WHERE n.issue_id=sqlc.arg(issue_id)::uuid AND n.event_seq=sqlc.arg(event_seq)::bigint;

-- name: MarkCollaborationRealtimeEventDelivered :exec
UPDATE collaboration_event SET outbox_state='delivered' WHERE id=$1 AND outbox_state='pending';
