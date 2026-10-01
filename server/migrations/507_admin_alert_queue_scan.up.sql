CREATE INDEX CONCURRENTLY admin_alert_queue_scan_idx ON agent_task_queue (queued_at,id) WHERE status='queued';
