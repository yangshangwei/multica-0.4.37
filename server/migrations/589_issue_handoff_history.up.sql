CREATE INDEX CONCURRENTLY issue_handoff_history_idx ON issue_handoff (issue_id, created_at, id);
