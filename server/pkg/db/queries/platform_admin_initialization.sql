-- name: GetPlatformAdminInitialization :one
SELECT * FROM platform_admin_initialization;

-- name: CreatePlatformAdminInitialization :one
INSERT INTO platform_admin_initialization
(organization_id, deployment_id, initialization_id, state, user_id, initial_credential_version)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: HasPlatformAdminHistory :one
SELECT EXISTS (SELECT 1 FROM platform_role_binding)
OR EXISTS (SELECT 1 FROM admin_operation WHERE kind = 'user.role.bootstrap' AND state = 'applied')
OR EXISTS (SELECT 1 FROM admin_audit_event WHERE target_kind = 'user' AND phase = 'applied'
    AND action IN ('user.role', 'user.role.bootstrap')
    AND (before_state->>'role' = 'super_admin' OR after_state->>'role' = 'super_admin'));
