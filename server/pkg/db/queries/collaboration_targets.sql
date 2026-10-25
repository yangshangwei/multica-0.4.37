-- Read projections use the caller's authorized repeatable-read transaction.
-- name: GetCollaborationSession :one
SELECT * FROM collaboration_session WHERE issue_id = $1;

-- name: ListCollaborationTargets :many
SELECT * FROM collaboration_target WHERE session_id = $1 ORDER BY id;

-- name: ListCollaborationExecutionGroups :many
SELECT * FROM collaboration_execution_group WHERE session_id = $1 ORDER BY target_id, generation;

-- name: ListCollaborationRuns :many
SELECT * FROM collaboration_run WHERE session_id = $1 ORDER BY created_at, id;

-- name: ListCollaborationCommands :many
SELECT * FROM collaboration_command WHERE session_id = $1 ORDER BY channel_id, seq;

-- The permanent source/session/channel locks serialize target membership and
-- controls. Target locks precede participant/grant/command/group/run locks.
-- name: LockCollaborationTargets :many
SELECT * FROM collaboration_target WHERE session_id = $1 ORDER BY id FOR UPDATE;

-- name: CreateCollaborationTarget :one
INSERT INTO collaboration_target (session_id, kind, resource_id, roster)
VALUES ($1, $2, $3, $4) RETURNING *;

-- A hold only gates this target. Consent advances its observation fence even
-- when an existing reason is retained; idempotent callers avoid this write.
-- name: SetCollaborationTargetHold :one
UPDATE collaboration_target
SET hold_reason = sqlc.narg(hold_reason)::text,
    hold_version = hold_version + CASE
        WHEN hold_reason IS DISTINCT FROM sqlc.narg(hold_reason)::text OR sqlc.arg(advance_version)::boolean THEN 1
        ELSE 0 END
WHERE session_id = sqlc.arg(session_id)::uuid AND id = sqlc.arg(target_id)::uuid
RETURNING *;

-- name: AdvanceCollaborationTargetSet :one
UPDATE collaboration_session SET target_set_version = target_set_version + 1
WHERE id = $1 RETURNING *;

-- name: PauseCollaborationTarget :one
UPDATE collaboration_target SET queue_state = 'paused', pause_reason = sqlc.arg(reason)::text,
    control_version = control_version + 1
WHERE session_id = sqlc.arg(session_id)::uuid AND id = sqlc.arg(target_id)::uuid
RETURNING *;

-- name: GetCollaborationCommandInput :one
SELECT * FROM collaboration_command WHERE target_id = $1 AND input_event_id = $2;

-- name: LockCollaborationTargetGrants :many
SELECT * FROM collaboration_grant WHERE target_id = $1 ORDER BY id FOR UPDATE;

-- This is durable input authorization, never a launch permit. Busy/offline and
-- installation admission pause do not revoke an otherwise valid resource grant.
-- Full namespace/supervisor/provider admission must still run before execution.
-- name: CollaborationGrantOwnerCurrent :one
SELECT EXISTS (
    SELECT 1 FROM collaboration_grant g
    JOIN issue_participant p ON p.issue_id = g.issue_id AND p.user_id = g.owner_user_id AND p.revoked_at IS NULL
    JOIN collaboration_account_fence f ON f.user_id = g.owner_user_id AND f.epoch = g.owner_auth_epoch
    JOIN "user" u ON u.id = g.owner_user_id AND u.disabled_at IS NULL
    WHERE g.id = $1 AND g.revoked_at IS NULL
)::boolean;

-- name: LockCollaborationUnsettledGroups :many
SELECT * FROM collaboration_execution_group WHERE session_id = $1 AND settled_at IS NULL
ORDER BY target_id, id FOR UPDATE;

-- name: GetCollaborationExecutionGroup :one
SELECT * FROM collaboration_execution_group WHERE session_id = $1 AND id = $2;

-- name: LockCollaborationGroupRuns :many
SELECT * FROM collaboration_run WHERE group_id = $1 ORDER BY id FOR UPDATE;

-- name: LockCollaborationGroupDelegations :many
SELECT * FROM collaboration_delegation WHERE group_id = $1 ORDER BY id FOR UPDATE;

-- Includes detached rows as barriers. Never derive
-- idleness exclusively from target.active_group_id or a group row.
-- name: LockCollaborationUnsettledRuns :many
SELECT * FROM collaboration_run WHERE session_id = $1 AND settled_at IS NULL
ORDER BY group_id, id FOR UPDATE;

-- name: SealCollaborationGroup :one
UPDATE collaboration_execution_group
SET expansion_sealed_at = COALESCE(expansion_sealed_at, now()), state = 'ending',
    disposition_reason = CASE WHEN forced_disposition IS NULL THEN sqlc.arg(reason)::text ELSE disposition_reason END,
    forced_disposition = COALESCE(forced_disposition, sqlc.arg(disposition)::text)
WHERE id = sqlc.arg(group_id)::uuid AND settled_at IS NULL
RETURNING *;

-- Cancels only uncompleted internal dispatch/continuation intents. Existing
-- runs still require independent proof; cancelling this row does not supply it.
-- name: CancelCollaborationGroupDelegations :exec
UPDATE collaboration_delegation SET state = 'cancelled', disposition_reason = sqlc.arg(reason)::text
WHERE group_id = sqlc.arg(group_id)::uuid AND state IN ('pending', 'waiting');

-- All run and delegation writers hold the group lock. The final root input is
-- never settled by a single parent's proof or an empty/corrupt execution ledger.
-- name: SettleStoppedCollaborationGroup :one
UPDATE collaboration_execution_group AS g
SET state = g.forced_disposition, settled_at = now()
WHERE g.id = sqlc.arg(group_id)::uuid AND g.settled_at IS NULL
  AND g.expansion_sealed_at IS NOT NULL AND g.forced_disposition IN ('stopped', 'cancelled')
  AND EXISTS (SELECT 1 FROM collaboration_run r WHERE r.group_id = g.id)
  AND NOT EXISTS (SELECT 1 FROM collaboration_run r WHERE r.group_id = g.id
      AND (NOT collaboration_managed_execution_ended(r.containment,r.proof_state,r.progress_state,r.settled_at,r.authorized_attempt_seq,r.sealed_attempt_seq,r.started_at,r.business_outcome)
           OR r.target_id <> g.target_id OR r.command_id <> g.command_id
           OR (r.parent_run_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM collaboration_run parent WHERE parent.id = r.parent_run_id AND parent.group_id = g.id))))
  AND NOT EXISTS (SELECT 1 FROM collaboration_delegation d WHERE d.group_id = g.id AND d.state NOT IN ('settled', 'cancelled'))
RETURNING *;

-- name: ClearCollaborationActiveGroup :one
UPDATE collaboration_target SET active_group_id = NULL
WHERE id = sqlc.arg(target_id)::uuid AND active_group_id = sqlc.arg(group_id)::uuid
RETURNING *;

-- name: RevokeCollaborationGrant :one
UPDATE collaboration_grant SET epoch = epoch + 1, revoked_at = now(), revocation_reason = sqlc.arg(reason)::text
WHERE id = sqlc.arg(grant_id)::uuid AND issue_id = sqlc.arg(issue_id)::uuid AND revoked_at IS NULL
RETURNING *;
