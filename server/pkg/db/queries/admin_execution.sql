-- name: ListAdminTasks :many
-- Page metadata before aggregating usage. No raw result, error, prompt, local
-- path, message, or agent configuration may enter this projection.
WITH candidates AS (
 SELECT t.id, t.agent_id, a.workspace_id, t.issue_id, t.runtime_id,
 t.chat_session_id, t.autopilot_run_id, t.status, t.attempt, t.parent_task_id,
 t.retry_of_task_id, t.rerun_of_task_id, t.accountable_user_id,
 t.submitted_installation_id, t.execution_installation_id,
 t.created_at, t.dispatched_at, t.started_at, t.completed_at, t.failure_reason,
 CASE WHEN t.chat_session_id IS NOT NULL THEN 'chat'
      WHEN t.issue_id IS NOT NULL AND (t.autopilot_run_id IS NOT NULL OR i.origin_type = 'autopilot') THEN 'autopilot_issue'
      WHEN t.issue_id IS NOT NULL THEN 'issue'
      WHEN t.autopilot_run_id IS NOT NULL THEN 'autopilot'
      WHEN t.context->>'type' = 'quick_create' THEN 'quick_create'
      ELSE 'unknown' END::text AS source,
 CASE WHEN m.user_id IS NOT NULL THEN coalesce(i.title,'') ELSE '' END::text AS title,
 CASE WHEN m.user_id IS NOT NULL AND i.id IS NOT NULL THEN '/' || w.slug || '/issues/' || i.id::text
      -- Mirror gatePublicChatSessionForUser: creator, current private-agent
      -- access, and a member-visible conversation (never command-only).
      WHEN m.user_id IS NOT NULL AND chat_agent.id IS NOT NULL AND cs.creator_id = sqlc.arg('actor_id')::uuid
       AND (cs.explicitly_created_at IS NOT NULL OR EXISTS (
         SELECT 1 FROM chat_message public_message WHERE public_message.chat_session_id=cs.id AND public_message.message_kind!='channel_command'))
       AND (chat_agent.owner_id=sqlc.arg('actor_id')::uuid OR m.role IN ('owner','admin')
         OR (chat_agent.permission_mode='public_to' AND EXISTS (
           SELECT 1 FROM agent_invocation_target target WHERE target.agent_id=chat_agent.id
            AND (target.target_type='workspace' OR (target.target_type='member' AND target.target_id=sqlc.arg('actor_id')::uuid)))))
       THEN '/' || w.slug || '/chat?session=' || cs.id::text
      ELSE '' END::text AS content_url,
 CASE WHEN rt.workspace_id = a.workspace_id THEN rt.provider ELSE '' END::text AS provider
 FROM agent_task_queue t
 JOIN agent a ON a.id = t.agent_id
 JOIN organization_workspace ow ON ow.workspace_id = a.workspace_id AND ow.organization_id = sqlc.arg('organization_id')::uuid
 JOIN workspace w ON w.id = a.workspace_id
 LEFT JOIN issue i ON i.id = t.issue_id AND i.workspace_id = a.workspace_id
 LEFT JOIN chat_session cs ON cs.id = t.chat_session_id AND cs.workspace_id = a.workspace_id
 LEFT JOIN agent chat_agent ON chat_agent.id=cs.agent_id AND chat_agent.workspace_id=a.workspace_id
 LEFT JOIN member m ON m.workspace_id = a.workspace_id AND m.user_id = sqlc.arg('actor_id')::uuid
 LEFT JOIN agent_runtime rt ON rt.id = t.runtime_id
 WHERE t.created_at >= sqlc.arg('time_from')::timestamptz
 AND t.created_at < sqlc.arg('time_to')::timestamptz AND t.created_at <= sqlc.arg('as_of')::timestamptz
 AND (sqlc.narg('workspace_id')::uuid IS NULL OR a.workspace_id = sqlc.narg('workspace_id'))
 AND (sqlc.narg('task_id')::uuid IS NULL OR t.id = sqlc.narg('task_id'))
 AND (sqlc.narg('issue_id')::uuid IS NULL OR t.issue_id = sqlc.narg('issue_id'))
 AND (sqlc.narg('runtime_id')::uuid IS NULL OR t.runtime_id = sqlc.narg('runtime_id'))
 AND (sqlc.narg('user_id')::uuid IS NULL OR t.accountable_user_id = sqlc.narg('user_id'))
 AND (sqlc.narg('installation_id')::uuid IS NULL OR t.execution_installation_id = sqlc.narg('installation_id'))
 AND (sqlc.arg('status')::text = '' OR t.status = sqlc.arg('status'))
 AND (sqlc.arg('search')::text = '' OR t.id::text ILIKE '%' || sqlc.arg('search') || '%' OR w.issue_prefix || '-' || i.number::text ILIKE '%' || sqlc.arg('search') || '%')
 AND (sqlc.narg('after_time')::timestamptz IS NULL OR (t.created_at,t.id) < (sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
), page AS (
 SELECT * FROM candidates WHERE sqlc.arg('source')::text = '' OR source = sqlc.arg('source')
 ORDER BY created_at DESC,id DESC LIMIT sqlc.arg('page_limit')::int
)
SELECT page.*, usage.input_tokens, usage.output_tokens, usage.cache_read_tokens, usage.cache_write_tokens,
 usage.reported_models, usage.model
FROM page LEFT JOIN LATERAL (
 SELECT coalesce(sum(input_tokens),0)::bigint AS input_tokens, coalesce(sum(output_tokens),0)::bigint AS output_tokens,
 coalesce(sum(cache_read_tokens),0)::bigint AS cache_read_tokens, coalesce(sum(cache_write_tokens),0)::bigint AS cache_write_tokens,
 count(*)::bigint AS reported_models,
 CASE WHEN count(DISTINCT model)=1 THEN min(model) ELSE '' END::text AS model
 FROM task_usage WHERE task_id=page.id
) usage ON true
ORDER BY page.created_at DESC,page.id DESC;

-- name: ListAdminIssues :many
-- Issue count and execution count are different entities. The correlated
-- count uses the existing issue_id task index and never fetches task content.
SELECT i.id, i.workspace_id, i.number, w.issue_prefix, i.status, i.created_at, i.updated_at,
 CASE WHEN m.user_id IS NOT NULL THEN i.title ELSE '' END::text AS title,
 CASE WHEN m.user_id IS NOT NULL THEN '/' || w.slug || '/issues/' || i.id::text ELSE '' END::text AS content_url,
 (SELECT count(*) FROM agent_task_queue t WHERE t.issue_id=i.id
  AND t.created_at>=sqlc.arg('time_from')::timestamptz AND t.created_at<sqlc.arg('time_to')::timestamptz
  AND t.created_at<=sqlc.arg('as_of')::timestamptz)::bigint AS execution_count
FROM issue i
JOIN organization_workspace ow ON ow.workspace_id=i.workspace_id AND ow.organization_id=sqlc.arg('organization_id')::uuid
JOIN workspace w ON w.id=i.workspace_id
LEFT JOIN member m ON m.workspace_id=i.workspace_id AND m.user_id=sqlc.arg('actor_id')::uuid
WHERE i.created_at>=sqlc.arg('time_from')::timestamptz AND i.created_at<sqlc.arg('time_to')::timestamptz AND i.created_at<=sqlc.arg('as_of')::timestamptz
AND (sqlc.narg('workspace_id')::uuid IS NULL OR i.workspace_id=sqlc.narg('workspace_id'))
AND (sqlc.narg('issue_id')::uuid IS NULL OR i.id=sqlc.narg('issue_id'))
AND (sqlc.arg('status')::text='' OR i.status=sqlc.arg('status'))
AND (sqlc.arg('search')::text='' OR i.id::text ILIKE '%'||sqlc.arg('search')||'%' OR w.issue_prefix||'-'||i.number::text ILIKE '%'||sqlc.arg('search')||'%')
AND (sqlc.narg('after_time')::timestamptz IS NULL OR (i.created_at,i.id)<(sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY i.created_at DESC,i.id DESC LIMIT sqlc.arg('page_limit')::int;
