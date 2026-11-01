-- Private run disclosure has its own NOWAIT graph. These prelocks cover the
-- remaining blocking command/grant/group readers in currentAdmission; all later
-- resource identity locks already use the authority NOWAIT/try-lock queries.
-- name: LockCollaborationKnowledgeGate :one
SELECT * FROM collaboration_execution_gate WHERE id=1 FOR SHARE NOWAIT;

-- name: TryCollaborationKnowledgeStatusFence :one
SELECT pg_try_advisory_xact_lock_shared(hashtextextended($1::uuid::text || ':issue_status', 0))::boolean;

-- name: LockCollaborationKnowledgeCommands :many
SELECT * FROM collaboration_command WHERE session_id=$1 ORDER BY id FOR UPDATE NOWAIT;

-- name: LockCollaborationKnowledgeGrants :many
SELECT g.* FROM collaboration_grant g JOIN collaboration_target t ON t.id=g.target_id
WHERE t.session_id=$1 ORDER BY g.id FOR UPDATE OF g NOWAIT;

-- name: LockCollaborationKnowledgeGroupRuns :many
SELECT * FROM collaboration_run WHERE group_id=$1 ORDER BY id FOR UPDATE NOWAIT;
