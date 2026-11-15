-- name: CreateCollaborationTestSubject :exec
INSERT INTO collaboration_test_subject (issue_id, workspace_id, command_id, candidate_id, candidate_version,
 candidate_digest, subject, bound_by_user_id, participant_epoch, account_epoch, lifecycle_epoch)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);

-- name: ListCollaborationCommandTestSubjects :many
SELECT * FROM collaboration_test_subject
WHERE issue_id=$1 AND command_id=ANY($2::uuid[]) ORDER BY command_id;

-- name: GetCollaborationTestReportEvent :one
SELECT e.* FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id
JOIN collaboration_session s ON s.id=r.session_id
WHERE s.issue_id=$1 AND e.run_id=$2 AND e.id=$3 AND e.kind='test_report';

-- name: ListCollaborationTestReportEvents :many
SELECT e.* FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id
JOIN collaboration_session s ON s.id=r.session_id
WHERE s.issue_id=$1 AND e.run_id=$2 AND e.kind='test_report' AND e.seq<=$3 AND e.seq<$4
ORDER BY e.seq DESC LIMIT $5;

-- name: ListCollaborationCandidateTestReportEvents :many
SELECT e.* FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id
JOIN collaboration_session s ON s.id=r.session_id
WHERE s.issue_id=sqlc.arg(issue_id)::uuid AND e.kind='test_report' AND e.payload->'report'->'subject'->>'candidate_id'=sqlc.arg(candidate_id)::text
ORDER BY e.created_at DESC, e.id DESC LIMIT sqlc.arg(row_limit)::int;

-- name: GetCollaborationTestReportLogEvent :one
SELECT * FROM collaboration_run_event WHERE run_id=$1 AND id=$2 AND kind IN ('record_checkpoint','outcome');

-- name: GetCollaborationStageTestReportEvent :one
SELECT e.* FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id
JOIN collaboration_session s ON s.id=r.session_id
WHERE s.issue_id=$1 AND e.issue_id=s.issue_id AND e.id=$2 AND e.kind='test_report';

-- name: LockCollaborationStageTestRun :one
SELECT * FROM collaboration_run WHERE id=$1 FOR UPDATE NOWAIT;
