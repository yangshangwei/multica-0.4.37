-- name: SetPlatformUserDisabled :exec
UPDATE "user" SET disabled_at = sqlc.narg(disabled_at), disabled_reason = sqlc.narg(disabled_reason),
 legacy_password_sessions_revoked_at = CASE WHEN sqlc.narg(disabled_at)::timestamptz IS NOT NULL THEN COALESCE(legacy_password_sessions_revoked_at, now()) ELSE legacy_password_sessions_revoked_at END
WHERE id = sqlc.arg(user_id);

-- name: ListPlatformUsers :many
SELECT u.id, u.name, u.email, u.created_at, u.disabled_at,
 c.username, c.session_version, c.must_change_password, b.role,
 (SELECT count(*) FROM member m JOIN organization_workspace ow ON ow.workspace_id = m.workspace_id
  WHERE m.user_id = u.id AND ow.organization_id = sqlc.arg(organization_id)) AS workspace_count
FROM "user" u LEFT JOIN user_password_credential c ON c.user_id = u.id
LEFT JOIN platform_role_binding b ON b.user_id = u.id
WHERE u.created_at <= sqlc.arg(as_of)
 AND (sqlc.narg(created_from)::timestamptz IS NULL OR u.created_at >= sqlc.narg(created_from))
 AND (sqlc.narg(created_to)::timestamptz IS NULL OR u.created_at < sqlc.narg(created_to))
 AND (sqlc.arg(search)::text = '' OR u.name ILIKE '%' || sqlc.arg(search) || '%' OR c.username ILIKE '%' || sqlc.arg(search) || '%')
 AND (sqlc.arg(role_filter)::text = '' OR (sqlc.arg(role_filter) = 'administrators' AND b.role IS NOT NULL) OR b.role = sqlc.arg(role_filter))
 AND (sqlc.narg(before_created)::timestamptz IS NULL OR (u.created_at,u.id) < (sqlc.narg(before_created),sqlc.narg(before_id)::uuid))
ORDER BY u.created_at DESC, u.id DESC LIMIT sqlc.arg(row_limit);

-- name: GetPlatformUserDetail :one
SELECT u.id, u.name, u.email, u.created_at, u.disabled_at,
 c.username, c.session_version, c.must_change_password, b.role,
 (SELECT count(*) FROM member m JOIN organization_workspace ow ON ow.workspace_id=m.workspace_id
 WHERE m.user_id=u.id AND ow.organization_id=sqlc.arg(organization_id)) AS workspace_count
FROM "user" u LEFT JOIN user_password_credential c ON c.user_id=u.id
LEFT JOIN platform_role_binding b ON b.user_id=u.id WHERE u.id=sqlc.arg(user_id);

-- name: ListPlatformUserMemberships :many
SELECT w.id AS workspace_id, w.name AS workspace_name, m.role
FROM member m JOIN workspace w ON w.id=m.workspace_id
JOIN organization_workspace ow ON ow.workspace_id=w.id
WHERE m.user_id=$1 AND ow.organization_id=$2 ORDER BY w.id LIMIT 100;

-- name: RevokePlatformLegacySessions :exec
UPDATE "user" SET legacy_password_sessions_revoked_at = COALESCE(legacy_password_sessions_revoked_at,now()) WHERE id=$1;
