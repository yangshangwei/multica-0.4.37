CREATE UNIQUE INDEX CONCURRENTLY report_snapshot_request_key ON report_snapshot (workspace_id, created_by, request_key);
