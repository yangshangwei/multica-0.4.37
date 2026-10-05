CREATE INDEX CONCURRENTLY triage_action_issue ON triage_action (workspace_id, issue_id, created_at, id);
