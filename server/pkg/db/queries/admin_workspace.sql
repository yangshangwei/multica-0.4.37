-- name: ListAdminWorkspaces :many
SELECT w.id,w.name,w.slug,w.created_at,
 (SELECT count(*) FROM member m WHERE m.workspace_id=w.id)::bigint AS member_count,
 (SELECT count(*) FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id=w.id
 AND t.created_at>=sqlc.arg('time_from')::timestamptz AND t.created_at<sqlc.arg('time_to')::timestamptz AND t.created_at<=sqlc.arg('as_of')::timestamptz)::bigint AS execution_count
FROM workspace w JOIN organization_workspace ow ON ow.workspace_id=w.id
WHERE ow.organization_id=sqlc.arg('organization_id')::uuid AND w.created_at<=sqlc.arg('as_of')::timestamptz
AND (sqlc.arg('search')::text='' OR w.name ILIKE '%'||sqlc.arg('search')||'%' OR w.slug ILIKE '%'||sqlc.arg('search')||'%' OR w.id::text=sqlc.arg('search'))
AND (sqlc.narg('after_time')::timestamptz IS NULL OR (w.created_at,w.id)<(sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY w.created_at DESC,w.id DESC LIMIT sqlc.arg('page_limit')::int;
