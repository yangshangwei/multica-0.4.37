-- All report-source readers borrow the capture transaction and its fixed scope.
-- name: ListReportSnapshotSourceIssues :many
SELECT i.id,i.workspace_id,i.project_id,i.revision,i.status,i.admission_status,
 i.created_at,i.due_date,i.assignee_type,i.assignee_id,i.origin_type,i.origin_id,
 rs.id AS known_report_snapshot_id,COALESCE(rs.report_revision,0)::bigint AS known_report_revision,
 COALESCE(rs.sequence,0)::bigint AS known_report_sequence,
 a.id AS autopilot_id,COALESCE(a.template_key,'')::text AS template_key,
 COALESCE(a.template_version,0)::integer AS template_version,
 ar.id AS linked_run_id,ar.created_at AS linked_run_created_at
FROM issue i
LEFT JOIN autopilot a ON i.origin_type='autopilot' AND a.id=i.origin_id AND a.workspace_id=i.workspace_id
LEFT JOIN LATERAL (
 SELECT s.id,s.report_revision,s.sequence FROM report_snapshot s
 WHERE s.workspace_id=i.workspace_id AND s.report_issue_id=i.id
 ORDER BY s.sequence DESC LIMIT 1
) rs ON true
LEFT JOIN LATERAL (
 SELECT r.id,r.created_at FROM autopilot_run r
 WHERE r.autopilot_id=a.id AND r.issue_id=i.id
 ORDER BY r.created_at ASC,r.id ASC LIMIT 1
) ar ON true
WHERE i.workspace_id=sqlc.arg('workspace_id')::uuid
 AND ((sqlc.narg('project_id')::uuid IS NOT NULL AND i.project_id=sqlc.narg('project_id')::uuid)
      OR (sqlc.narg('project_id')::uuid IS NULL AND i.id=sqlc.arg('report_issue_id')::uuid))
 AND i.admission_status IN ('not_required','accepted')
 AND NOT EXISTS (SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)
 AND i.id>sqlc.arg('after_id')::uuid
ORDER BY i.id ASC LIMIT sqlc.arg('row_limit')::integer;

-- name: ListReportSnapshotSourceActivities :many
SELECT id,issue_id,action,created_at,
 COALESCE(details->>'from','')::text AS from_status,
 COALESCE(details->>'to','')::text AS to_status
FROM activity_log
WHERE workspace_id=sqlc.arg('workspace_id')::uuid
 AND issue_id=ANY(sqlc.arg('issue_ids')::uuid[])
 AND action='status_changed'
 AND (created_at,id)>(sqlc.arg('after_time')::timestamptz,sqlc.arg('after_id')::uuid)
ORDER BY created_at ASC,id ASC LIMIT sqlc.arg('row_limit')::integer;

-- name: ListReportSnapshotSourceStatuses :many
SELECT id,key,category FROM issue_status
WHERE workspace_id=sqlc.arg('workspace_id')::uuid
 AND key=ANY(sqlc.arg('status_keys')::text[])
ORDER BY key ASC;

-- name: GetReportSnapshotSourceLatestUpdate :one
SELECT id,current_revision,published_at FROM project_update
WHERE workspace_id=sqlc.arg('workspace_id')::uuid AND project_id=sqlc.arg('project_id')::uuid
ORDER BY published_at DESC,id DESC LIMIT 1;
