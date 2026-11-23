CREATE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_observation_gap_idx ON issue_budget_observation (budget_root_id, reason) WHERE kind = 'coverage_gap';
