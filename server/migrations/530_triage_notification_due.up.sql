CREATE INDEX CONCURRENTLY triage_notification_due ON triage_notification (due_at) WHERE delivered_at IS NULL;
