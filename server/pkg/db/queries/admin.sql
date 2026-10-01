-- name: GetPlatformRole :one
SELECT role FROM platform_role_binding WHERE user_id = $1;

-- name: SetPlatformRole :exec
INSERT INTO platform_role_binding (user_id, role, granted_by, granted_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role, granted_by = EXCLUDED.granted_by, granted_at = now();

-- name: DeletePlatformRole :exec
DELETE FROM platform_role_binding WHERE user_id = $1;

-- name: LockPlatformAdminChanges :exec
SELECT pg_advisory_xact_lock(70856394211471801::bigint);

-- name: ListEffectiveSuperAdminCandidates :many
-- The service also filters the shared emergency denylist by identity.
SELECT u.id, u.email FROM platform_role_binding b
JOIN "user" u ON u.id = b.user_id
JOIN user_password_credential c ON c.user_id = b.user_id
WHERE b.role = 'super_admin' AND u.disabled_at IS NULL AND c.must_change_password = false AND c.session_version > 0;

-- name: BumpPlatformUserAuthVersion :one
UPDATE user_password_credential SET session_version = session_version + 1, updated_at = now()
WHERE user_id = $1 AND session_version = $2 AND session_version < 9223372036854775807
RETURNING *;

-- name: GetAdminOperationByKey :one
SELECT * FROM admin_operation
WHERE organization_id = $1 AND actor_id = $2 AND idempotency_key = $3;

-- name: GetAdminOperationForActor :one
SELECT * FROM admin_operation
WHERE organization_id = $1 AND actor_id = $2 AND id = $3;

-- name: CreateAdminOperation :one
INSERT INTO admin_operation (organization_id, actor_kind, actor_id, actor_auth_version,
 target_kind, target_id, kind, idempotency_key, payload_hash, reason,
 state, result_code, applied_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: CreateAdminAuditEvent :exec
INSERT INTO admin_audit_event (operation_id, organization_id, actor_kind, actor_user_id,
 target_kind, target_id, action, phase, request_id, reason, before_state, after_state, result_code)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: GetPlatformAdminAccount :one
-- One snapshot prevents a pre-grant credential version from being combined
-- with the newly granted role by separate authorization reads.
SELECT u.id, u.email, u.disabled_at, c.session_version, c.must_change_password, b.role
FROM "user" u JOIN user_password_credential c ON c.user_id = u.id
LEFT JOIN platform_role_binding b ON b.user_id = u.id
WHERE u.id = $1;
