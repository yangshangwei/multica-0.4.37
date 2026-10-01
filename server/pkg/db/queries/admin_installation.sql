-- name: ListAdminInstallations :many
SELECT i.id,i.deployment_id,i.organization_id,i.lifecycle,i.responsible_user_id,
 i.display_name,i.groups,i.desktop_version,i.os,i.client_seen_at,i.admission,i.admission_version,i.key_version,i.created_at,i.updated_at,
 (SELECT count(DISTINCT r.id) FROM installation_daemon_binding b JOIN agent_runtime r
  ON r.workspace_id=b.workspace_id AND r.daemon_id=b.daemon_id AND r.owner_id=b.principal_user_id
  JOIN organization_workspace ow ON ow.workspace_id=b.workspace_id AND ow.organization_id=i.organization_id
  WHERE b.installation_id=i.id AND b.state='active')::bigint AS runtime_count,
 (SELECT count(*) FROM installation_daemon_binding b WHERE b.installation_id=i.id AND b.state='active')::bigint AS binding_count
FROM managed_installation i
WHERE i.organization_id=sqlc.arg('organization_id')::uuid AND i.deployment_id=sqlc.arg('deployment_id')::uuid
AND i.created_at<=sqlc.arg('as_of')::timestamptz
AND i.created_at>=sqlc.arg('time_from')::timestamptz AND i.created_at<sqlc.arg('time_to')::timestamptz
AND (sqlc.narg('installation_id')::uuid IS NULL OR i.id=sqlc.narg('installation_id'))
AND (sqlc.arg('lifecycle')::text='' OR i.lifecycle=sqlc.arg('lifecycle'))
AND (sqlc.arg('version')::text='' OR i.desktop_version=sqlc.arg('version'))
AND (sqlc.arg('os')::text='' OR i.os=sqlc.arg('os'))
AND (sqlc.arg('group_name')::text='' OR sqlc.arg('group_name')=ANY(i.groups))
AND (sqlc.arg('search')::text='' OR i.id::text ILIKE '%'||sqlc.arg('search')||'%' OR i.display_name ILIKE '%'||sqlc.arg('search')||'%')
AND (sqlc.arg('client_state')::text='' OR CASE WHEN i.client_seen_at IS NULL THEN 'unknown'
 WHEN i.client_seen_at>sqlc.arg('observed_at')::timestamptz-interval '180 seconds' THEN 'active' ELSE 'inactive' END=sqlc.arg('client_state'))
AND (sqlc.narg('after_time')::timestamptz IS NULL OR (i.created_at,i.id)<(sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY i.created_at DESC,i.id DESC LIMIT sqlc.arg('page_limit')::int;

-- name: ListAdminInstallationEvidence :many
-- Exact verified binding identity only. Never infer associations from names,
-- device_info, paths, IPs or the legacy daily-usage installation identifier.
SELECT b.installation_id,b.id AS binding_id,b.workspace_id,b.principal_user_id,b.capability_version,b.last_seen_at AS binding_seen_at,
 r.id AS runtime_id,coalesce(r.status,'')::text AS runtime_status,coalesce(r.provider,'')::text AS provider,r.last_seen_at,
 CASE WHEN r.metadata->'offline_reason'->>'code'='not_executable' THEN 'not_executable' ELSE '' END::text AS offline_code,
 coalesce(u.email,'')::text AS principal_email,
 coalesce((i.lifecycle='active' AND u.id IS NOT NULL AND u.disabled_at IS NULL AND c.session_version=b.auth_version AND NOT c.must_change_password
 AND EXISTS(SELECT 1 FROM member m WHERE m.workspace_id=b.workspace_id AND m.user_id=b.principal_user_id)
 AND EXISTS(SELECT 1 FROM daemon_token dt WHERE dt.installation_binding_id=b.id AND dt.installation_binding_epoch=b.binding_epoch
  AND dt.user_id=b.principal_user_id AND dt.auth_version=b.auth_version AND dt.workspace_id=b.workspace_id AND dt.daemon_id=b.daemon_id AND dt.expires_at>now())),false)::boolean AS authorized,
 (SELECT count(*) FROM agent_task_queue t WHERE t.runtime_id=r.id AND t.status IN ('preparing','dispatched','running','waiting_local_directory'))::bigint AS running_tasks
FROM managed_installation i JOIN installation_daemon_binding b ON b.installation_id=i.id AND b.state='active'
JOIN organization_workspace ow ON ow.workspace_id=b.workspace_id AND ow.organization_id=i.organization_id
LEFT JOIN agent_runtime r ON r.workspace_id=b.workspace_id AND r.daemon_id=b.daemon_id AND r.owner_id=b.principal_user_id
LEFT JOIN "user" u ON u.id=b.principal_user_id
LEFT JOIN user_password_credential c ON c.user_id=b.principal_user_id
WHERE i.organization_id=sqlc.arg('organization_id')::uuid AND i.deployment_id=sqlc.arg('deployment_id')::uuid
AND i.id=ANY(sqlc.arg('installation_ids')::uuid[])
ORDER BY i.id,b.id,r.id LIMIT 2001;

-- name: ListAdminInstallationBindings :many
SELECT b.id,b.workspace_id,b.principal_user_id,b.binding_epoch,b.state,b.capability_version,b.authenticated_at,b.last_seen_at,b.revoked_at
FROM installation_daemon_binding b JOIN managed_installation i ON i.id=b.installation_id
WHERE i.id=$1 AND i.organization_id=$2 AND i.deployment_id=$3
ORDER BY b.created_at DESC,b.id DESC LIMIT 101;

-- name: ListAdminInstallationUsers :many
SELECT iu.user_id,iu.first_seen_at,iu.last_seen_at
FROM installation_user iu JOIN managed_installation i ON i.id=iu.installation_id
WHERE i.id=$1 AND i.organization_id=$2 AND i.deployment_id=$3
ORDER BY iu.last_seen_at DESC,iu.user_id DESC LIMIT 101;

-- name: ListAdminUnassociatedRuntimes :many
SELECT r.id,r.workspace_id,r.owner_id,r.provider,r.status,r.last_seen_at,r.created_at
FROM agent_runtime r JOIN organization_workspace ow ON ow.workspace_id=r.workspace_id AND ow.organization_id=sqlc.arg('organization_id')::uuid
WHERE r.created_at<=sqlc.arg('as_of')::timestamptz
AND r.created_at>=sqlc.arg('time_from')::timestamptz AND r.created_at<sqlc.arg('time_to')::timestamptz
AND (sqlc.narg('workspace_id')::uuid IS NULL OR r.workspace_id=sqlc.narg('workspace_id'))
AND NOT EXISTS(SELECT 1 FROM installation_daemon_binding b JOIN managed_installation i ON i.id=b.installation_id
 WHERE b.workspace_id=r.workspace_id AND b.daemon_id=r.daemon_id AND b.principal_user_id=r.owner_id AND b.state='active'
 AND i.organization_id=ow.organization_id AND i.deployment_id=sqlc.arg('deployment_id')::uuid AND i.lifecycle='active')
AND (sqlc.narg('after_time')::timestamptz IS NULL OR (r.created_at,r.id)<(sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY r.created_at DESC,r.id DESC LIMIT sqlc.arg('page_limit')::int;
