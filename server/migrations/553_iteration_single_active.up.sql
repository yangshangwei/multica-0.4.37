CREATE UNIQUE INDEX CONCURRENTLY iteration_single_active ON iteration (workspace_id) WHERE status = 'active';
