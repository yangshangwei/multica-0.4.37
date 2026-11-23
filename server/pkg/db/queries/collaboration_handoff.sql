-- All projections are called inside the current issue/run authorization transaction.
-- name: GetCollaborationHandoffByID :one
SELECT * FROM issue_handoff WHERE issue_id=$1 AND id=$2;

-- name: GetCollaborationLatestHandoff :one
SELECT h.*, e.event_seq FROM collaboration_event e
JOIN issue_handoff h ON h.issue_id=e.issue_id AND h.id::text=e.payload->>'handoff_id'
WHERE e.issue_id=$1 ORDER BY e.event_seq DESC LIMIT 1;

-- name: ListCollaborationHandoffRuns :many
SELECT * FROM collaboration_run WHERE session_id=sqlc.arg(session_id)::uuid
 AND (sqlc.narg(before_at)::timestamptz IS NULL OR (created_at,id)<(sqlc.narg(before_at)::timestamptz,sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(row_limit)::int;

-- name: CountCollaborationHandoffRuns :one
SELECT count(*)::bigint FROM collaboration_run WHERE session_id=$1;

-- name: ListCollaborationHandoffRunResults :many
SELECT * FROM collaboration_run_event WHERE issue_id=$1 AND run_id=ANY($2::uuid[]) AND kind IN ('outcome','squad_activity') ORDER BY seq;

-- name: GetCollaborationHandoffSourceEvent :one
SELECT * FROM collaboration_run_event WHERE issue_id=$1 AND id=$2 AND kind IN ('comment','outcome','squad_activity');

-- name: ListCollaborationHandoffCauses :many
SELECT * FROM collaboration_run_event WHERE issue_id=$1 AND run_id=$2 AND kind='wakeup_assigned' AND seq>$3 ORDER BY seq LIMIT $4;

-- name: CountCollaborationHandoffCauses :one
SELECT count(*)::bigint FROM collaboration_run_event WHERE issue_id=$1 AND run_id=$2 AND kind='wakeup_assigned';

-- name: ListCollaborationPendingWakeCauses :many
SELECT e.* FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id
WHERE r.group_id=$1 AND e.kind='wakeup_pending' AND NOT EXISTS (
 SELECT 1 FROM collaboration_run_event a JOIN collaboration_run ar ON ar.id=a.run_id
 WHERE ar.group_id=$1 AND a.kind='wakeup_assigned' AND a.payload->>'pending_id'=e.id::text
) ORDER BY e.created_at,e.id LIMIT 256;

-- name: CountCollaborationHandoffSnapshots :one
SELECT count(*)::bigint FROM collaboration_run_event WHERE run_id=$1 AND kind='handoff_snapshot';

-- name: ListCollaborationHandoffReaderRuns :many
SELECT * FROM collaboration_run WHERE group_id=sqlc.arg(group_id)::uuid
 AND (sqlc.narg(after_id)::uuid IS NULL OR id>sqlc.narg(after_id)::uuid)
ORDER BY id LIMIT sqlc.arg(row_limit)::int;

-- name: GetCollaborationHandoffDelegation :one
SELECT * FROM collaboration_delegation WHERE group_id=$1 AND run_id=$2;

-- name: GetCollaborationHandoffCommand :one
SELECT * FROM collaboration_command WHERE session_id=$1 AND id=$2;

-- Bind an active request to its own original workflow event, never the latest
-- handoff (which may be a nested return). Missing legacy links remain unknown.
-- name: GetCollaborationInformationRequestEventSequence :one
SELECT event_seq FROM collaboration_event
WHERE issue_id=sqlc.arg(issue_id)::uuid AND kind='information_requested'
  AND payload->>'handoff_id'=sqlc.arg(request_id)::uuid::text
ORDER BY event_seq LIMIT 1;
