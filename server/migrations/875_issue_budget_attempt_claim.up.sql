CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_attempt_claim_key ON issue_budget_attempt (task_id, claim_generation);
