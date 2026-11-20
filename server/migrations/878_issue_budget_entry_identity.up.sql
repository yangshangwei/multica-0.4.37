CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_entry_identity_key ON issue_budget_entry (budget_root_id, entry_id);
