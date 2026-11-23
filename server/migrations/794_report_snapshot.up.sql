CREATE TABLE report_snapshot (
    id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    report_issue_id uuid NOT NULL,
    project_id uuid,
    report_revision bigint NOT NULL CHECK (report_revision > 0),
    sequence bigint NOT NULL CHECK (sequence > 0),
    created_by uuid NOT NULL,
    request_key uuid NOT NULL,
    request_digest text NOT NULL,
    cycle_digest text NOT NULL,
    correction_of uuid,
    captured_at timestamptz NOT NULL,
    snapshot_json bytea NOT NULL CHECK (octet_length(snapshot_json) <= 16777216),
    facts_digest text NOT NULL,
    result_digest text NOT NULL,
    snapshot_digest text NOT NULL
);
