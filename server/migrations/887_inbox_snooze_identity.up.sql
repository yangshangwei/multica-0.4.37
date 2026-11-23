CREATE UNIQUE INDEX CONCURRENTLY idx_inbox_snooze_identity ON inbox_snooze (workspace_id, recipient_type, recipient_id, group_kind, group_id);
