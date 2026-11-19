-- The joined open request in GetCollaborationAccess is a capability fact.
-- Context recovery must separately retain the real pointer, including dangling IDs.
-- name: GetCollaborationInformationPointer :one
SELECT active_information_request_id FROM issue_collaboration WHERE issue_id = $1;

-- name: CollaborationInformationSchemaReady :one
SELECT NOT EXISTS (
        SELECT 1 FROM (VALUES
            ('unblock_condition', 'text'),
            ('estimated_resolution_at', 'timestamp with time zone'),
            ('superseded_by_repair_handoff_id', 'uuid')
        ) AS want(col, typ) WHERE NOT EXISTS (
            SELECT 1 FROM pg_catalog.pg_attribute a
            WHERE a.attrelid = to_regclass('issue_handoff') AND a.attname = want.col
                AND NOT a.attnotnull AND NOT a.attisdropped
                AND format_type(a.atttypid, a.atttypmod) = want.typ
        )
    )::boolean;

-- Every read borrows the current authorized snapshot or workflow transaction.
-- name: ListCollaborationInformationEvents :many
SELECT * FROM collaboration_event
WHERE issue_id = sqlc.arg(issue_id)::uuid
    AND payload->>'handoff_id' = sqlc.arg(handoff_id)::uuid::text
    AND kind IN ('information_requested', 'information_returned', 'information_repaired')
ORDER BY event_seq LIMIT 2;

-- name: GetCollaborationInformationSubmissionEvent :one
SELECT * FROM collaboration_event WHERE issue_id = sqlc.arg(issue_id)::uuid
    AND event_seq = sqlc.arg(event_seq)::bigint AND kind = 'acceptance_submitted';

-- name: GetCollaborationInformationBasisEvent :one
SELECT * FROM collaboration_event WHERE issue_id = sqlc.arg(issue_id)::uuid
    AND id = sqlc.arg(event_id)::uuid AND kind IN ('information_requested', 'acceptance_submitted');

-- name: ListCollaborationInformationAuthorityFacts :many
SELECT u.id, u.disabled_at, p.epoch AS participant_epoch, p.revoked_at,
    f.epoch AS account_epoch
FROM issue_participant p JOIN "user" u ON u.id = p.user_id
LEFT JOIN collaboration_account_fence f ON f.user_id = p.user_id
WHERE p.issue_id = $1 ORDER BY u.id;

-- Repair is a supported transfer row only when its exact repair event links it.
-- Legacy information rows remain in the collection even without an event link.
-- name: ListCollaborationInformationHandoffs :many
SELECT h.* FROM issue_handoff h
WHERE h.issue_id = sqlc.arg(issue_id)::uuid
    AND (h.purpose IN ('information', 'information_return') OR
        (h.purpose = 'transfer' AND EXISTS (
            SELECT 1 FROM collaboration_event e WHERE e.issue_id = h.issue_id
                AND e.kind = 'information_repaired' AND e.payload->>'handoff_id' = h.id::text)))
    AND (sqlc.narg(upper_at)::timestamptz IS NULL OR
        (h.created_at, h.id) <= (sqlc.narg(upper_at)::timestamptz, sqlc.narg(upper_id)::uuid))
    AND (sqlc.narg(before_at)::timestamptz IS NULL OR
        (h.created_at, h.id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY h.created_at DESC, h.id DESC LIMIT sqlc.arg(row_limit)::int;

-- name: GetCollaborationInformationHandoff :one
SELECT h.* FROM issue_handoff h
WHERE h.issue_id = sqlc.arg(issue_id)::uuid AND h.id = sqlc.arg(handoff_id)::uuid
    AND (h.purpose IN ('information', 'information_return') OR
        (h.purpose = 'transfer' AND EXISTS (
            SELECT 1 FROM collaboration_event e WHERE e.issue_id = h.issue_id
                AND e.kind = 'information_repaired' AND e.payload->>'handoff_id' = h.id::text)));

-- name: CountCollaborationInformationOpenRepairLinks :one
SELECT count(*)::bigint FROM issue_handoff WHERE issue_id = $1
    AND purpose = 'information' AND resolution = 'open'
    AND superseded_by_repair_handoff_id IS NOT NULL;

-- The new repair relation is write-once. Previously returned/superseded rows
-- and a security removal's earlier supersession are never attributed to repair.
-- name: SupersedeCollaborationInformationForRepair :one
WITH affected AS (
    UPDATE issue_handoff SET resolution = 'superseded',
        superseded_by_repair_handoff_id = sqlc.arg(repair_handoff_id)::uuid
    WHERE issue_id = sqlc.arg(issue_id)::uuid AND purpose = 'information'
        AND resolution = 'open' AND superseded_by_repair_handoff_id IS NULL
    RETURNING id
) SELECT count(*)::bigint FROM affected;

-- name: CountCollaborationInformationRepairAffected :one
SELECT count(*)::bigint FROM issue_handoff WHERE issue_id = sqlc.arg(issue_id)::uuid
    AND superseded_by_repair_handoff_id = sqlc.arg(repair_handoff_id)::uuid;

-- Do not filter corrupted rows out of the denominator. The projector refuses
-- an invalid purpose/resolution rather than inventing a complete page.
-- name: ListCollaborationInformationRepairAffected :many
SELECT * FROM issue_handoff
WHERE issue_id = sqlc.arg(issue_id)::uuid
    AND superseded_by_repair_handoff_id = sqlc.arg(repair_handoff_id)::uuid
    AND (sqlc.narg(upper_at)::timestamptz IS NULL OR
        (created_at, id) <= (sqlc.narg(upper_at)::timestamptz, sqlc.narg(upper_id)::uuid))
    AND (sqlc.narg(before_at)::timestamptz IS NULL OR
        (created_at, id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(row_limit)::int;
