-- Resource owners already hold the runtime or installation namespace fence.
-- Retained grants, frozen rosters and runs keep sources discoverable after
-- engine cleanup. Current pending targets also need a durable hold on deletion.
-- name: ListCollaborationResourceIssues :many
SELECT affected.issue_id, i.workspace_id FROM (
    SELECT g.issue_id FROM collaboration_grant g
    WHERE g.runtime_id = sqlc.narg(runtime_id)::uuid
       OR (g.binding_id = sqlc.narg(binding_id)::uuid AND g.binding_epoch = sqlc.arg(binding_epoch)::bigint)
    UNION
    SELECT s.issue_id FROM collaboration_target t
    JOIN collaboration_session s ON s.id = t.session_id
    WHERE t.revoked_at IS NULL AND sqlc.narg(runtime_id)::uuid IS NOT NULL
      AND EXISTS (SELECT 1 FROM jsonb_array_elements(t.roster) resource
                  WHERE resource->>'runtime_id' = sqlc.narg(runtime_id)::uuid::text)
    UNION
    SELECT s.issue_id FROM collaboration_execution_group g
    JOIN collaboration_session s ON s.id = g.session_id
    WHERE g.settled_at IS NULL AND sqlc.narg(runtime_id)::uuid IS NOT NULL
      AND EXISTS (SELECT 1 FROM jsonb_array_elements(g.roster) resource
                  WHERE resource->>'runtime_id' = sqlc.narg(runtime_id)::uuid::text)
    UNION
    SELECT s.issue_id FROM collaboration_run r
    JOIN collaboration_session s ON s.id = r.session_id
    WHERE r.settled_at IS NULL AND (r.runtime_id = sqlc.narg(runtime_id)::uuid
       OR (r.binding_id = sqlc.narg(binding_id)::uuid AND r.binding_epoch = sqlc.arg(binding_epoch)::bigint))
) affected
LEFT JOIN issue i ON i.id = affected.issue_id
ORDER BY affected.issue_id;

-- Every source-side row lock is NOWAIT while the caller owns resources. The
-- shared stop helpers can safely reacquire these locks in the same transaction.
-- name: TryLockCollaborationResourceChannels :many
SELECT * FROM collaboration_channel WHERE session_id = $1 ORDER BY id FOR UPDATE NOWAIT;

-- name: TryLockCollaborationResourceTargets :many
SELECT * FROM collaboration_target WHERE session_id = $1 ORDER BY id FOR UPDATE NOWAIT;

-- name: TryLockCollaborationResourceParticipants :many
SELECT user_id FROM issue_participant WHERE issue_id = $1 ORDER BY user_id FOR UPDATE NOWAIT;

-- name: TryLockCollaborationResourceGrants :many
SELECT * FROM collaboration_grant WHERE issue_id = $1 ORDER BY id FOR UPDATE NOWAIT;

-- name: TryLockCollaborationResourceCommands :many
SELECT id FROM collaboration_command WHERE session_id = $1 ORDER BY id FOR UPDATE NOWAIT;

-- name: TryLockCollaborationResourceBatches :many
SELECT id FROM collaboration_batch WHERE session_id = $1 AND state IN ('waiting', 'in_flight') ORDER BY id FOR UPDATE NOWAIT;

-- name: TryLockCollaborationResourceGroups :many
SELECT * FROM collaboration_execution_group WHERE session_id = $1 AND settled_at IS NULL ORDER BY id FOR UPDATE NOWAIT;

-- The settled siblings are also read by validateGroup; an orphan unsettled run
-- must remain visible so a broken ledger refuses teardown instead of vanishing.
-- name: TryLockCollaborationResourceRuns :many
SELECT r.* FROM collaboration_run r WHERE r.session_id = $1
  AND (r.settled_at IS NULL OR EXISTS (SELECT 1 FROM collaboration_execution_group g WHERE g.id = r.group_id AND g.settled_at IS NULL))
ORDER BY r.id FOR UPDATE OF r NOWAIT;

-- name: TryLockCollaborationResourceDelegations :many
SELECT d.id FROM collaboration_delegation d
JOIN collaboration_execution_group g ON g.id = d.group_id
WHERE g.session_id = $1 AND g.settled_at IS NULL ORDER BY d.id FOR UPDATE OF d NOWAIT;

-- name: TryLockCollaborationResourceTerminations :many
SELECT proof.run_id FROM collaboration_termination proof
JOIN collaboration_run r ON r.id = proof.run_id
WHERE r.session_id = $1 ORDER BY proof.run_id FOR UPDATE OF proof NOWAIT;

-- name: TryLockCollaborationResourceCapabilities :many
SELECT capability.run_id FROM collaboration_business_capability capability
JOIN collaboration_run r ON r.id = capability.run_id
WHERE r.session_id = $1 ORDER BY capability.run_id, capability.attempt_seq FOR UPDATE OF capability NOWAIT;

-- name: TryLockCollaborationResourceReservations :many
SELECT reserve.run_id FROM collaboration_runtime_reservation reserve
JOIN collaboration_run r ON r.id = reserve.run_id
WHERE r.session_id = $1 ORDER BY reserve.run_id FOR UPDATE OF reserve NOWAIT;

-- Retention GC must not infer process exit from ordinary task projections.
-- Orphan reservations remain a refusal; a missing run is never proof.
-- name: HasUnprovenCollaborationRuntimeWork :one
SELECT (EXISTS (
    SELECT 1 FROM collaboration_run r WHERE r.runtime_id = $1 AND r.settled_at IS NULL
) OR EXISTS (
    SELECT 1 FROM collaboration_runtime_reservation reserve WHERE reserve.runtime_id = $1 AND reserve.released_at IS NULL
))::boolean;
