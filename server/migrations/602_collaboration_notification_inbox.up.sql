CREATE INDEX CONCURRENTLY collaboration_notification_inbox_idx ON collaboration_notification (recipient_user_id, archived_at, read_at, created_at, id);
