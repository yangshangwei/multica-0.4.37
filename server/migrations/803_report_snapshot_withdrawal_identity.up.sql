CREATE UNIQUE INDEX CONCURRENTLY report_snapshot_withdrawal_identity_key ON report_snapshot_control (withdrawal_id) WHERE withdrawal_id IS NOT NULL;
