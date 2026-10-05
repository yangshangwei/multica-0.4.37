CREATE INDEX CONCURRENTLY issue_triage_queue ON issue_triage (workspace_id, entered_at, issue_id);
