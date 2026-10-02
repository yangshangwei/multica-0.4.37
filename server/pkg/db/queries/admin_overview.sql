-- name: GetAdminOverviewExecutions :one
-- Finished-window metrics never use created_at. Missing/negative clocks are
-- excluded from percentile samples; observations are lower bounds, not exact.
-- Let each consumer project only its needed columns and plan its joins directly.
WITH finished AS NOT MATERIALIZED (
 SELECT t.id,t.status,t.queued_at,t.queued_at_source,t.dispatched_at,t.started_at,t.completed_at
 FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id
 JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
 WHERE ow.organization_id=sqlc.arg('organization_id')::uuid
 AND t.completed_at>=sqlc.arg('time_from')::timestamptz AND t.completed_at<sqlc.arg('time_to')::timestamptz
 AND t.completed_at<=sqlc.arg('as_of')::timestamptz
 AND (sqlc.narg('workspace_id')::uuid IS NULL OR a.workspace_id=sqlc.narg('workspace_id'))
), outcome AS (
 SELECT count(*) FILTER(WHERE status='completed')::bigint AS completed,
 count(*) FILTER(WHERE status='failed')::bigint AS failed,
 count(*) FILTER(WHERE status='cancelled')::bigint AS cancelled,
 count(*) FILTER(WHERE status NOT IN ('completed','failed','cancelled'))::bigint AS unknown_status,
 count(*) FILTER(WHERE queued_at_source='transition' AND dispatched_at>=queued_at)::bigint AS queue_samples,
 count(*) FILTER(WHERE queued_at_source='observation' AND dispatched_at>=queued_at)::bigint AS queue_lower_bound_samples,
 count(*) FILTER(WHERE started_at IS NOT NULL AND completed_at>=started_at)::bigint AS run_samples,
 coalesce(percentile_cont(0.5) WITHIN GROUP(ORDER BY extract(epoch FROM dispatched_at-queued_at)) FILTER(WHERE queued_at_source='transition' AND dispatched_at>=queued_at),-1)::double precision AS queue_p50,
 coalesce(percentile_cont(0.95) WITHIN GROUP(ORDER BY extract(epoch FROM dispatched_at-queued_at)) FILTER(WHERE queued_at_source='transition' AND dispatched_at>=queued_at),-1)::double precision AS queue_p95,
 coalesce(percentile_cont(0.5) WITHIN GROUP(ORDER BY extract(epoch FROM completed_at-started_at)) FILTER(WHERE started_at IS NOT NULL AND completed_at>=started_at),-1)::double precision AS run_p50,
 coalesce(percentile_cont(0.95) WITHIN GROUP(ORDER BY extract(epoch FROM completed_at-started_at)) FILTER(WHERE started_at IS NOT NULL AND completed_at>=started_at),-1)::double precision AS run_p95,
 count(*)::bigint AS finished_count FROM finished
), usage AS (
 -- Count tasks independently from reports: a task may report several models,
 -- and any unpriced report makes that task's cost unknown.
 SELECT count(DISTINCT u.task_id)::bigint AS reported_tasks,
 count(*) FILTER(WHERE u.task_id IS NULL)::bigint AS missing_tasks,
 count(DISTINCT u.task_id) FILTER(WHERE u.cost_usd_ticks IS NULL)::bigint AS unpriced_tasks,
 coalesce(sum(u.input_tokens),0)::bigint AS input_tokens,coalesce(sum(u.output_tokens),0)::bigint AS output_tokens,
 coalesce(sum(u.cache_read_tokens),0)::bigint AS cache_read_tokens,coalesce(sum(u.cache_write_tokens),0)::bigint AS cache_write_tokens
 FROM finished f LEFT JOIN task_usage u ON u.task_id=f.id
), current_states AS (
 SELECT count(*) FILTER(WHERE t.status NOT IN ('completed','failed','cancelled'))::bigint AS unfinished,
 count(*) FILTER(WHERE t.status='queued')::bigint AS queued,count(*) FILTER(WHERE t.status='dispatched')::bigint AS dispatched,
 count(*) FILTER(WHERE t.status='running')::bigint AS running,count(*) FILTER(WHERE t.status='waiting_local_directory')::bigint AS waiting_local_directory,
 count(*) FILTER(WHERE t.status='deferred')::bigint AS deferred
 FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
 WHERE t.status NOT IN ('completed','failed','cancelled')
 AND ow.organization_id=sqlc.arg('organization_id')::uuid AND t.created_at<=sqlc.arg('as_of')::timestamptz
 AND (sqlc.narg('workspace_id')::uuid IS NULL OR a.workspace_id=sqlc.narg('workspace_id'))
)
SELECT outcome.*,usage.*,current_states.* FROM outcome CROSS JOIN usage CROSS JOIN current_states;

-- name: GetAdminOverviewCounts :one
SELECT
 (SELECT count(*) FROM managed_installation WHERE organization_id=sqlc.arg('organization_id')::uuid AND deployment_id=sqlc.arg('deployment_id')::uuid)::bigint AS installations,
 (SELECT count(*) FROM managed_installation WHERE organization_id=sqlc.arg('organization_id')::uuid AND deployment_id=sqlc.arg('deployment_id')::uuid AND lifecycle='retired')::bigint AS retired,
 (SELECT count(*) FROM managed_installation WHERE organization_id=sqlc.arg('organization_id')::uuid AND deployment_id=sqlc.arg('deployment_id')::uuid AND lifecycle='active' AND client_seen_at>sqlc.arg('as_of')::timestamptz-interval '180 seconds' AND client_seen_at<=sqlc.arg('as_of')::timestamptz+interval '1 minute')::bigint AS client_active,
 (SELECT count(*) FROM agent_runtime r JOIN organization_workspace ow ON ow.workspace_id=r.workspace_id WHERE ow.organization_id=sqlc.arg('organization_id')::uuid
  AND NOT EXISTS(SELECT 1 FROM installation_daemon_binding b JOIN managed_installation i ON i.id=b.installation_id WHERE b.workspace_id=r.workspace_id AND b.daemon_id=r.daemon_id AND b.principal_user_id=r.owner_id AND b.state='active' AND i.lifecycle='active' AND i.organization_id=ow.organization_id AND i.deployment_id=sqlc.arg('deployment_id')::uuid))::bigint AS unassociated,
 (SELECT count(*) FROM admin_alert WHERE organization_id=sqlc.arg('organization_id')::uuid AND status='open')::bigint AS alerts_open,
 (SELECT count(*) FROM admin_alert WHERE organization_id=sqlc.arg('organization_id')::uuid AND status='acknowledged')::bigint AS alerts_acknowledged,
 (SELECT count(*) FROM admin_alert WHERE organization_id=sqlc.arg('organization_id')::uuid AND status='resolved' AND first_seen_at>=sqlc.arg('time_from')::timestamptz AND first_seen_at<sqlc.arg('time_to')::timestamptz AND first_seen_at<=sqlc.arg('as_of')::timestamptz)::bigint AS alerts_resolved,
 (SELECT count(*) FROM admin_alert WHERE organization_id=sqlc.arg('organization_id')::uuid AND status='closed' AND first_seen_at>=sqlc.arg('time_from')::timestamptz AND first_seen_at<sqlc.arg('time_to')::timestamptz AND first_seen_at<=sqlc.arg('as_of')::timestamptz)::bigint AS alerts_closed;

-- name: ListAdminOverviewInstallations :many
SELECT id,lifecycle,admission,client_seen_at FROM managed_installation
WHERE organization_id=$1 AND deployment_id=$2 AND lifecycle='active' ORDER BY id LIMIT 10001;

-- name: ListAdminOverviewEvidence :many
SELECT b.installation_id,b.id AS binding_id,b.workspace_id,b.principal_user_id,b.capability_version,b.last_seen_at AS binding_seen_at,
 r.id AS runtime_id,coalesce(r.status,'')::text AS runtime_status,coalesce(r.provider,'')::text AS provider,r.last_seen_at,
 CASE WHEN r.metadata->'offline_reason'->>'code'='not_executable' THEN 'not_executable' ELSE '' END::text AS offline_code,
 coalesce(u.email,'')::text AS principal_email,
 coalesce((i.lifecycle='active' AND u.id IS NOT NULL AND u.disabled_at IS NULL AND c.session_version=b.auth_version AND NOT c.must_change_password
 AND EXISTS(SELECT 1 FROM member m WHERE m.workspace_id=b.workspace_id AND m.user_id=b.principal_user_id)
 AND EXISTS(SELECT 1 FROM daemon_token dt WHERE dt.installation_binding_id=b.id AND dt.installation_binding_epoch=b.binding_epoch AND dt.user_id=b.principal_user_id AND dt.auth_version=b.auth_version AND dt.workspace_id=b.workspace_id AND dt.daemon_id=b.daemon_id AND dt.expires_at>sqlc.arg('as_of')::timestamptz)),false)::boolean AS authorized
FROM managed_installation i JOIN installation_daemon_binding b ON b.installation_id=i.id AND b.state='active'
JOIN organization_workspace ow ON ow.workspace_id=b.workspace_id AND ow.organization_id=i.organization_id
LEFT JOIN agent_runtime r ON r.workspace_id=b.workspace_id AND r.daemon_id=b.daemon_id AND r.owner_id=b.principal_user_id
LEFT JOIN "user" u ON u.id=b.principal_user_id LEFT JOIN user_password_credential c ON c.user_id=b.principal_user_id
WHERE i.organization_id=sqlc.arg('organization_id')::uuid AND i.deployment_id=sqlc.arg('deployment_id')::uuid AND i.lifecycle='active' ORDER BY i.id,b.id,r.id LIMIT 10001;
