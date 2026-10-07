-- Pipeline equivalents of the canonical membership statements. The SQL
-- equivalence test guards semantic drift; only batch result transport differs.

-- name: AppendIterationLifecycleEventBatch :batchexec
INSERT INTO iteration_event (workspace_id,iteration_id,sequence,operation_id,issue_id,
 kind,actor,occurred_at,sampled_at,before_facts,after_facts,reason)
SELECT sqlc.arg('workspace_id'),sqlc.arg('iteration_id'),COALESCE(previous.sequence,0)+1,
 sqlc.arg('operation_id'),sqlc.narg('issue_id'),sqlc.arg('kind'),sqlc.arg('actor'),
 GREATEST(sqlc.arg('sampled_at')::timestamptz,previous.occurred_at),sqlc.arg('sampled_at'),
 sqlc.arg('before_facts'),sqlc.arg('after_facts'),sqlc.narg('reason')
FROM (SELECT 1) seed LEFT JOIN LATERAL (
 SELECT sequence,occurred_at FROM iteration_event
 WHERE workspace_id=sqlc.arg('workspace_id') AND iteration_id=sqlc.arg('iteration_id')
 ORDER BY sequence DESC LIMIT 1
) previous ON true;

-- name: LeaveDeletedIssueIterationParticipationBatch :batchone
-- The delete event was appended in this transaction; retain its clamped time.
UPDATE iteration_participation p SET current_joined_at=NULL,
 has_started_current_participation=false,
 last_left_at=GREATEST(sqlc.arg('business_at')::timestamptz,
  (SELECT e.occurred_at FROM iteration_event e
   WHERE e.workspace_id=sqlc.arg('workspace_id') AND e.iteration_id=sqlc.arg('iteration_id')
   ORDER BY e.sequence DESC LIMIT 1))
WHERE p.workspace_id=sqlc.arg('workspace_id') AND p.iteration_id=sqlc.arg('iteration_id')
 AND p.issue_id=sqlc.arg('issue_id') AND p.current_joined_at IS NOT NULL RETURNING issue_id;

-- name: AdvanceIterationScopeRevisionBatch :batchexec
UPDATE iteration SET scope_revision=scope_revision+1 WHERE workspace_id=$1 AND id=$2;

-- name: JoinIterationParticipationBatch :batchexec
INSERT INTO iteration_participation (workspace_id,iteration_id,issue_id,
 first_joined_at,current_joined_at,has_started_current_participation)
SELECT sqlc.arg('workspace_id'),sqlc.arg('iteration_id'),sqlc.arg('issue_id'),
 GREATEST(sqlc.arg('business_at')::timestamptz,last_event.occurred_at),
 GREATEST(sqlc.arg('business_at')::timestamptz,last_event.occurred_at),sqlc.arg('has_started')
FROM (SELECT 1) seed LEFT JOIN LATERAL (
 SELECT occurred_at FROM iteration_event
 WHERE workspace_id=sqlc.arg('workspace_id') AND iteration_id=sqlc.arg('iteration_id')
 ORDER BY sequence DESC LIMIT 1
) last_event ON true
ON CONFLICT (workspace_id,iteration_id,issue_id)
DO UPDATE SET current_joined_at=EXCLUDED.current_joined_at,
 has_started_current_participation=EXCLUDED.has_started_current_participation;

-- name: SetIssueCurrentIterationBatch :batchone
-- Caller holds the workspace iteration fence and the issue row lock. Admission,
-- target eligibility, participation and event writes belong to that transaction.
UPDATE issue SET current_iteration_id = sqlc.narg('current_iteration_id'),
    iteration_rollover_count = sqlc.arg('rollover_count'), revision = revision + 1,
    updated_at = sqlc.arg('business_at'), last_activity_at = sqlc.arg('business_at')
WHERE workspace_id = sqlc.arg('workspace_id') AND id = sqlc.arg('issue_id')
    AND revision = sqlc.arg('expected_revision')
    AND admission_status IN ('not_required', 'accepted')
RETURNING *;
