CREATE INDEX CONCURRENTLY admin_operation_due_idx ON admin_operation (next_reconcile_at,state,id) WHERE kind='task.cancel' AND root_operation_id IS NULL AND next_reconcile_at IS NOT NULL;
