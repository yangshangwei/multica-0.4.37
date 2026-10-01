-- name: LockManagedInstallationForAdmissionRead :one
SELECT * FROM managed_installation WHERE id=$1 FOR SHARE;

-- name: LockTaskForClaimFinalization :one
SELECT * FROM agent_task_queue WHERE id=$1 FOR UPDATE;

-- name: UpdateManagedInstallationAdmission :one
UPDATE managed_installation SET admission=$3,admission_version=admission_version+1,updated_at=now()
WHERE id=$1 AND admission_version=$2 AND admission_version < 9223372036854775807
RETURNING *;

-- name: CompleteAdminAdmissionOperation :one
UPDATE admin_operation SET target_installation_id=target_id,state='succeeded',
 confirmation='not_required',reconciliation_state='complete',applied_at=now(),confirmed_at=now(),
 updated_at=now(),version=version+1
WHERE id=$1 AND kind='installation.admission' AND target_kind='installation'
RETURNING *;

-- name: ListInstallationAdmissionRuntimes :many
SELECT DISTINCT r.id,r.workspace_id FROM agent_runtime r
JOIN installation_daemon_binding b ON b.workspace_id=r.workspace_id AND b.daemon_id=r.daemon_id
 AND b.principal_user_id=r.owner_id AND b.state='active'
WHERE b.installation_id=$1;
