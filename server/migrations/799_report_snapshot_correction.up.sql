CREATE UNIQUE INDEX CONCURRENTLY report_snapshot_correction_key ON report_snapshot (workspace_id, report_issue_id, correction_of) WHERE correction_of IS NOT NULL;
