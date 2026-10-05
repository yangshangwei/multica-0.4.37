CREATE INDEX CONCURRENTLY iteration_issue_current ON issue (workspace_id, current_iteration_id) WHERE current_iteration_id IS NOT NULL;
