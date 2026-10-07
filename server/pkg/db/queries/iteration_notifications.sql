-- name: CreateIterationNotification :exec
INSERT INTO iteration_notification(workspace_id,iteration_id,operation_id,recipient_user_id,kind,local_date)
VALUES(sqlc.arg('workspace_id'),sqlc.arg('iteration_id'),sqlc.arg('operation_id'),sqlc.arg('recipient_user_id'),sqlc.arg('kind'),sqlc.narg('local_date'))
ON CONFLICT DO NOTHING;

-- Discovery is lock-free. Delivery locks workspace, recipient fence/member, iteration, outbox.
-- name: ListDueIterationNotifications :many
SELECT * FROM iteration_notification WHERE status='pending' AND next_attempt_at<=clock_timestamp()
ORDER BY next_attempt_at,id LIMIT 100;

-- name: LockIterationNotification :one
SELECT * FROM iteration_notification WHERE workspace_id=sqlc.arg('workspace_id') AND id=sqlc.arg('id')
AND status='pending' AND next_attempt_at<=clock_timestamp() FOR UPDATE SKIP LOCKED;

-- name: CompleteIterationNotification :exec
UPDATE iteration_notification SET status=sqlc.arg('status'),last_error_code=sqlc.narg('last_error_code')
WHERE workspace_id=sqlc.arg('workspace_id') AND id=sqlc.arg('id');

-- name: RetryIterationNotification :exec
UPDATE iteration_notification SET attempts=attempts+1,status=CASE WHEN attempts+1>=12 THEN 'dead_letter' ELSE 'pending' END,
next_attempt_at=clock_timestamp()+LEAST(900,5*power(2,LEAST(attempts,8))) * interval '1 second',last_error_code='delivery_failed'
WHERE workspace_id=sqlc.arg('workspace_id') AND id=sqlc.arg('id') AND status='pending';

-- name: CreateIterationInbox :one
INSERT INTO inbox_item(id,workspace_id,recipient_type,recipient_id,type,severity,title,body,actor_type,details)
VALUES(sqlc.arg('id'),sqlc.arg('workspace_id'),'member',sqlc.arg('recipient_id'),'iteration','info',sqlc.arg('title'),'','system',sqlc.arg('details'))
ON CONFLICT(id) DO NOTHING RETURNING *;

-- name: ListActiveIterationsForReminder :many
SELECT i.* FROM iteration i JOIN workspace_iteration_settings s ON s.workspace_id=i.workspace_id
JOIN member m ON m.workspace_id=i.workspace_id AND m.user_id=COALESCE(i.coordinator_user_id,i.started_by)
WHERE i.status='active' AND s.enabled
AND i.end_date < (sqlc.arg('sampled_at')::timestamptz AT TIME ZONE i.timezone)::date
AND NOT EXISTS (SELECT 1 FROM iteration_notification n
 WHERE n.iteration_id=i.id AND n.recipient_user_id=m.user_id AND n.kind='overdue'
 AND n.local_date=(sqlc.arg('sampled_at')::timestamptz AT TIME ZONE i.timezone)::date)
ORDER BY i.workspace_id,i.id LIMIT 100;

-- name: GetIterationNotificationCounts :one
SELECT count(*) FILTER (WHERE status='pending')::bigint AS pending,
 count(*) FILTER (WHERE status='dead_letter')::bigint AS dead_letter FROM iteration_notification
WHERE status IN ('pending','dead_letter');
