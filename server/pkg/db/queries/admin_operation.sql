-- name: GetAdminCancellationTaskForUpdate :one
SELECT t.* FROM agent_task_queue t
WHERE t.id = sqlc.arg('task_id')::uuid
AND EXISTS (SELECT 1 FROM agent a JOIN organization_workspace ow ON ow.workspace_id=a.workspace_id
 WHERE a.id=t.agent_id AND ow.organization_id=sqlc.arg('organization_id')::uuid)
FOR UPDATE OF t;

-- name: GetCancellationTaskForUpdate :one
SELECT * FROM agent_task_queue WHERE id=$1 FOR UPDATE;

-- name: SetAdminCancellationOperation :one
UPDATE admin_operation SET target_task_id=sqlc.arg('task_id')::uuid,
 target_installation_id=sqlc.narg('installation_id')::uuid,
 binding_id=sqlc.narg('binding_id')::uuid, binding_epoch=sqlc.narg('binding_epoch')::bigint,
 execution_runtime_id=sqlc.narg('runtime_id')::uuid,
 execution_dispatched_at=sqlc.narg('dispatched_at')::timestamptz,
 execution_fence=sqlc.arg('execution_fence')::bigint,
 root_operation_id=sqlc.narg('root_operation_id')::uuid,
 state=sqlc.arg('state')::text, result_code=sqlc.arg('result_code')::text,
 confirmation=sqlc.arg('confirmation')::text,
 reconciliation_state=sqlc.arg('reconciliation_state')::text,
 applied_at=sqlc.arg('applied_at')::timestamptz,
 confirmed_at=sqlc.narg('confirmed_at')::timestamptz,
 ack_deadline=sqlc.narg('ack_deadline')::timestamptz,
 next_reconcile_at=sqlc.narg('next_reconcile_at')::timestamptz,
 effects_completed_at=sqlc.narg('effects_completed_at')::timestamptz,
 version=version+1, updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND organization_id=sqlc.arg('organization_id')::uuid
 AND kind='task.cancel' AND target_kind='task'
RETURNING *;

-- name: FindAdminCancellationRoot :one
SELECT * FROM admin_operation
WHERE organization_id=sqlc.arg('organization_id')::uuid AND kind='task.cancel' AND state<>'failed'
 AND target_task_id=sqlc.arg('task_id')::uuid AND root_operation_id IS NULL
 AND execution_runtime_id IS NOT DISTINCT FROM sqlc.narg('runtime_id')::uuid
 AND execution_dispatched_at IS NOT DISTINCT FROM sqlc.narg('dispatched_at')::timestamptz
ORDER BY accepted_at,id LIMIT 1;

-- name: GetAdminCancellationRootForUpdate :one
SELECT * FROM admin_operation
WHERE id=$1 AND kind='task.cancel' AND root_operation_id IS NULL AND state<>'failed'
FOR UPDATE;

-- name: GetAdminOperationRoot :one
SELECT root.* FROM admin_operation root JOIN admin_operation follower ON follower.root_operation_id=root.id
WHERE follower.id=$1 AND root.root_operation_id IS NULL
 AND root.organization_id=follower.organization_id AND root.kind=follower.kind
 AND root.kind='task.cancel' AND root.target_kind=follower.target_kind AND root.target_id=follower.target_id
 AND root.target_task_id=follower.target_task_id
 AND root.execution_fence=follower.execution_fence
 AND root.execution_runtime_id IS NOT DISTINCT FROM follower.execution_runtime_id
 AND root.execution_dispatched_at IS NOT DISTINCT FROM follower.execution_dispatched_at
 AND root.binding_id IS NOT DISTINCT FROM follower.binding_id
 AND root.binding_epoch IS NOT DISTINCT FROM follower.binding_epoch;

-- name: ConfirmAdminCancellationRoot :one
UPDATE admin_operation SET state='succeeded', result_code='daemon_stopped',
 confirmation='confirmed', reconciliation_state='complete', confirmed_at=sqlc.arg('confirmed_at')::timestamptz,
 next_reconcile_at=sqlc.arg('confirmed_at')::timestamptz, version=version+1, updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND version=sqlc.arg('expected_version')::bigint
 AND kind='task.cancel' AND root_operation_id IS NULL AND state='applied'
RETURNING *;

-- name: RecordAdminCancellationPhase :exec
INSERT INTO admin_audit_event (operation_id,organization_id,actor_kind,actor_user_id,
 target_kind,target_id,action,phase,request_id,reason,before_state,after_state,result_code)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (operation_id,phase) WHERE operation_id IS NOT NULL DO NOTHING;

-- name: LeaseDueAdminCancellations :many
WITH due AS (
 SELECT id FROM admin_operation WHERE kind='task.cancel' AND root_operation_id IS NULL
  AND next_reconcile_at <= sqlc.arg('as_of')::timestamptz
 ORDER BY next_reconcile_at,id LIMIT sqlc.arg('batch_limit')::int FOR UPDATE SKIP LOCKED
)
UPDATE admin_operation op SET next_reconcile_at=sqlc.arg('lease_until')::timestamptz,
 version=op.version+1,updated_at=now()
FROM due WHERE op.id=due.id RETURNING op.*;

-- name: MarkAdminCancellationEffectsComplete :exec
UPDATE admin_operation SET effects_completed_at=sqlc.arg('completed_at')::timestamptz,
 version=version+1,updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND kind='task.cancel' AND root_operation_id IS NULL AND effects_completed_at IS NULL;

-- name: MarkAdminCancellationUnconfirmed :one
UPDATE admin_operation SET confirmation='unconfirmed',reconciliation_state='unconfirmed',
 version=version+1,updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND kind='task.cancel' AND root_operation_id IS NULL
 AND state='applied' AND confirmation='pending' AND ack_deadline<=sqlc.arg('as_of')::timestamptz
RETURNING *;

-- name: ListAdminCancellationFollowers :many
SELECT follower.* FROM admin_operation follower JOIN admin_operation root ON root.id=follower.root_operation_id
WHERE root.id=sqlc.arg('root_id')::uuid AND root.root_operation_id IS NULL
 AND follower.organization_id=root.organization_id AND follower.kind=root.kind
 AND follower.target_task_id=root.target_task_id AND follower.execution_fence=root.execution_fence
 AND (follower.state,follower.result_code,follower.confirmation,follower.reconciliation_state,follower.confirmed_at)
 IS DISTINCT FROM (root.state,root.result_code,root.confirmation,root.reconciliation_state,root.confirmed_at)
ORDER BY follower.id LIMIT sqlc.arg('batch_limit')::int FOR UPDATE OF follower SKIP LOCKED;

-- name: SyncAdminCancellationFollower :one
UPDATE admin_operation follower SET state=root.state,result_code=root.result_code,
 confirmation=root.confirmation,reconciliation_state=root.reconciliation_state,
 confirmed_at=root.confirmed_at,version=follower.version+1,updated_at=now()
FROM admin_operation root WHERE follower.id=sqlc.arg('id')::uuid
 AND follower.version=sqlc.arg('expected_version')::bigint
 AND follower.root_operation_id=root.id AND root.root_operation_id IS NULL
 AND follower.organization_id=root.organization_id AND follower.kind=root.kind
 AND follower.target_task_id=root.target_task_id AND follower.execution_fence=root.execution_fence
RETURNING follower.*;

-- name: ScheduleAdminCancellationReconcile :exec
UPDATE admin_operation SET next_reconcile_at=sqlc.narg('next_reconcile_at')::timestamptz,updated_at=now()
WHERE id=sqlc.arg('id')::uuid AND kind='task.cancel' AND root_operation_id IS NULL;

-- name: HasUnreconciledAdminCancellationFollowers :one
-- A skipped row may still need repair. Never infer completeness from a short
-- SKIP LOCKED batch, because another transaction can temporarily hold it.
SELECT EXISTS (
 SELECT 1 FROM admin_operation follower JOIN admin_operation root ON root.id=follower.root_operation_id
 WHERE root.id=$1 AND root.root_operation_id IS NULL
  AND follower.organization_id=root.organization_id AND follower.kind=root.kind
  AND follower.target_task_id=root.target_task_id AND follower.execution_fence=root.execution_fence
  AND (follower.state,follower.result_code,follower.confirmation,follower.reconciliation_state,follower.confirmed_at)
  IS DISTINCT FROM (root.state,root.result_code,root.confirmation,root.reconciliation_state,root.confirmed_at)
);

-- name: GetTaskCancellationRoot :one
SELECT op.* FROM admin_operation op JOIN agent_task_queue task ON task.id=op.target_task_id
WHERE task.id=$1 AND task.status='cancelled' AND op.kind='task.cancel' AND op.root_operation_id IS NULL
 AND op.execution_runtime_id IS NOT DISTINCT FROM task.runtime_id
 AND op.execution_dispatched_at IS NOT DISTINCT FROM task.dispatched_at
 AND op.binding_id IS NOT DISTINCT FROM task.execution_binding_id
 AND op.binding_epoch IS NOT DISTINCT FROM task.execution_binding_epoch
 AND op.result_code IN ('awaiting_daemon_confirmation','daemon_stopped')
ORDER BY op.accepted_at,op.id LIMIT 1;
