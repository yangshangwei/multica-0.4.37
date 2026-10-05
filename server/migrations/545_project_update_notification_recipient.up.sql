CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_update_notification_recipient ON project_update_notification (workspace_id, update_id, recipient_user_id);
