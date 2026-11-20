CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_budget_operation_scope_key ON issue_budget_operation (workspace_id, scope_kind, scope_id, operation_key);
