-- Identity hints are read before source locks, then re-read under the source
-- lock. If the set grew, the owner rolls back instead of locking users late.
-- name: GetCollaborationWorkflowUserIDs :many
SELECT id FROM (
    SELECT creator_id AS id FROM issue WHERE id = sqlc.arg(issue_id)::uuid
    UNION SELECT assignee_id FROM issue WHERE id = sqlc.arg(issue_id)::uuid
    UNION SELECT user_id FROM issue_participant WHERE issue_id = sqlc.arg(issue_id)::uuid AND revoked_at IS NULL
    UNION SELECT sqlc.arg(actor_id)::uuid
    UNION SELECT sqlc.narg(target_id)::uuid
) identities WHERE id IS NOT NULL ORDER BY id;

-- name: EnrollCollaborationParticipant :one
INSERT INTO issue_participant (issue_id, user_id) VALUES ($1, $2)
ON CONFLICT (issue_id, user_id) DO UPDATE
SET epoch = issue_participant.epoch + 1, joined_at = now(), revoked_at = NULL, revocation_reason = NULL
WHERE issue_participant.revoked_at IS NOT NULL
RETURNING *;

-- name: RevokeCollaborationParticipant :one
UPDATE issue_participant SET epoch = epoch + 1, revoked_at = now(), revocation_reason = sqlc.arg(reason)::text
WHERE issue_id = sqlc.arg(issue_id)::uuid AND user_id = sqlc.arg(user_id)::uuid AND revoked_at IS NULL
RETURNING *;

-- name: AdvanceCollaborationWorkflowRevision :one
UPDATE issue_collaboration SET revision = revision + 1, authority_epoch = authority_epoch + 1
WHERE issue_id = $1 RETURNING revision;

-- name: UpdateCollaborationHandler :one
UPDATE issue SET assignee_type = 'member', assignee_id = sqlc.arg(handler_id)::uuid,
    revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg(issue_id)::uuid RETURNING *;

-- requester_user_id is set only on information requests. parent_request_id
-- links a nested request to its parent, or an information return to the
-- request it consumed.
-- name: CreateCollaborationHandoff :one
INSERT INTO issue_handoff (issue_id, actor_user_id, from_user_id, to_user_id, purpose, note,
    requester_user_id, parent_request_id, previous_status)
VALUES (sqlc.arg(issue_id)::uuid, sqlc.arg(actor_user_id)::uuid, sqlc.arg(from_user_id)::uuid,
    sqlc.arg(to_user_id)::uuid, sqlc.arg(purpose)::text, sqlc.arg(note)::text,
    sqlc.narg(requester_user_id)::uuid, sqlc.narg(parent_request_id)::uuid, sqlc.arg(previous_status)::text)
RETURNING *;

-- name: SupersedeCollaborationInformationRequests :exec
UPDATE issue_handoff SET resolution = 'superseded'
WHERE issue_id = $1 AND purpose = 'information' AND resolution = 'open';

-- name: ClearCollaborationInformationRequest :exec
UPDATE issue_collaboration SET active_information_request_id = NULL WHERE issue_id = $1;

-- The caller holds the issue_collaboration row lock, which serializes every
-- writer of the request chain and its active pointer.
-- name: GetCollaborationInformationRequest :one
SELECT * FROM issue_handoff
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = sqlc.arg(request_id)::uuid AND purpose = 'information';

-- name: SetCollaborationInformationRequest :exec
UPDATE issue_collaboration SET active_information_request_id = sqlc.narg(request_id)::uuid
WHERE issue_id = sqlc.arg(issue_id)::uuid;

-- name: ReturnCollaborationInformationRequest :one
UPDATE issue_handoff SET resolution = 'returned'
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = sqlc.arg(request_id)::uuid
  AND purpose = 'information' AND resolution = 'open'
RETURNING *;

-- name: LockCollaborationOwnerGrants :many
SELECT * FROM collaboration_grant WHERE issue_id = $1 AND owner_user_id = $2 ORDER BY id FOR UPDATE;

-- name: RevokeCollaborationOwnerGrants :exec
UPDATE collaboration_grant SET epoch = epoch + 1, revoked_at = now(), revocation_reason = sqlc.arg(reason)::text
WHERE issue_id = sqlc.arg(issue_id)::uuid AND owner_user_id = sqlc.arg(owner_user_id)::uuid AND revoked_at IS NULL;

-- name: CancelCollaborationAuthorWaitingCommands :many
UPDATE collaboration_command SET state = 'cancelled', reason = sqlc.arg(reason)::text
WHERE session_id = sqlc.arg(session_id)::uuid AND author_user_id = sqlc.arg(author_user_id)::uuid AND state = 'waiting' AND group_id IS NULL
RETURNING *;

-- Preserve the first effective forced-disposition root. The workflow event
-- retains this departure's additional reason. This query never sets proof.
-- name: RequestCollaborationRunCancellation :one
UPDATE collaboration_run SET
    disposition_reason = CASE WHEN forced_disposition IS NULL THEN sqlc.arg(reason)::text ELSE disposition_reason END,
    forced_disposition = COALESCE(forced_disposition, 'cancelled'),
    claim_generation = claim_generation + CASE WHEN progress_state = 'ending' AND forced_disposition IS NOT NULL THEN 0 ELSE 1 END, progress_state = 'ending'
WHERE id = sqlc.arg(run_id)::uuid AND settled_at IS NULL RETURNING *;

-- name: ListCollaborationNotificationRecipients :many
SELECT u.* FROM issue_participant p JOIN "user" u ON u.id = p.user_id
WHERE p.issue_id = $1 AND p.revoked_at IS NULL AND u.disabled_at IS NULL
ORDER BY u.id;
