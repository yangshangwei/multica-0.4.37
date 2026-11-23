CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_member_issue_key ON issue_budget_member (workspace_id, issue_id);
