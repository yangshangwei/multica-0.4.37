-- Parent project is locked before any of these update/write queries.
-- name: CreateProjectUpdate :one
INSERT INTO project_update (id,workspace_id,project_id,author_user_id)
VALUES (sqlc.arg('id'),sqlc.arg('workspace_id'),sqlc.arg('project_id'),sqlc.arg('author_user_id')) RETURNING *;

-- name: GetProjectUpdate :one
SELECT * FROM project_update WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') AND project_id=sqlc.arg('project_id');

-- name: ListProjectUpdates :many
SELECT * FROM project_update WHERE workspace_id=sqlc.arg('workspace_id') AND project_id=sqlc.arg('project_id')
AND (sqlc.narg('before_time')::timestamptz IS NULL OR (published_at,id)<(sqlc.narg('before_time')::timestamptz,sqlc.narg('before_id')::uuid))
ORDER BY published_at DESC,id DESC LIMIT sqlc.arg('row_limit');

-- name: AdvanceProjectUpdateRevision :one
UPDATE project_update SET current_revision=current_revision+1
WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') AND project_id=sqlc.arg('project_id') AND current_revision=sqlc.arg('expected_revision') RETURNING *;

-- name: CreateProjectUpdateRevision :one
INSERT INTO project_update_revision(id,workspace_id,project_id,update_id,revision,editor_user_id,kind,body,health_judgment,correction_reason,evidence,statistics_snapshot,acceptance)
VALUES(sqlc.arg('id'),sqlc.arg('workspace_id'),sqlc.arg('project_id'),sqlc.arg('update_id'),sqlc.arg('revision'),sqlc.arg('editor_user_id'),sqlc.arg('kind'),sqlc.arg('body'),sqlc.narg('health_judgment'),sqlc.narg('correction_reason'),sqlc.arg('evidence'),sqlc.narg('statistics_snapshot'),sqlc.narg('acceptance')) RETURNING *;

-- name: GetProjectUpdateRevision :one
SELECT * FROM project_update_revision WHERE workspace_id=sqlc.arg('workspace_id') AND project_id=sqlc.arg('project_id') AND update_id=sqlc.arg('update_id') AND revision=sqlc.arg('revision');

-- name: ListProjectUpdateRevisions :many
SELECT * FROM project_update_revision WHERE workspace_id=sqlc.arg('workspace_id') AND project_id=sqlc.arg('project_id') AND update_id=sqlc.arg('update_id')
AND (sqlc.narg('before_revision')::bigint IS NULL OR revision<sqlc.narg('before_revision')::bigint)
ORDER BY revision DESC LIMIT sqlc.arg('row_limit');

-- name: GetProjectUpdateRequest :one
SELECT * FROM project_update_request WHERE workspace_id=sqlc.arg('workspace_id') AND actor_user_id=sqlc.arg('actor_user_id') AND request_id=sqlc.arg('request_id');

-- name: CreateProjectUpdateRequest :exec
INSERT INTO project_update_request(workspace_id,project_id,actor_user_id,request_id,operation,payload_hash,update_id,result_revision)
VALUES(sqlc.arg('workspace_id'),sqlc.arg('project_id'),sqlc.arg('actor_user_id'),sqlc.arg('request_id'),sqlc.arg('operation'),sqlc.arg('payload_hash'),sqlc.arg('update_id'),sqlc.arg('result_revision'));

-- name: ListProjectUpdateMemberIDs :many
SELECT user_id FROM member WHERE workspace_id=sqlc.arg('workspace_id') AND user_id=ANY(sqlc.arg('user_ids')::uuid[]) ORDER BY user_id;

-- name: LockProjectUpdateRecipients :many
SELECT user_id FROM member WHERE workspace_id=sqlc.arg('workspace_id') AND user_id=ANY(sqlc.arg('user_ids')::uuid[]) ORDER BY user_id FOR SHARE NOWAIT;

-- Evidence locks fail NOWAIT: retry the entire RR transaction, never retain an old authorization snapshot.
-- name: LockProjectUpdateEvidenceIssue :one
SELECT * FROM issue WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') FOR SHARE NOWAIT;

-- name: LockProjectUpdateEvidenceExecution :one
SELECT t.* FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id
WHERE t.id=sqlc.arg('id') AND a.workspace_id=sqlc.arg('workspace_id') FOR SHARE OF t NOWAIT;

-- name: LockProjectUpdateEvidenceAgent :one
SELECT * FROM agent WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') FOR SHARE NOWAIT;

-- name: LockProjectUpdateEvidenceChat :one
SELECT * FROM chat_session WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') FOR SHARE NOWAIT;

-- name: LockProjectUpdateEvidenceTargets :many
SELECT * FROM agent_invocation_target WHERE agent_id=sqlc.arg('agent_id') ORDER BY target_type,target_id FOR SHARE NOWAIT;

-- Notification recipients are already parsed from explicit member mentions and authorized.
-- name: CreateProjectUpdateNotification :exec
INSERT INTO project_update_notification(id,workspace_id,project_id,update_id,recipient_user_id,source_revision)
VALUES(sqlc.arg('id'),sqlc.arg('workspace_id'),sqlc.arg('project_id'),sqlc.arg('update_id'),sqlc.arg('recipient_user_id'),sqlc.arg('source_revision'))
ON CONFLICT(workspace_id,update_id,recipient_user_id) DO NOTHING;

-- Deliberately NO row locks while scanning. Per candidate: workspace > recipient fence/member > project > notification.
-- name: ListDueProjectUpdateNotifications :many
SELECT * FROM project_update_notification WHERE status='pending' AND (next_attempt_at IS NULL OR next_attempt_at<=now())
ORDER BY next_attempt_at NULLS FIRST,id LIMIT 100;

-- name: LockProjectUpdateNotification :one
SELECT * FROM project_update_notification WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') AND project_id=sqlc.arg('project_id')
AND status='pending' AND (next_attempt_at IS NULL OR next_attempt_at<=now()) FOR UPDATE SKIP LOCKED;

-- name: CompleteProjectUpdateNotification :exec
UPDATE project_update_notification SET status=sqlc.arg('status'),delivered_at=CASE WHEN sqlc.arg('status')='delivered' THEN now() ELSE NULL END,
last_error_code=sqlc.narg('last_error_code') WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id');

-- name: RetryProjectUpdateNotification :exec
UPDATE project_update_notification SET attempts=attempts+1,status=CASE WHEN attempts+1>=12 THEN 'dead_letter' ELSE 'pending' END,
next_attempt_at=sqlc.arg('next_attempt_at'),last_error_code=sqlc.arg('last_error_code') WHERE id=sqlc.arg('id') AND workspace_id=sqlc.arg('workspace_id') AND status='pending';

-- name: CreateProjectUpdateInbox :one
INSERT INTO inbox_item(id,workspace_id,recipient_type,recipient_id,type,severity,title,body,actor_type,actor_id,details)
VALUES(sqlc.arg('id'),sqlc.arg('workspace_id'),'member',sqlc.arg('recipient_id'),'project_update','info',sqlc.arg('title'),'','member',sqlc.arg('actor_id'),sqlc.arg('details'))
ON CONFLICT(id) DO NOTHING RETURNING *;
