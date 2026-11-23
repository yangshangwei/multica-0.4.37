CREATE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_attempt_open_idx ON issue_budget_attempt (budget_root_id) WHERE state = 'open';
