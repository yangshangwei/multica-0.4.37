CREATE INDEX CONCURRENTLY admin_execution_finished_page_idx ON agent_task_queue (completed_at DESC,id DESC) WHERE completed_at IS NOT NULL;
