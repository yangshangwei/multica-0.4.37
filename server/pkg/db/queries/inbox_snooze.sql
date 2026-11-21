-- name: EnsureInboxSnooze :exec
INSERT INTO inbox_snooze (workspace_id, recipient_type, recipient_id, group_kind, group_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, recipient_type, recipient_id, group_kind, group_id) DO NOTHING;

-- name: LockInboxSnooze :one
SELECT * FROM inbox_snooze
WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND group_kind=$4 AND group_id=$5
FOR UPDATE;

-- name: GetInboxSnooze :one
SELECT * FROM inbox_snooze
WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND group_kind=$4 AND group_id=$5;

-- name: ListDueInboxSnoozes :many
SELECT * FROM inbox_snooze
WHERE snoozed_until IS NOT NULL AND snoozed_until<=now()
ORDER BY snoozed_until, workspace_id, recipient_id, group_id
LIMIT 100;

-- name: DeleteMemberInboxSnoozes :exec
WITH sources AS (
    DELETE FROM inbox_critical_source WHERE inbox_critical_source.workspace_id=$1 AND inbox_critical_source.recipient_type='member' AND inbox_critical_source.recipient_id=$2
)
DELETE FROM inbox_snooze WHERE inbox_snooze.workspace_id=$1 AND inbox_snooze.recipient_type='member' AND inbox_snooze.recipient_id=$2;
