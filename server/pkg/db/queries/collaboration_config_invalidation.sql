-- A locked earlier intent still excludes its later resource epochs. A failed
-- resource cannot monopolize a bounded pass or overtake its own source cursor.
-- name: LockNextCollaborationConfigInvalidation :one
SELECT intent.* FROM collaboration_config_invalidation intent
WHERE intent.reconciled_at IS NULL
  AND intent.id <> ALL(COALESCE(sqlc.arg(excluded_intent_ids)::uuid[], '{}'))
  AND NOT EXISTS (
      SELECT 1 FROM collaboration_config_invalidation earlier
      WHERE earlier.resource_kind = intent.resource_kind AND earlier.resource_id = intent.resource_id
        AND (earlier.config_epoch, earlier.id) < (intent.config_epoch, intent.id)
        AND earlier.reconciled_at IS NULL
  )
ORDER BY COALESCE(intent.last_attempted_at, intent.created_at), intent.id
LIMIT 1 FOR UPDATE OF intent SKIP LOCKED;

-- After a confirmed rollback or uncertain commit, only fair scheduling changes.
-- A concurrent worker owns its own attempt; this statement never waits for it.
-- name: RecordCollaborationConfigInvalidationAttempt :exec
WITH pending AS (
    SELECT id FROM collaboration_config_invalidation WHERE id = sqlc.arg(intent_id)::uuid
      AND reconciled_at IS NULL FOR UPDATE SKIP LOCKED
)
UPDATE collaboration_config_invalidation intent SET last_attempted_at = clock_timestamp()
FROM pending WHERE intent.id = pending.id;

-- Discovery uses retained identities, so deleting the current resource or its
-- grant does not hide a possibly-live group. Fresh configurations are evaluated
-- under source/resource fences by the consumer and survive an older intent.
-- name: NextCollaborationConfigInvalidationIssue :one
SELECT affected.issue_id, i.workspace_id FROM (
    SELECT s.issue_id FROM collaboration_target t
    JOIN collaboration_session s ON s.id = t.session_id
    WHERE t.revoked_at IS NULL AND (
        (sqlc.arg(resource_kind)::text = 'squad' AND t.kind = 'squad' AND t.resource_id = sqlc.arg(resource_id)::uuid)
        OR EXISTS (SELECT 1 FROM jsonb_array_elements(t.roster) resource WHERE
            (sqlc.arg(resource_kind)::text = 'agent' AND resource->>'agent_id' = sqlc.arg(resource_id)::uuid::text)
            OR (sqlc.arg(resource_kind)::text = 'runtime' AND resource->>'runtime_id' = sqlc.arg(resource_id)::uuid::text))
    )
    UNION
    SELECT grant_row.issue_id FROM collaboration_grant grant_row
    LEFT JOIN collaboration_target t ON t.id = grant_row.target_id
    WHERE (sqlc.arg(resource_kind)::text = 'agent' AND grant_row.agent_id = sqlc.arg(resource_id)::uuid)
       OR (sqlc.arg(resource_kind)::text = 'runtime' AND grant_row.runtime_id = sqlc.arg(resource_id)::uuid)
       OR (sqlc.arg(resource_kind)::text = 'squad' AND t.kind = 'squad' AND t.resource_id = sqlc.arg(resource_id)::uuid)
    UNION
    SELECT s.issue_id FROM collaboration_execution_group g
    JOIN collaboration_session s ON s.id = g.session_id
    WHERE g.settled_at IS NULL AND (
        EXISTS (SELECT 1 FROM jsonb_array_elements(g.roster) resource WHERE
            (sqlc.arg(resource_kind)::text = 'agent' AND resource->>'agent_id' = sqlc.arg(resource_id)::uuid::text)
            OR (sqlc.arg(resource_kind)::text = 'runtime' AND resource->>'runtime_id' = sqlc.arg(resource_id)::uuid::text))
        OR (sqlc.arg(resource_kind)::text = 'squad' AND EXISTS (
            SELECT 1 FROM collaboration_target t WHERE t.kind = 'squad' AND t.resource_id = sqlc.arg(resource_id)::uuid
              AND (t.id = g.target_id OR EXISTS (SELECT 1 FROM collaboration_command c
                  WHERE c.batch_id = g.batch_id AND c.target_id = t.id AND c.state = 'in_flight'))))
    )
    UNION
    SELECT s.issue_id FROM collaboration_run r
    JOIN collaboration_session s ON s.id = r.session_id
    WHERE r.settled_at IS NULL AND (
        (sqlc.arg(resource_kind)::text = 'agent' AND r.agent_id = sqlc.arg(resource_id)::uuid)
        OR (sqlc.arg(resource_kind)::text = 'runtime' AND r.runtime_id = sqlc.arg(resource_id)::uuid)
        OR (sqlc.arg(resource_kind)::text = 'squad' AND EXISTS (
            SELECT 1 FROM collaboration_target t WHERE t.id = r.target_id AND t.kind = 'squad' AND t.resource_id = sqlc.arg(resource_id)::uuid))
    )
) affected
LEFT JOIN issue i ON i.id = affected.issue_id
WHERE sqlc.narg(last_issue_id)::uuid IS NULL OR affected.issue_id > sqlc.narg(last_issue_id)::uuid
ORDER BY affected.issue_id LIMIT 1;

-- Cursor and source hold/grant/stop effects always commit in one transaction.
-- name: AdvanceCollaborationConfigInvalidation :execrows
UPDATE collaboration_config_invalidation
SET last_issue_id = sqlc.arg(issue_id)::uuid, last_attempted_at = clock_timestamp()
WHERE id = sqlc.arg(intent_id)::uuid AND reconciled_at IS NULL
  AND (last_issue_id IS NULL OR last_issue_id < sqlc.arg(issue_id)::uuid);

-- name: CompleteCollaborationConfigInvalidation :execrows
UPDATE collaboration_config_invalidation SET reconciled_at = now(), last_attempted_at = clock_timestamp()
WHERE id = sqlc.arg(intent_id)::uuid AND reconciled_at IS NULL;
