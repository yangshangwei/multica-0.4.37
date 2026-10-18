-- Reads borrow the caller's authorized snapshot. Writes hold issue/session/
-- sorted channels/targets first and take all later resource locks NOWAIT.
-- name: ListCollaborationTargetGrants :many
SELECT * FROM collaboration_grant WHERE target_id = $1 ORDER BY id;

-- name: GetCollaborationRuntimeEnrollment :one
SELECT * FROM collaboration_runtime_enrollment WHERE id = $1;

-- name: CreateConsentedCollaborationTarget :one
INSERT INTO collaboration_target (session_id, kind, resource_id, roster, configuration_fingerprint)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ReconfigureCollaborationTarget :one
UPDATE collaboration_target SET roster = sqlc.arg(roster)::jsonb,
    configuration_fingerprint = sqlc.arg(configuration_fingerprint)::text,
    config_version = config_version + 1, target_epoch = target_epoch + 1,
    hold_reason = 'switch_hold', hold_version = hold_version + 1, revoked_at = NULL
WHERE id = sqlc.arg(target_id)::uuid AND session_id = sqlc.arg(session_id)::uuid
RETURNING *;

-- name: CreateCollaborationResourceConsent :one
INSERT INTO collaboration_grant (issue_id, target_id, config_version, role, owner_user_id,
    owner_auth_epoch, owner_participant_epoch, target_epoch, configuration_fingerprint,
    execution_workspace_id, agent_id, runtime_id, identity_kind, installation_id, binding_id, binding_epoch)
VALUES (sqlc.arg(issue_id)::uuid, sqlc.arg(target_id)::uuid, sqlc.arg(config_version)::bigint,
    sqlc.arg(role)::text, sqlc.arg(owner_user_id)::uuid, sqlc.arg(owner_auth_epoch)::bigint,
    sqlc.arg(owner_participant_epoch)::bigint, sqlc.arg(target_epoch)::bigint, sqlc.arg(configuration_fingerprint)::text,
    sqlc.arg(execution_workspace_id)::uuid, sqlc.arg(agent_id)::uuid, sqlc.arg(runtime_id)::uuid,
    sqlc.arg(identity_kind)::text, sqlc.arg(installation_id)::uuid, sqlc.arg(binding_id)::uuid, sqlc.arg(binding_epoch)::bigint)
RETURNING *;

-- name: RevokeCollaborationTargetConsents :exec
UPDATE collaboration_grant SET revoked_at = now(), revocation_reason = sqlc.arg(reason)::text, epoch = epoch + 1
WHERE target_id = sqlc.arg(target_id)::uuid AND revoked_at IS NULL;

-- Lock retained inputs before any group/run writer, including detached and
-- incomplete work. Stop must not take a group then wait for its input's row lock.
-- name: LockCollaborationSessionCommands :many
SELECT * FROM collaboration_command WHERE session_id = $1 ORDER BY id FOR UPDATE;

-- name: TryLockCollaborationAuthorityWorkspaces :many
SELECT id FROM workspace WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR KEY SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityMembers :many
SELECT * FROM member WHERE workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[])
    AND user_id = ANY(sqlc.arg(user_ids)::uuid[]) ORDER BY workspace_id, user_id FOR KEY SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityAgents :many
SELECT * FROM agent WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityRuntimes :many
SELECT * FROM agent_runtime WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR SHARE NOWAIT;

-- name: TryLockCollaborationAuthoritySquad :one
SELECT * FROM squad WHERE id = $1 FOR SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityInvocationTargets :many
SELECT * FROM agent_invocation_target WHERE agent_id = ANY(sqlc.arg(agent_ids)::uuid[])
ORDER BY agent_id, id FOR SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityNamespace :one
SELECT pg_try_advisory_xact_lock(hashtextextended($1::text, 81271465))::boolean AS acquired;

-- name: TryLockCollaborationAuthorityBinding :one
SELECT * FROM installation_daemon_binding WHERE id = $1 FOR SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityInstallation :one
SELECT * FROM managed_installation WHERE id = $1 FOR SHARE NOWAIT;

-- name: TryLockCollaborationAuthorityEnrollment :one
SELECT * FROM collaboration_runtime_enrollment WHERE id = $1 FOR SHARE NOWAIT;
