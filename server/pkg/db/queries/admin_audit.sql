-- name: ListAdminAuditEvents :many
-- actor_display_name is the immutable event snapshot; never join the current user.
SELECT id,operation_id,actor_kind,actor_user_id,actor_display_name,target_kind,target_id,action,phase,request_id,reason,before_state,after_state,result_code,created_at
FROM admin_audit_event
WHERE organization_id=sqlc.arg('organization_id')::uuid AND created_at>=sqlc.arg('time_from')::timestamptz AND created_at<sqlc.arg('time_to')::timestamptz AND created_at<=sqlc.arg('as_of')::timestamptz
AND (sqlc.arg('action')::text='' OR action=sqlc.arg('action'))
AND (sqlc.narg('actor_user_id')::uuid IS NULL OR actor_user_id=sqlc.narg('actor_user_id'))
AND (sqlc.arg('target_kind')::text='' OR target_kind=sqlc.arg('target_kind'))
AND (sqlc.narg('target_id')::uuid IS NULL OR target_id=sqlc.narg('target_id'))
AND (sqlc.arg('phase')::text='' OR phase=sqlc.arg('phase'))
AND (sqlc.narg('after_time')::timestamptz IS NULL OR (created_at,id)<(sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg('page_limit')::int;
