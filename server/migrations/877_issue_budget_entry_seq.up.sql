CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_entry_seq_key ON issue_budget_entry (budget_root_id, seq);
