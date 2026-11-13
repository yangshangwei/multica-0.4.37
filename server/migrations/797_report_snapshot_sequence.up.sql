CREATE UNIQUE INDEX CONCURRENTLY report_snapshot_sequence_key ON report_snapshot (workspace_id, report_issue_id, sequence);
