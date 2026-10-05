CREATE UNIQUE INDEX CONCURRENTLY iteration_notification_operation ON iteration_notification (workspace_id, operation_id, recipient_user_id, kind);
