-- name: LockAdminAlertFingerprint :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg('fingerprint_lock')::text,728401));

-- name: GetLatestAdminAlert :one
SELECT * FROM admin_alert WHERE organization_id=$1 AND fingerprint=$2
ORDER BY first_seen_at DESC,id DESC LIMIT 1 FOR UPDATE;

-- name: CreateAdminAlert :one
INSERT INTO admin_alert (organization_id,rule,subject_kind,subject_id,fingerprint,severity,
 first_seen_at,last_seen_at,last_observed_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$7,$7) RETURNING *;

-- name: ObserveAdminAlertCondition :one
UPDATE admin_alert SET last_seen_at=GREATEST(last_seen_at,sqlc.arg('observed_at')::timestamptz),
 last_observed_at=GREATEST(last_observed_at,sqlc.arg('observed_at')::timestamptz),updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND condition_active RETURNING *;

-- name: ObserveAdminAlertRecovery :one
UPDATE admin_alert SET condition_active=false,
 status=CASE WHEN status IN ('open','acknowledged') THEN 'resolved' ELSE status END,
 resolved_at=CASE WHEN condition_active THEN sqlc.arg('observed_at')::timestamptz ELSE resolved_at END,
 resolution_code=CASE WHEN condition_active THEN sqlc.arg('resolution_code')::text ELSE resolution_code END,
 version=version+CASE WHEN condition_active THEN 1 ELSE 0 END,
 last_observed_at=sqlc.arg('observed_at')::timestamptz,updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND rule<>'execution_failed'
 AND last_observed_at<=sqlc.arg('observed_at')::timestamptz RETURNING *;

-- name: GetAdminAlert :one
SELECT * FROM admin_alert WHERE id=$1 AND organization_id=$2;

-- name: GetAdminAlertForUpdate :one
SELECT * FROM admin_alert WHERE id=$1 AND organization_id=$2 FOR UPDATE;

-- name: UpdateAdminAlertAction :one
UPDATE admin_alert SET status=sqlc.arg('status')::text,assignee_id=sqlc.narg('assignee_id')::uuid,
 acknowledged_at=sqlc.narg('acknowledged_at')::timestamptz,
 resolved_at=sqlc.narg('resolved_at')::timestamptz,closed_at=sqlc.narg('closed_at')::timestamptz,
 resolution_code=sqlc.narg('resolution_code')::text,related_task_id=sqlc.narg('related_task_id')::uuid,
 operation_id=sqlc.arg('operation_id')::uuid,version=version+1,updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND organization_id=sqlc.arg('organization_id')::uuid
 AND version=sqlc.arg('expected_version')::bigint AND version<9223372036854775807 RETURNING *;

-- name: FinishAdminAlertOperation :one
UPDATE admin_operation SET state=sqlc.arg('state')::text,result_code=sqlc.arg('result_code')::text,
 confirmation='not_required',reconciliation_state='complete',
 applied_at=sqlc.narg('applied_at')::timestamptz,version=version+1,updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND target_kind='alert'
 AND kind IN ('alert.acknowledge','alert.assign','alert.close') RETURNING *;

-- name: IsCompletedAdminAlertRetry :one
WITH RECURSIVE lineage AS (
 SELECT t.id,t.retry_of_task_id,t.rerun_of_task_id,ARRAY[t.id] AS visited,0 AS depth
 FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id
 JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
 WHERE t.id=sqlc.arg('related_task_id')::uuid AND t.status='completed'
  AND ow.organization_id=sqlc.arg('organization_id')::uuid
 UNION ALL
 SELECT parent.id,parent.retry_of_task_id,parent.rerun_of_task_id,lineage.visited||parent.id,lineage.depth+1
 FROM lineage JOIN agent_task_queue parent ON parent.id IN (lineage.retry_of_task_id,lineage.rerun_of_task_id)
 JOIN agent a ON a.id=parent.agent_id JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
 WHERE lineage.depth<64 AND NOT parent.id=ANY(lineage.visited)
  AND ow.organization_id=sqlc.arg('organization_id')::uuid
)
SELECT EXISTS(SELECT 1 FROM lineage WHERE id=sqlc.arg('original_task_id')::uuid AND depth>0);

-- name: ListAdminAlerts :many
SELECT * FROM admin_alert WHERE organization_id=sqlc.arg('organization_id')::uuid
 AND first_seen_at<=sqlc.arg('as_of')::timestamptz
 AND (sqlc.narg('time_from')::timestamptz IS NULL OR first_seen_at>=sqlc.narg('time_from'))
 AND first_seen_at<sqlc.arg('time_to')::timestamptz
 AND (sqlc.arg('status')::text='all' OR status=sqlc.arg('status')
  OR (sqlc.arg('status')='active' AND status IN ('open','acknowledged')))
 AND (sqlc.arg('rule')::text='' OR rule=sqlc.arg('rule'))
 AND (sqlc.arg('severity')::text='' OR severity=sqlc.arg('severity'))
 AND (sqlc.arg('subject_kind')::text='' OR subject_kind=sqlc.arg('subject_kind'))
 AND (sqlc.narg('subject_id')::uuid IS NULL OR subject_id=sqlc.narg('subject_id'))
 AND (sqlc.narg('assignee_id')::uuid IS NULL OR assignee_id=sqlc.narg('assignee_id'))
 AND (sqlc.arg('search')::text='' OR id::text ILIKE '%'||sqlc.arg('search')||'%'
  OR subject_id::text ILIKE '%'||sqlc.arg('search')||'%')
 AND (sqlc.narg('after_time')::timestamptz IS NULL OR (first_seen_at,id)<(sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY first_seen_at DESC,id DESC LIMIT sqlc.arg('page_limit')::int;

-- name: EnsureAdminAlertDetector :exec
INSERT INTO admin_alert_detector_state (organization_id,rule) VALUES ($1,$2)
ON CONFLICT (organization_id,rule) DO NOTHING;

-- name: LeaseAdminAlertDetector :one
UPDATE admin_alert_detector_state SET lease_owner=sqlc.arg('lease_owner')::uuid,
 lease_until=sqlc.arg('lease_until')::timestamptz,last_started_at=sqlc.arg('observed_at')::timestamptz,
 cycle_started_at=CASE WHEN cursor_time IS NULL OR cycle_started_at IS NULL THEN sqlc.arg('observed_at')::timestamptz ELSE cycle_started_at END,
 version=version+1,updated_at=now()
WHERE organization_id=sqlc.arg('organization_id')::uuid AND rule=sqlc.arg('rule')::text
 AND (lease_until IS NULL OR lease_until<sqlc.arg('observed_at')::timestamptz)
RETURNING *;

-- name: LockAdminAlertDetectorLease :one
SELECT * FROM admin_alert_detector_state
WHERE organization_id=$1 AND rule=$2 AND lease_owner=$3 AND version=$4 FOR UPDATE;

-- name: CompleteAdminAlertDetectorBatch :execrows
UPDATE admin_alert_detector_state SET cursor_time=sqlc.narg('cursor_time')::timestamptz,
 cursor_id=sqlc.narg('cursor_id')::uuid,source_state=sqlc.arg('source_state')::text,
 last_error_code=sqlc.narg('last_error_code')::text,
 last_successful_at=sqlc.arg('observed_at')::timestamptz,
 scan_complete=sqlc.arg('scan_complete')::boolean,cycle_unknown_count=sqlc.arg('cycle_unknown_count')::bigint,
 lease_owner=NULL,lease_until=NULL,version=version+1,updated_at=now()
WHERE organization_id=sqlc.arg('organization_id')::uuid AND rule=sqlc.arg('rule')::text
 AND lease_owner=sqlc.arg('lease_owner')::uuid AND version=sqlc.arg('expected_version')::bigint;

-- name: FailAdminAlertDetectorBatch :exec
UPDATE admin_alert_detector_state SET source_state='unavailable',last_error_code=sqlc.arg('error_code')::text,
 scan_complete=false,lease_owner=NULL,lease_until=NULL,version=version+1,updated_at=now()
WHERE organization_id=sqlc.arg('organization_id')::uuid AND rule=sqlc.arg('rule')::text
 AND lease_owner=sqlc.arg('lease_owner')::uuid AND version=sqlc.arg('expected_version')::bigint;

-- name: ListAdminAlertDetectorHealth :many
SELECT * FROM admin_alert_detector_state WHERE organization_id=$1 ORDER BY rule;

-- name: ListAdminAlertFailureCandidates :many
SELECT t.id AS subject_id,t.completed_at AS cursor_time FROM agent_task_queue t
JOIN agent a ON a.id=t.agent_id JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
WHERE ow.organization_id=sqlc.arg('organization_id')::uuid AND t.status='failed'
 AND t.completed_at>=sqlc.arg('window_from')::timestamptz AND t.completed_at<=sqlc.arg('observed_at')::timestamptz
 AND (sqlc.narg('cursor_time')::timestamptz IS NULL OR (t.completed_at,t.id)>(sqlc.narg('cursor_time'),sqlc.narg('cursor_id')::uuid))
ORDER BY t.completed_at,t.id LIMIT sqlc.arg('batch_limit')::int;

-- name: ListAdminAlertQueueCandidates :many
WITH candidates AS (
 SELECT t.id AS subject_id,COALESCE(t.queued_at,t.created_at) AS cursor_time,t.status,t.queued_at,t.queued_at_source,true AS subject_exists
 FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id
 JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
 WHERE ow.organization_id=sqlc.arg('organization_id')::uuid AND t.status='queued'
 UNION ALL
 SELECT alert.subject_id,COALESCE(t.queued_at,t.created_at,alert.first_seen_at) AS cursor_time,
  COALESCE(t.status,'') AS status,t.queued_at,t.queued_at_source,
  EXISTS(SELECT 1 FROM agent_task_queue existing WHERE existing.id=alert.subject_id) AS subject_exists
 FROM admin_alert alert LEFT JOIN agent_task_queue t ON t.id=alert.subject_id
  AND EXISTS(SELECT 1 FROM agent a JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
   WHERE a.id=t.agent_id AND ow.organization_id=alert.organization_id)
 WHERE alert.organization_id=sqlc.arg('organization_id')::uuid AND alert.rule='queue_timeout'
  AND alert.condition_active AND t.status IS DISTINCT FROM 'queued'
)
SELECT * FROM candidates WHERE cursor_time<=sqlc.arg('observed_at')::timestamptz
 AND (sqlc.narg('cursor_time')::timestamptz IS NULL OR (cursor_time,subject_id)>(sqlc.narg('cursor_time'),sqlc.narg('cursor_id')::uuid))
ORDER BY cursor_time,subject_id LIMIT sqlc.arg('batch_limit')::int;

-- name: ListAdminAlertInstallationCandidates :many
SELECT i.id AS subject_id,i.created_at AS cursor_time,i.lifecycle,
 (SELECT count(*) FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id
  JOIN organization_workspace own_scope ON own_scope.workspace_id=a.workspace_id AND own_scope.organization_id=i.organization_id
  WHERE t.execution_installation_id=i.id
  AND t.status IN ('dispatched','running','waiting_local_directory'))::bigint AS inflight_count
FROM managed_installation i WHERE i.organization_id=sqlc.arg('organization_id')::uuid
 AND i.deployment_id=sqlc.arg('deployment_id')::uuid AND i.created_at<=sqlc.arg('observed_at')::timestamptz
 AND (sqlc.narg('cursor_time')::timestamptz IS NULL OR (i.created_at,i.id)>(sqlc.narg('cursor_time'),sqlc.narg('cursor_id')::uuid))
ORDER BY i.created_at,i.id LIMIT sqlc.arg('batch_limit')::int;

-- name: ListAdminAlertInstallationEvidence :many
SELECT b.installation_id,b.id AS binding_id,b.last_seen_at AS binding_seen_at,
 r.id AS runtime_id,r.last_seen_at AS runtime_seen_at,
 (SELECT count(*) FROM agent_task_queue task JOIN agent owner ON owner.id=task.agent_id AND owner.workspace_id=b.workspace_id
  WHERE task.runtime_id=r.id
  AND task.execution_installation_id=b.installation_id AND task.execution_binding_id=b.id
  AND task.execution_binding_epoch=b.binding_epoch AND task.status IN ('dispatched','running','waiting_local_directory'))::bigint AS inflight_count,
 b.principal_user_id,COALESCE(u.email,'')::text AS principal_email,
 COALESCE(u.id IS NOT NULL AND u.disabled_at IS NULL AND c.session_version=b.auth_version
  AND NOT c.must_change_password AND EXISTS(SELECT 1 FROM member m WHERE m.workspace_id=b.workspace_id AND m.user_id=b.principal_user_id)
  AND EXISTS(SELECT 1 FROM daemon_token dt WHERE dt.installation_binding_id=b.id AND dt.installation_binding_epoch=b.binding_epoch
   AND dt.user_id=b.principal_user_id AND dt.auth_version=b.auth_version
   AND dt.workspace_id=b.workspace_id AND dt.daemon_id=b.daemon_id AND dt.expires_at>sqlc.arg('observed_at')::timestamptz),false)::boolean AS authorized
FROM installation_daemon_binding b JOIN managed_installation i ON i.id=b.installation_id
JOIN organization_workspace ow ON ow.workspace_id=b.workspace_id AND ow.organization_id=i.organization_id
LEFT JOIN agent_runtime r ON r.workspace_id=b.workspace_id AND r.daemon_id=b.daemon_id AND r.owner_id=b.principal_user_id
LEFT JOIN "user" u ON u.id=b.principal_user_id LEFT JOIN user_password_credential c ON c.user_id=b.principal_user_id
WHERE i.organization_id=sqlc.arg('organization_id')::uuid AND i.deployment_id=sqlc.arg('deployment_id')::uuid
 AND b.state='active' AND b.installation_id=ANY(sqlc.arg('installation_ids')::uuid[])
ORDER BY b.installation_id,b.id,r.id LIMIT 2001;

-- name: ListAdminAlertFailureHints :many
SELECT t.id AS subject_id,t.completed_at AS cursor_time FROM agent_task_queue t
JOIN agent a ON a.id=t.agent_id JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
WHERE ow.organization_id=sqlc.arg('organization_id')::uuid AND t.status='failed'
 AND t.id=ANY(sqlc.arg('task_ids')::uuid[])
 AND t.completed_at>=sqlc.arg('window_from')::timestamptz AND t.completed_at<=sqlc.arg('observed_at')::timestamptz
ORDER BY t.completed_at,t.id LIMIT 200;
