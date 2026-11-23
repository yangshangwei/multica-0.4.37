-- Offer writers borrow sorted user/source/session/channel/target locks before
-- locking the issue-scoped offer. An organizer sees only explicitly offered
-- resources; the visibility predicate is applied before pagination.
-- name: TryLockCollaborationResourceOfferMember :one
SELECT * FROM member WHERE workspace_id=$1 AND user_id=$2 FOR SHARE NOWAIT;

-- name: CreateCollaborationResourceOffer :one
INSERT INTO collaboration_resource_offer(issue_id, offered_by, lifecycle_epoch,
    kind, resource_id, execution_workspace_id, roster, configuration_fingerprint, resource_authority)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;

-- name: LockCollaborationResourceOffer :one
SELECT * FROM collaboration_resource_offer
WHERE issue_id=$1 AND id=$2 FOR UPDATE;

-- name: GetVisibleCollaborationResourceOffer :one
SELECT * FROM collaboration_resource_offer
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = sqlc.arg(offer_id)::uuid
    AND (sqlc.arg(can_organize)::boolean OR offered_by = sqlc.arg(viewer_id)::uuid);

-- name: ListVisibleCollaborationResourceOffers :many
SELECT * FROM collaboration_resource_offer
WHERE issue_id = sqlc.arg(issue_id)::uuid
    AND (sqlc.arg(can_organize)::boolean OR offered_by = sqlc.arg(viewer_id)::uuid)
    AND (sqlc.narg(before_created_at)::timestamptz IS NULL OR
         (created_at, id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(row_limit)::int;

-- name: WithdrawCollaborationResourceOffer :one
UPDATE collaboration_resource_offer SET state='withdrawn', version=version+1, withdrawn_at=now()
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = sqlc.arg(offer_id)::uuid
    AND offered_by = sqlc.arg(offered_by)::uuid
    AND state='offered' AND version = sqlc.arg(expected_version)::bigint
RETURNING *;

-- name: AdoptCollaborationResourceOffer :one
UPDATE collaboration_resource_offer SET state='adopted', version=version+1,
    adopted_at=now(), adopted_by=sqlc.arg(adopted_by)::uuid, target_id=sqlc.arg(target_id)::uuid
WHERE issue_id = sqlc.arg(issue_id)::uuid AND id = sqlc.arg(offer_id)::uuid
    AND state='offered' AND version = sqlc.arg(expected_version)::bigint
RETURNING *;
