CREATE UNIQUE INDEX CONCURRENTLY report_snapshot_cycle_key ON report_snapshot (workspace_id, report_issue_id, cycle_digest) WHERE correction_of IS NULL;
