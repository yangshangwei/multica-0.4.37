CREATE INDEX CONCURRENTLY triage_action_history ON triage_action (workspace_id, created_at DESC, id);
