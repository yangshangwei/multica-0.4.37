-- name: GetCollaborationAccess :one
SELECT sqlc.embed(i), c.revision AS collaboration_revision, c.lifecycle_epoch,
       s.id AS session_id, s.queue_state, s.control_version,
       participant.epoch AS participant_epoch, account_fence.epoch AS account_epoch,
       (u.disabled_at IS NULL)::boolean AS account_active, u.email AS viewer_email,
       EXISTS (SELECT 1 FROM member m WHERE m.workspace_id = i.workspace_id AND m.user_id = u.id) AS source_member,
       EXISTS (SELECT 1 FROM issue_participant p WHERE p.issue_id = i.id AND p.user_id = u.id AND p.revoked_at IS NULL) AS participant,
       COALESCE(st.category, CASE WHEN i.status IN ('backlog', 'todo', 'in_progress', 'in_review', 'blocked', 'done', 'cancelled') THEN i.status ELSE '' END)::text AS status_category,
       information.id AS information_request_id, information.to_user_id AS information_recipient_id
FROM issue i
JOIN issue_collaboration c ON c.issue_id = i.id
JOIN collaboration_session s ON s.issue_id = i.id
JOIN "user" u ON u.id = sqlc.arg(viewer_id)::uuid
LEFT JOIN issue_participant participant ON participant.issue_id = i.id AND participant.user_id = u.id AND participant.revoked_at IS NULL
LEFT JOIN collaboration_account_fence account_fence ON account_fence.user_id = u.id
LEFT JOIN issue_status st ON st.workspace_id = i.workspace_id AND st.key = i.status
-- Only the open top of the request chain grants the directed return action.
LEFT JOIN issue_handoff information ON information.id = c.active_information_request_id
    AND information.issue_id = i.id AND information.purpose = 'information' AND information.resolution = 'open'
WHERE i.id = sqlc.arg(issue_id)::uuid;

-- name: LockCollaborationUsers :many
SELECT * FROM "user" WHERE id = ANY(sqlc.arg(user_ids)::uuid[]) ORDER BY id FOR UPDATE;

-- name: LockCollaborationSourceMember :one
SELECT * FROM member WHERE workspace_id = sqlc.arg(workspace_id)::uuid AND user_id = sqlc.arg(user_id)::uuid FOR KEY SHARE;

-- name: EnsureCollaborationAccountFence :one
INSERT INTO collaboration_account_fence (user_id) VALUES ($1)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id RETURNING *;

-- name: CreateCollaborationOperation :one
INSERT INTO collaboration_operation (scope_kind, scope_id, actor_user_id, request_id, kind, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (scope_kind, scope_id, actor_user_id, request_id) DO NOTHING
RETURNING *;

-- name: GetCollaborationOperation :one
SELECT * FROM collaboration_operation
WHERE scope_kind = $1 AND scope_id = $2 AND actor_user_id = $3 AND request_id = $4;

-- name: CompleteCollaborationCreation :one
UPDATE collaboration_operation SET resolved_issue_id = $2, result_code = 'created', completed_at = now()
WHERE id = $1 AND completed_at IS NULL RETURNING *;

-- name: CreateIssueCollaboration :one
INSERT INTO issue_collaboration (issue_id) VALUES ($1) RETURNING *;

-- name: CreateCollaborationParticipant :one
INSERT INTO issue_participant (issue_id, user_id) VALUES ($1, $2)
ON CONFLICT (issue_id, user_id) DO NOTHING RETURNING *;

-- name: CreateCollaborationSession :one
INSERT INTO collaboration_session (issue_id) VALUES ($1) RETURNING *;

-- name: LockIssueCollaboration :one
SELECT * FROM issue_collaboration WHERE issue_id = $1 FOR UPDATE;

-- name: LockCollaborationSession :one
SELECT * FROM collaboration_session WHERE issue_id = $1 FOR UPDATE;

-- name: AdvanceCollaborationEventSequence :one
UPDATE collaboration_session SET event_seq = event_seq + 1 WHERE issue_id = $1 RETURNING event_seq;

-- name: CreateCollaborationEvent :one
INSERT INTO collaboration_event (issue_id, event_seq, actor_user_id, kind, payload)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: CreateCollaborationNotification :one
INSERT INTO collaboration_notification (recipient_user_id, issue_id, event_seq, kind, is_mention)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (recipient_user_id, issue_id, event_seq) DO NOTHING RETURNING *;

-- name: WorkspaceHasCollaborationRecords :one
SELECT EXISTS (SELECT 1 FROM issue_collaboration c JOIN issue i ON i.id = c.issue_id WHERE i.workspace_id = $1)::boolean;

-- name: GetCollaborationGate :one
SELECT * FROM collaboration_execution_gate WHERE id = 1;

-- name: LockCollaborationGateShared :one
SELECT * FROM collaboration_execution_gate WHERE id = 1 FOR SHARE;

-- name: GetCollaborationParticipant :one
SELECT * FROM issue_participant WHERE issue_id = $1 AND user_id = $2;

-- name: IsCollaborationIssue :one
SELECT EXISTS (SELECT 1 FROM issue_collaboration WHERE issue_id = $1)::boolean;

-- name: AnyCollaborationIssuesInWorkspace :one
SELECT EXISTS (
    SELECT 1 FROM issue_collaboration c JOIN issue i ON i.id = c.issue_id
    WHERE i.workspace_id = sqlc.arg(workspace_id)::uuid AND i.id = ANY(sqlc.arg(issue_ids)::uuid[])
)::boolean;

-- name: UpdateCollaborationPosition :one
UPDATE issue SET position = sqlc.arg(position)::double precision,
    revision = revision + CASE WHEN position IS DISTINCT FROM sqlc.arg(position)::double precision THEN 1 ELSE 0 END
WHERE id = sqlc.arg(issue_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
  AND (sqlc.narg(expected_revision)::bigint IS NULL OR revision = sqlc.narg(expected_revision)::bigint)
RETURNING *;

-- name: ListCollaborationWorkbench :many
WITH viewer AS MATERIALIZED (
    SELECT u.id FROM "user" u
    LEFT JOIN user_password_credential credential ON credential.user_id=u.id
    WHERE u.id=sqlc.arg(viewer_id)::uuid AND u.disabled_at IS NULL
      AND (sqlc.narg(expected_auth_version)::bigint IS NULL
           OR (credential.session_version=sqlc.narg(expected_auth_version)::bigint AND NOT credential.must_change_password))
), visible AS MATERIALIZED (
    SELECT i.id, i.updated_at
    FROM issue i
    JOIN issue_collaboration c ON c.issue_id = i.id
    JOIN issue_participant p ON p.issue_id = i.id AND p.user_id = sqlc.arg(viewer_id)::uuid AND p.revoked_at IS NULL
    JOIN viewer ON viewer.id = p.user_id
    LEFT JOIN issue_status st ON st.workspace_id = i.workspace_id AND st.key = i.status
    WHERE sqlc.arg(view_kind)::text = 'participating'
       OR (i.assignee_type = 'member' AND i.assignee_id = p.user_id
           AND COALESCE(st.category, i.status) IN ('backlog', 'todo', 'in_progress', 'in_review', 'blocked'))
), page AS (
    SELECT i.id, i.title, i.status, i.priority, i.updated_at, i.number,
           c.revision, w.id AS source_workspace_id, w.name AS source_name, w.issue_prefix AS source_prefix,
           COALESCE(st.category, i.status)::text AS status_category,
           i.assignee_id AS handler_user_id, handler.name AS handler_name, handler.avatar_url AS handler_avatar_url,
           NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=i.assignee_id) AS handler_external
    FROM visible v JOIN issue i ON i.id=v.id
    JOIN issue_collaboration c ON c.issue_id=i.id
    JOIN workspace w ON w.id=i.workspace_id
    LEFT JOIN issue_status st ON st.workspace_id=i.workspace_id AND st.key=i.status
    LEFT JOIN "user" handler ON handler.id=i.assignee_id
    WHERE sqlc.narg(cursor_updated_at)::timestamptz IS NULL
       OR (v.updated_at,v.id) < (sqlc.narg(cursor_updated_at)::timestamptz,sqlc.narg(cursor_id)::uuid)
    ORDER BY v.updated_at DESC,v.id DESC
    LIMIT sqlc.arg(page_limit)::int
), total AS (SELECT count(*)::bigint AS total_count FROM visible)
SELECT (EXISTS (SELECT 1 FROM viewer))::boolean AS viewer_active,
       total.total_count, page.id, COALESCE(page.title,'')::text AS title,
       COALESCE(page.status,'')::text AS status, COALESCE(page.status_category,'')::text AS status_category,
       COALESCE(page.priority,'')::text AS priority, page.updated_at,
       COALESCE(page.number,0)::int AS number, COALESCE(page.revision,0)::bigint AS revision,
       page.source_workspace_id, COALESCE(page.source_name,'')::text AS source_name,
       COALESCE(page.source_prefix,'')::text AS source_prefix, page.handler_user_id,
       COALESCE(page.handler_name,'Unavailable account')::text AS handler_name,
       COALESCE(page.handler_avatar_url,'')::text AS handler_avatar_url,
       COALESCE(page.handler_external,false)::boolean AS handler_external
FROM total LEFT JOIN page ON true ORDER BY page.updated_at DESC,page.id DESC;

-- Queue mutations borrow the same transaction as current object authorization.
-- Locking the source issue, permanent session and target serializes input order,
-- cancellation and stopping; none of these queries grants execution admission.

-- name: LockCollaborationIssue :one
SELECT * FROM issue WHERE id = sqlc.arg(issue_id)::uuid FOR NO KEY UPDATE;

-- name: LockCollaborationParticipant :one
SELECT * FROM issue_participant
WHERE issue_id = sqlc.arg(issue_id)::uuid AND user_id = sqlc.arg(user_id)::uuid
FOR UPDATE;

-- name: GetCollaborationAccountFence :one
SELECT * FROM collaboration_account_fence WHERE user_id = sqlc.arg(user_id)::uuid;

-- name: AllocateCollaborationCommandSequence :one
UPDATE collaboration_channel SET next_command_seq = next_command_seq + 1
WHERE id = sqlc.arg(channel_id)::uuid AND session_id = sqlc.arg(session_id)::uuid
RETURNING (next_command_seq - 1)::bigint AS command_seq;

-- name: CreateCollaborationCommand :one
INSERT INTO collaboration_command (session_id, target_id, input_event_id, seq, author_user_id, participant_epoch, author_auth_epoch, lifecycle_epoch, content,channel_id,batch_id)
VALUES (sqlc.arg(session_id)::uuid, sqlc.arg(target_id)::uuid, sqlc.arg(input_event_id)::uuid, sqlc.arg(seq)::bigint, sqlc.arg(author_user_id)::uuid,
        sqlc.arg(participant_epoch)::bigint, sqlc.arg(author_auth_epoch)::bigint,
        sqlc.arg(lifecycle_epoch)::bigint, sqlc.arg(content)::text,sqlc.arg(channel_id)::uuid,sqlc.arg(batch_id)::uuid)
RETURNING *;

-- name: LockCollaborationCommand :one
SELECT * FROM collaboration_command
WHERE session_id = sqlc.arg(session_id)::uuid AND id = sqlc.arg(command_id)::uuid
FOR UPDATE;

-- name: CancelWaitingCollaborationCommand :one
UPDATE collaboration_command SET state = 'cancelled', reason = sqlc.arg(reason)::text
WHERE session_id = sqlc.arg(session_id)::uuid AND id = sqlc.arg(command_id)::uuid AND state = 'waiting'
RETURNING *;

-- name: CompleteCollaborationOperation :one
UPDATE collaboration_operation
SET result_code = sqlc.arg(result_code)::text, result = sqlc.arg(result)::jsonb,
    resolved_issue_id = sqlc.arg(resolved_issue_id)::uuid, completed_at = now()
WHERE id = sqlc.arg(id)::uuid AND completed_at IS NULL
RETURNING *;

-- name: GetCollaborationRun :one
SELECT * FROM collaboration_run
WHERE session_id = sqlc.arg(session_id)::uuid AND id = sqlc.arg(run_id)::uuid;

-- name: PauseCollaborationSession :one
UPDATE collaboration_session
SET queue_state = CASE WHEN queue_state = 'closed' THEN 'closed' ELSE 'paused' END,
    pause_reason = sqlc.arg(pause_reason)::text, control_version = control_version + 1
WHERE id = sqlc.arg(session_id)::uuid
RETURNING *;

-- name: RequestCollaborationRunStop :one
UPDATE collaboration_run
SET disposition_reason = CASE WHEN forced_disposition IS NULL THEN sqlc.arg(disposition_reason)::text ELSE disposition_reason END,
    forced_disposition = COALESCE(forced_disposition, 'stopped'),
    claim_generation = claim_generation + CASE WHEN progress_state = 'ending' AND forced_disposition IS NOT NULL THEN 0 ELSE 1 END, progress_state = 'ending'
WHERE id = sqlc.arg(run_id)::uuid AND settled_at IS NULL
RETURNING *;

-- A server-issued launch permit is the authority here. Absence of a PID,
-- a disconnected daemon or an elapsed timeout cannot establish this proof.
-- The caller holds the run lock and verifies the original pre-stop progress
-- and outcome before RequestCollaborationRunStop overwrites progress_state.
-- Conflicting running/success evidence must retain the barrier.
-- name: SettleNeverLaunchedCollaborationRun :one
UPDATE collaboration_run
SET sealed_attempt_seq = 0, proof_state = 'never_launched_proven', progress_state = 'settled', settled_at = now()
WHERE id = sqlc.arg(run_id)::uuid AND authorized_attempt_seq = 0
  AND started_at IS NULL AND settled_at IS NULL AND proof_state = 'unknown'
  AND (business_outcome IS NULL OR business_outcome = 'failure')
RETURNING *;

-- name: FinishCollaborationCommand :one
UPDATE collaboration_command SET state = sqlc.arg(state)::text, reason = sqlc.arg(reason)::text
WHERE session_id = sqlc.arg(session_id)::uuid AND id = sqlc.arg(command_id)::uuid
  AND state = 'in_flight' AND sqlc.arg(state)::text IN ('stopped', 'cancelled')
RETURNING *;

-- name: RequestCollaborationTerminationStop :exec
UPDATE collaboration_termination SET stop_requested_at = COALESCE(stop_requested_at, now())
WHERE run_id = sqlc.arg(run_id)::uuid;
