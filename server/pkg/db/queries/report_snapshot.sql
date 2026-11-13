-- name: CreateReportSnapshot :one
INSERT INTO report_snapshot (
 id,workspace_id,report_issue_id,project_id,report_revision,sequence,created_by,
 request_key,request_digest,cycle_digest,correction_of,captured_at,snapshot_json,
 facts_digest,result_digest,snapshot_digest,seal_digest
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
RETURNING *;

-- name: GetReportSnapshot :one
SELECT * FROM report_snapshot
WHERE workspace_id=$1 AND report_issue_id=$2 AND id=$3;

-- name: GetCurrentReportSnapshot :one
SELECT * FROM report_snapshot
WHERE workspace_id=$1 AND report_issue_id=$2
ORDER BY sequence DESC LIMIT 1;

-- name: GetReportSnapshotByRequest :one
SELECT * FROM report_snapshot
WHERE workspace_id=$1 AND created_by=$2 AND request_key=$3;

-- name: GetReportSnapshotCycle :one
SELECT * FROM report_snapshot
WHERE workspace_id=$1 AND report_issue_id=$2 AND cycle_digest=$3
ORDER BY sequence DESC LIMIT 1;

-- name: GetReportSnapshotReadCut :one
SELECT clock_timestamp()::timestamptz AS as_of,pg_current_snapshot()::text AS read_cut;

-- name: EnsureReportSnapshotControl :exec
INSERT INTO report_snapshot_control (workspace_id,report_issue_id)
SELECT $1,$2 WHERE NOT EXISTS (
 SELECT 1 FROM report_snapshot WHERE workspace_id=$1 AND report_issue_id=$2
)
ON CONFLICT (workspace_id,report_issue_id) DO UPDATE
SET revision=report_snapshot_control.revision;

-- name: LockReportSnapshotControl :one
SELECT * FROM report_snapshot_control
WHERE workspace_id=$1 AND report_issue_id=$2 FOR UPDATE;

-- name: GetReportSnapshotControl :one
SELECT * FROM report_snapshot_control
WHERE workspace_id=$1 AND report_issue_id=$2;

-- name: AdvanceReportSnapshotControl :one
UPDATE report_snapshot_control
SET current_snapshot_id=sqlc.arg('snapshot_id')::uuid,
 current_sequence=current_sequence+1,revision=revision+1
WHERE workspace_id=sqlc.arg('workspace_id')::uuid
 AND report_issue_id=sqlc.arg('report_issue_id')::uuid
 AND revision=sqlc.arg('expected_revision')::bigint
 AND withdrawn_at IS NULL
RETURNING *;

-- name: GetReportSnapshotWithdrawalByRequest :one
SELECT * FROM report_snapshot_control
WHERE workspace_id=$1 AND withdrawn_by=$2 AND withdrawal_request_key=$3;

-- name: WithdrawReportSnapshots :one
UPDATE report_snapshot_control SET
 revision=revision+1,
 withdrawal_id=sqlc.arg('withdrawal_id')::uuid,
 withdrawn_by=sqlc.arg('withdrawn_by')::uuid,
 withdrawal_request_key=sqlc.arg('request_key')::uuid,
 withdrawal_request_digest=sqlc.arg('request_digest')::text,
 withdrawal_expected_snapshot_id=sqlc.arg('expected_snapshot_id')::uuid,
 withdrawal_report_revision=sqlc.arg('report_revision')::bigint,
 withdrawal_reason=sqlc.arg('reason')::text,
 withdrawn_at=clock_timestamp()
WHERE workspace_id=sqlc.arg('workspace_id')::uuid
 AND report_issue_id=sqlc.arg('report_issue_id')::uuid
 AND revision=sqlc.arg('expected_revision')::bigint
 AND current_snapshot_id=sqlc.arg('expected_snapshot_id')::uuid
 AND withdrawn_at IS NULL
RETURNING *;
