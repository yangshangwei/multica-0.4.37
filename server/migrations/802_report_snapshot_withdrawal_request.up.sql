CREATE UNIQUE INDEX CONCURRENTLY report_snapshot_withdrawal_request_key ON report_snapshot_control (workspace_id, withdrawn_by, withdrawal_request_key) WHERE withdrawal_request_key IS NOT NULL;
