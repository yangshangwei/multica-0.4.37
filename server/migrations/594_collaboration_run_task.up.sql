CREATE UNIQUE INDEX CONCURRENTLY collaboration_run_task_idx ON collaboration_run (task_id) WHERE task_id IS NOT NULL;
