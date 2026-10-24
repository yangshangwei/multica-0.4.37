-- One worker owns one intent for one source issue at a time. A locked earlier
-- epoch still blocks later epochs for that account; SKIP LOCKED only permits
-- other accounts to make progress. No credential/user lock is taken here.
-- A failed account is excluded only for the current bounded pass, without
-- advancing its durable cursor or letting its later epochs overtake it.
-- name: LockNextCollaborationAccountInvalidation :one
SELECT intent.* FROM collaboration_account_invalidation AS intent
WHERE intent.reconciled_at IS NULL
  AND intent.user_id <> ALL(COALESCE(sqlc.arg(excluded_user_ids)::uuid[], '{}'))
  AND NOT EXISTS (
      SELECT 1 FROM collaboration_account_invalidation AS earlier
      WHERE earlier.user_id = intent.user_id
        AND earlier.account_epoch < intent.account_epoch
        AND earlier.reconciled_at IS NULL
  )
ORDER BY COALESCE(intent.last_attempted_at, intent.created_at), intent.user_id, intent.account_epoch
LIMIT 1 FOR UPDATE OF intent SKIP LOCKED;

-- A failed source transaction has rolled back before this independent statement.
-- Rotate the original intent without advancing its source cursor. If another
-- worker owns it now, that worker will record its own attempt; do not wait on it.
-- This persists fairness when an entire bounded pass encounters failed accounts.
-- name: RecordCollaborationInvalidationAttempt :exec
WITH target AS (
    SELECT user_id, account_epoch FROM collaboration_account_invalidation
    WHERE user_id = sqlc.arg(user_id)::uuid AND account_epoch = sqlc.arg(account_epoch)::bigint
      AND reconciled_at IS NULL
    FOR UPDATE SKIP LOCKED
)
UPDATE collaboration_account_invalidation AS intent
SET last_attempted_at = clock_timestamp()
FROM target
WHERE intent.user_id = target.user_id AND intent.account_epoch = target.account_epoch;

-- The cursor and source effects commit together. All writers of new commands
-- and grants must hold the current account/user fence and reject stale epochs:
-- after the invalidation commits they cannot insert old authority behind this
-- cursor. Retained grants also find old runs after a grant has been revoked.
-- The LEFT JOIN keeps a missing source visible as an integrity failure.
-- name: NextCollaborationAccountInvalidationIssue :one
SELECT affected.issue_id, i.workspace_id FROM (
    SELECT s.issue_id FROM collaboration_command AS c
    JOIN collaboration_session AS s ON s.id = c.session_id
    WHERE c.author_user_id = sqlc.arg(user_id)::uuid
      AND c.author_auth_epoch < sqlc.arg(account_epoch)::bigint
      AND c.state = 'waiting'
    UNION
    SELECT s.issue_id FROM collaboration_command AS c
    JOIN collaboration_session AS s ON s.id = c.session_id
    WHERE c.author_user_id = sqlc.arg(user_id)::uuid
      AND c.author_auth_epoch < sqlc.arg(account_epoch)::bigint
      AND c.state = 'in_flight'
      AND (EXISTS (SELECT 1 FROM collaboration_execution_group g
          WHERE g.session_id=c.session_id AND g.batch_id=c.batch_id AND g.settled_at IS NULL)
          OR EXISTS (SELECT 1 FROM collaboration_run r
          WHERE r.session_id=c.session_id AND r.batch_id=c.batch_id AND r.settled_at IS NULL))
    UNION
    SELECT g.issue_id FROM collaboration_grant AS g
    WHERE g.owner_user_id = sqlc.arg(user_id)::uuid
      AND g.owner_auth_epoch < sqlc.arg(account_epoch)::bigint
    UNION
    SELECT s.issue_id FROM collaboration_run AS r
    JOIN collaboration_session AS s ON s.id = r.session_id
    JOIN collaboration_command AS c ON c.id = r.command_id AND c.session_id = r.session_id
    WHERE r.settled_at IS NULL
      AND c.author_user_id = sqlc.arg(user_id)::uuid
      AND c.author_auth_epoch < sqlc.arg(account_epoch)::bigint
    UNION
    SELECT s.issue_id FROM collaboration_execution_group AS g
    JOIN collaboration_session AS s ON s.id = g.session_id
    JOIN collaboration_command AS c ON c.id = g.command_id AND c.session_id = g.session_id
    WHERE g.settled_at IS NULL
      AND c.author_user_id = sqlc.arg(user_id)::uuid
      AND c.author_auth_epoch < sqlc.arg(account_epoch)::bigint
) AS affected
LEFT JOIN issue AS i ON i.id = affected.issue_id
WHERE sqlc.narg(last_issue_id)::uuid IS NULL OR affected.issue_id > sqlc.narg(last_issue_id)::uuid
ORDER BY affected.issue_id
LIMIT 1;

-- Source locks follow workspace -> issue -> collaboration -> session. A busy
-- source rolls back the intent transaction; it never advances past that issue.
-- name: LockCollaborationReconciliationWorkspace :one
SELECT id FROM workspace WHERE id = $1 FOR KEY SHARE NOWAIT;

-- name: LockCollaborationReconciliationIssue :one
SELECT * FROM issue WHERE id = $1 FOR NO KEY UPDATE NOWAIT;

-- name: LockCollaborationReconciliationState :one
SELECT * FROM issue_collaboration WHERE issue_id = $1 FOR UPDATE NOWAIT;

-- name: LockCollaborationReconciliationSession :one
SELECT * FROM collaboration_session WHERE issue_id = $1 FOR UPDATE NOWAIT;

-- The source/session locks serialize this with all control/authority writers.
-- Epoch comparisons must use the persisted intent, never the current account
-- epoch or disabled flag: an older intent must preserve freshly authorized work.
-- name: CancelObsoleteCollaborationAuthorCommands :many
UPDATE collaboration_command SET state = 'cancelled', reason = sqlc.arg(reason)::text
WHERE session_id = sqlc.arg(session_id)::uuid
  AND author_user_id = sqlc.arg(user_id)::uuid
  AND author_auth_epoch < sqlc.arg(account_epoch)::bigint
  AND state = 'waiting' AND group_id IS NULL
RETURNING *;

-- name: RevokeObsoleteCollaborationOwnerGrants :execrows
UPDATE collaboration_grant SET epoch = epoch + 1, revoked_at = now(), revocation_reason = sqlc.arg(reason)::text
WHERE issue_id = sqlc.arg(issue_id)::uuid
  AND owner_user_id = sqlc.arg(user_id)::uuid
  AND owner_auth_epoch < sqlc.arg(account_epoch)::bigint
  AND revoked_at IS NULL;

-- name: AdvanceCollaborationAccountInvalidation :execrows
UPDATE collaboration_account_invalidation
SET last_issue_id = sqlc.arg(issue_id)::uuid, last_attempted_at = clock_timestamp()
WHERE user_id = sqlc.arg(user_id)::uuid AND account_epoch = sqlc.arg(account_epoch)::bigint
  AND reconciled_at IS NULL
  AND (last_issue_id IS NULL OR last_issue_id < sqlc.arg(issue_id)::uuid);

-- name: CompleteCollaborationAccountInvalidation :execrows
UPDATE collaboration_account_invalidation SET reconciled_at = now(), last_attempted_at = clock_timestamp()
WHERE user_id = sqlc.arg(user_id)::uuid AND account_epoch = sqlc.arg(account_epoch)::bigint
  AND reconciled_at IS NULL;
