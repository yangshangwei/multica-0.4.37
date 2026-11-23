CREATE INDEX CONCURRENTLY collaboration_notification_page_idx ON collaboration_notification (recipient_user_id, created_at DESC, id DESC);
