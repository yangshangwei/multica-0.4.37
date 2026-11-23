-- name: LockIssueDeleteTasks :exec
SELECT task.id FROM agent_task_queue task JOIN issue owner ON owner.id=task.issue_id
WHERE owner.workspace_id=sqlc.arg('workspace_id') AND owner.id=ANY(sqlc.arg('issue_ids')::uuid[])
 AND task.status IN ('queued','dispatched','running','waiting_local_directory','deferred')
ORDER BY task.id FOR UPDATE OF task NOWAIT;

-- name: LockIssueDeleteTaskTokens :exec
SELECT token.id FROM task_token token JOIN agent_task_queue task ON task.id=token.task_id
JOIN issue owner ON owner.id=task.issue_id
WHERE owner.workspace_id=sqlc.arg('workspace_id') AND owner.id=ANY(sqlc.arg('issue_ids')::uuid[])
 AND task.status IN ('queued','dispatched','running','waiting_local_directory','deferred')
ORDER BY token.id FOR UPDATE OF token NOWAIT;

-- name: LockIssueDeleteAutopilotRuns :exec
SELECT run.id FROM autopilot_run run JOIN issue owner ON owner.id=run.issue_id
WHERE owner.workspace_id=sqlc.arg('workspace_id') AND owner.id=ANY(sqlc.arg('issue_ids')::uuid[])
 AND run.status IN ('issue_created','running')
ORDER BY run.id FOR UPDATE OF run NOWAIT;

-- name: LockIssueDeleteAutopilotQuotaReservations :exec
SELECT reservation.id FROM autopilot_quota_reservation reservation
JOIN autopilot_run run ON run.quota_reservation_id=reservation.id JOIN issue owner ON owner.id=run.issue_id
WHERE owner.workspace_id=sqlc.arg('workspace_id') AND owner.id=ANY(sqlc.arg('issue_ids')::uuid[])
 AND run.status IN ('issue_created','running') AND reservation.state='reserved'
ORDER BY reservation.id FOR UPDATE OF reservation NOWAIT;

-- name: LockIssueDeleteAutopilotQuotaPeriods :exec
SELECT period.workspace_id FROM autopilot_quota_period period
JOIN autopilot_quota_reservation reservation ON reservation.workspace_id=period.workspace_id
 AND reservation.period_start=period.period_start AND reservation.period_end=period.period_end
JOIN autopilot_run run ON run.quota_reservation_id=reservation.id JOIN issue owner ON owner.id=run.issue_id
WHERE owner.workspace_id=sqlc.arg('workspace_id') AND owner.id=ANY(sqlc.arg('issue_ids')::uuid[])
 AND run.status IN ('issue_created','running') AND reservation.state='reserved'
ORDER BY period.workspace_id,period.period_start,period.period_end FOR UPDATE OF period NOWAIT;
