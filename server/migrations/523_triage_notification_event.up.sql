CREATE UNIQUE INDEX CONCURRENTLY triage_notification_event ON triage_notification (workspace_id, recipient_id, event_key);
