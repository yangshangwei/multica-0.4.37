CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_root_issue_key ON issue_budget_root (workspace_id, root_issue_id);
