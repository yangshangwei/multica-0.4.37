-- name: CreateInstallationChallenge :one
INSERT INTO installation_challenge (id,purpose,deployment_id,organization_id,user_id,auth_version,
 installation_id,workspace_id,daemon_id,public_key,key_fingerprint,expected_binding_epoch,
 nonce_hash,payload_hash,body_hash,expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING *;

-- name: LockInstallationChallenge :one
SELECT * FROM installation_challenge WHERE id=$1 FOR UPDATE;

-- name: GetInstallationChallenge :one
SELECT * FROM installation_challenge WHERE id=$1;

-- name: ConsumeInstallationChallenge :exec
UPDATE installation_challenge SET consumed_at=now(),result_id=$2,result_credential=$3 WHERE id=$1 AND consumed_at IS NULL;

-- name: LockInstallationNamespace :exec
SELECT pg_advisory_xact_lock(hashtextextended($1::text, 81271465));

-- name: GetManagedInstallation :one
SELECT * FROM managed_installation WHERE id=$1;

-- name: LockManagedInstallation :one
SELECT * FROM managed_installation WHERE id=$1 FOR UPDATE;

-- name: GetManagedInstallationByKey :one
SELECT * FROM managed_installation WHERE organization_id=$1 AND key_fingerprint=$2;

-- name: CreateManagedInstallation :one
INSERT INTO managed_installation (deployment_id,organization_id,public_key,key_fingerprint,responsible_user_id,desktop_version,os)
VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING *;

-- name: TouchInstallationUser :exec
INSERT INTO installation_user (installation_id,user_id) VALUES ($1,$2)
ON CONFLICT (installation_id,user_id) DO UPDATE SET last_seen_at=now();

-- name: GetActiveInstallationBinding :one
SELECT * FROM installation_daemon_binding WHERE workspace_id=$1 AND daemon_id=$2 AND state='active';

-- name: GetInstallationBinding :one
SELECT * FROM installation_daemon_binding WHERE id=$1;

-- name: GetLatestInstallationBinding :one
SELECT * FROM installation_daemon_binding WHERE workspace_id=$1 AND daemon_id=$2 ORDER BY binding_epoch DESC,created_at DESC,id DESC LIMIT 1;

-- name: CreateInstallationBinding :one
INSERT INTO installation_daemon_binding (installation_id,workspace_id,daemon_id,principal_user_id,auth_version,binding_epoch)
VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;

-- name: RevokeInstallationBinding :exec
UPDATE installation_daemon_binding SET state='revoked',revoked_at=now() WHERE id=$1;

-- name: TouchInstallationBinding :exec
UPDATE installation_daemon_binding SET last_seen_at=now() WHERE id=$1 AND state='active';

-- name: CountInstallationDaemonRuntimes :one
SELECT count(*) FROM agent_runtime WHERE workspace_id=$1 AND daemon_id=$2;

-- name: GetInstallationWorkspaceOrganization :one
SELECT organization_id FROM organization_workspace WHERE workspace_id=$1;

-- name: CreateManagedDaemonToken :one
INSERT INTO daemon_token (token_hash,workspace_id,daemon_id,expires_at,user_id,auth_version,installation_binding_id,installation_binding_epoch)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- name: DeleteInstallationBindingTokens :exec
DELETE FROM daemon_token WHERE installation_binding_id=$1;

-- name: AdvanceInstallationReport :one
INSERT INTO installation_report_cursor (installation_id,user_id,auth_version,boot_id,sequence,reported_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (installation_id,user_id,boot_id) DO UPDATE
SET sequence=EXCLUDED.sequence,auth_version=EXCLUDED.auth_version,reported_at=EXCLUDED.reported_at,received_at=now()
WHERE installation_report_cursor.auth_version < EXCLUDED.auth_version
   OR (installation_report_cursor.sequence < EXCLUDED.sequence AND installation_report_cursor.auth_version=EXCLUDED.auth_version)
RETURNING *;

-- name: ListOwnInstallationBindingHints :many
SELECT DISTINCT ON (b.workspace_id,b.daemon_id) b.id,b.workspace_id,b.daemon_id,b.binding_epoch,b.auth_version
FROM installation_daemon_binding b
JOIN member m ON m.workspace_id=b.workspace_id AND m.user_id=b.principal_user_id
WHERE b.installation_id=$1 AND b.principal_user_id=$2
ORDER BY b.workspace_id,b.daemon_id,b.binding_epoch DESC LIMIT 101;

-- name: GetOwnInstallationBindingHint :one
SELECT b.id,b.workspace_id,b.daemon_id,b.binding_epoch,b.auth_version
FROM installation_daemon_binding b
JOIN member m ON m.workspace_id=b.workspace_id AND m.user_id=b.principal_user_id
WHERE b.installation_id=$1 AND b.principal_user_id=$2 AND b.workspace_id=$3 AND b.daemon_id=$4
ORDER BY b.binding_epoch DESC,b.id DESC LIMIT 1;

-- name: UpdateInstallationClientReport :exec
UPDATE managed_installation SET client_seen_at=now(),desktop_version=$2,os=$3,updated_at=now() WHERE id=$1 AND lifecycle='active';

-- name: RevokeWorkspaceInstallationBindings :exec
UPDATE installation_daemon_binding SET state='revoked',revoked_at=now() WHERE workspace_id=$1 AND state='active';

-- name: ExpireWorkspaceInstallationChallenges :exec
UPDATE installation_challenge SET expires_at=LEAST(expires_at,now()) WHERE workspace_id=$1;
