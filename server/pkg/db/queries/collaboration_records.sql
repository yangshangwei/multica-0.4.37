-- All reads borrow the current issue-authorized transaction. Run/event records
-- are source-owned; never join engine task/message or live resource tables.
-- name: GetCollaborationRecordedRun :one
SELECT r.* FROM collaboration_run r
JOIN collaboration_session s ON s.id=r.session_id
WHERE s.issue_id=sqlc.arg(issue_id)::uuid AND r.id=sqlc.arg(run_id)::uuid;

-- name: GetCollaborationRunRecordMetadata :many
SELECT * FROM collaboration_run_event
WHERE issue_id=$1 AND run_id=$2 AND kind IN ('resource_snapshot','outcome') ORDER BY seq;

-- name: GetCollaborationRunRecordSequence :one
SELECT COALESCE(max(seq),0)::bigint FROM collaboration_run_event WHERE issue_id=$1 AND run_id=$2;

-- Latest facts at the record cursor's watermark, not a scan returned to the
-- caller. A fifth/unknown stage is a corruption sentinel, never a public fact.
-- name: ListCollaborationRunPreparationRecords :many
SELECT DISTINCT ON (payload->>'stage') * FROM collaboration_run_event
WHERE issue_id=sqlc.arg(issue_id)::uuid AND run_id=sqlc.arg(run_id)::uuid
  AND kind='preparation' AND seq <= sqlc.arg(through_seq)::bigint
ORDER BY payload->>'stage', seq DESC LIMIT 5;

-- name: ListCollaborationRunRecordEvents :many
SELECT * FROM collaboration_run_event
WHERE issue_id=sqlc.arg(issue_id)::uuid AND run_id=sqlc.arg(run_id)::uuid
  AND seq >= sqlc.arg(after_seq)::bigint AND seq <= sqlc.arg(through_seq)::bigint
  -- Preparation has its own negotiated contract; installed record-item
  -- readers retain their existing discriminated kinds.
  AND kind NOT IN ('resource_snapshot', 'preparation')
ORDER BY seq LIMIT sqlc.arg(row_limit)::int;

-- name: MarkCollaborationRunRecordingIncomplete :exec
UPDATE collaboration_run SET recording_incomplete=true WHERE id=$1 AND settled_at IS NULL;

-- name: CreateCollaborationResultNotification :exec
INSERT INTO collaboration_notification(recipient_user_id,issue_id,event_seq,kind,source_kind,run_id)
VALUES($1,$2,$3,$4,'issue',$5)
ON CONFLICT (recipient_user_id,issue_id,event_seq) DO NOTHING;
