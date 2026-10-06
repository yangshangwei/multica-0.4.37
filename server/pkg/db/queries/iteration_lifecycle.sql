-- name: CreateIteration :one
INSERT INTO iteration (id,workspace_id,name,description,coordinator_user_id,timezone,
 start_date,end_date,created_by,created_at)
VALUES (sqlc.arg('id'),sqlc.arg('workspace_id'),sqlc.arg('name'),sqlc.narg('description'),
 sqlc.narg('coordinator_user_id'),sqlc.arg('timezone'),sqlc.arg('start_date'),
 sqlc.arg('end_date'),sqlc.arg('created_by'),sqlc.arg('business_at')) RETURNING *;

-- name: EditIteration :one
UPDATE iteration SET name=sqlc.arg('name'),description=sqlc.narg('description'),
 coordinator_user_id=sqlc.narg('coordinator_user_id'),start_date=sqlc.arg('start_date'),
 end_date=sqlc.arg('end_date'),revision=revision+1
WHERE workspace_id=sqlc.arg('workspace_id') AND id=sqlc.arg('id')
 AND revision=sqlc.arg('expected_revision') AND revision<9007199254740991
RETURNING *;

-- name: StartIteration :one
UPDATE iteration SET status='active',start_date=sqlc.arg('start_date'),
 end_date=sqlc.arg('end_date'),started_by=sqlc.arg('started_by'),
 started_at=GREATEST(sqlc.arg('business_at')::timestamptz,
  (SELECT e.occurred_at FROM iteration_event e WHERE e.workspace_id=sqlc.arg('workspace_id')
   AND e.iteration_id=sqlc.arg('id') ORDER BY e.sequence DESC LIMIT 1)),
 revision=revision+1,scope_revision=scope_revision+1
WHERE workspace_id=sqlc.arg('workspace_id') AND id=sqlc.arg('id')
 AND status='planned' AND started_at IS NULL
 AND revision<9007199254740991 AND scope_revision<9007199254740991 RETURNING *;

-- name: CancelPlannedIteration :one
UPDATE iteration SET status='cancelled',logical_ended_at=GREATEST(sqlc.arg('business_at')::timestamptz,
 (SELECT e.occurred_at FROM iteration_event e WHERE e.workspace_id=sqlc.arg('workspace_id')
 AND e.iteration_id=sqlc.arg('id') ORDER BY e.sequence DESC LIMIT 1)),
 processed_at=GREATEST(sqlc.arg('business_at')::timestamptz,
 (SELECT e.occurred_at FROM iteration_event e WHERE e.workspace_id=sqlc.arg('workspace_id')
 AND e.iteration_id=sqlc.arg('id') ORDER BY e.sequence DESC LIMIT 1)),end_reason=sqlc.narg('reason'),
 revision=revision+1,scope_revision=scope_revision+1
WHERE workspace_id=sqlc.arg('workspace_id') AND id=sqlc.arg('id')
 AND status='planned' AND started_at IS NULL
 AND revision<9007199254740991 AND scope_revision<9007199254740991 RETURNING *;

-- name: DeleteUnusedPlannedIteration :execrows
-- Caller holds the I1 fence and iteration lock. Metadata-only activity does
-- not prevent deletion; any historical membership does. Operation results
-- remain independent so an authorized actor can replay the deletion.
WITH eligible AS (
 SELECT i.id,i.workspace_id FROM iteration i
 WHERE i.workspace_id=sqlc.arg('workspace_id') AND i.id=sqlc.arg('id')
 AND i.status='planned' AND i.started_at IS NULL
 AND NOT EXISTS (SELECT 1 FROM iteration_participation p
  WHERE p.workspace_id=i.workspace_id AND p.iteration_id=i.id)
 AND NOT EXISTS (SELECT 1 FROM issue x
  WHERE x.workspace_id=i.workspace_id AND x.current_iteration_id=i.id)
 AND NOT EXISTS (SELECT 1 FROM iteration_event e
  WHERE e.workspace_id=i.workspace_id AND e.iteration_id=i.id AND e.issue_id IS NOT NULL)
), removed_metadata AS (
 DELETE FROM iteration_event e USING eligible i
 WHERE e.workspace_id=i.workspace_id AND e.iteration_id=i.id
)
DELETE FROM iteration i USING eligible d
WHERE i.workspace_id=d.workspace_id AND i.id=d.id;

-- name: GetIterationActive :one
SELECT * FROM iteration WHERE workspace_id=$1 AND status='active';

-- name: ListIterationOperationIssues :many
SELECT * FROM issue WHERE workspace_id=sqlc.arg('workspace_id')
 AND (current_iteration_id=ANY(sqlc.arg('iteration_ids')::uuid[])
 OR id=ANY(sqlc.arg('issue_ids')::uuid[])) ORDER BY id;

-- name: LockIterationOperationIssues :many
SELECT * FROM issue WHERE workspace_id=sqlc.arg('workspace_id')
 AND (current_iteration_id=ANY(sqlc.arg('iteration_ids')::uuid[])
 OR id=ANY(sqlc.arg('issue_ids')::uuid[])) ORDER BY id FOR UPDATE;

-- name: LockIterationReferenceMember :one
SELECT m.* FROM member m JOIN "user" u ON u.id=m.user_id
WHERE m.workspace_id=sqlc.arg('workspace_id') AND m.user_id=sqlc.arg('user_id')
FOR SHARE OF m,u NOWAIT;

-- name: HasIterationActiveJoin :one
SELECT EXISTS (SELECT 1 FROM iteration_event
 WHERE workspace_id=$1 AND iteration_id=$2 AND issue_id=$3
 AND kind IN ('join','reenter'));

-- name: JoinIterationParticipation :exec
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

-- name: CaptureIterationOriginal :execrows
UPDATE iteration_participation SET in_original=true,original_facts=sqlc.arg('original_facts'),
 has_started_current_participation=sqlc.arg('has_started')
WHERE workspace_id=sqlc.arg('workspace_id') AND iteration_id=sqlc.arg('iteration_id')
 AND issue_id=sqlc.arg('issue_id') AND NOT in_original AND original_facts IS NULL
 AND current_joined_at IS NOT NULL;

-- name: AppendIterationLifecycleEvent :exec
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
