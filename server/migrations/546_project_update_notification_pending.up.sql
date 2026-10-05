CREATE INDEX CONCURRENTLY IF NOT EXISTS project_update_notification_pending ON project_update_notification (next_attempt_at) WHERE status = 'pending';
