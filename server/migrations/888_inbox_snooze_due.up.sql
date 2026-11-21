CREATE INDEX CONCURRENTLY idx_inbox_snooze_due ON inbox_snooze (snoozed_until, workspace_id, recipient_id, group_id) WHERE snoozed_until IS NOT NULL;
