CREATE INDEX CONCURRENTLY admin_alert_failure_scan_idx ON agent_task_queue (completed_at,id) WHERE status='failed' AND completed_at IS NOT NULL;
