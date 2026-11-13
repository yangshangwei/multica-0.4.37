CREATE TABLE report_snapshot_control (
    workspace_id uuid NOT NULL,
    report_issue_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    current_snapshot_id uuid,
    current_sequence bigint NOT NULL DEFAULT 0 CHECK (current_sequence >= 0),
    withdrawal_id uuid,
    withdrawn_by uuid,
    withdrawal_request_key uuid,
    withdrawal_request_digest text,
    withdrawal_expected_snapshot_id uuid,
    withdrawal_report_revision bigint,
    withdrawal_reason text,
    withdrawn_at timestamptz,
    CHECK ((current_snapshot_id IS NULL) = (current_sequence = 0)),
    CHECK (
        (withdrawn_at IS NULL AND withdrawal_id IS NULL AND withdrawn_by IS NULL
         AND withdrawal_request_key IS NULL AND withdrawal_request_digest IS NULL
         AND withdrawal_expected_snapshot_id IS NULL AND withdrawal_report_revision IS NULL
         AND withdrawal_reason IS NULL)
        OR
        (withdrawn_at IS NOT NULL AND withdrawal_id IS NOT NULL AND withdrawn_by IS NOT NULL
         AND withdrawal_request_key IS NOT NULL AND withdrawal_request_digest IS NOT NULL
         AND withdrawal_expected_snapshot_id IS NOT NULL AND withdrawal_report_revision IS NOT NULL AND withdrawal_report_revision > 0
         AND withdrawal_reason IS NOT NULL AND length(btrim(withdrawal_reason)) > 0 AND octet_length(withdrawal_reason) <= 4096
         AND current_snapshot_id IS NOT NULL)
    )
);
