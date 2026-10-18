-- Source issue/session/channel/target locks serialize proposal, consent and
-- final Switch mutations. Reads borrow an authorized repeatable-read snapshot.
-- name: CreateCollaborationSwitchProposal :one
INSERT INTO collaboration_switch_proposal(issue_id, session_id, target_id, proposed_by,
    lifecycle_epoch, base_target_epoch, base_config_version, roster, configuration_fingerprint)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;

-- name: GetCollaborationSwitchProposal :one
SELECT * FROM collaboration_switch_proposal WHERE id=$1 AND issue_id=$2 AND target_id=$3;

-- name: ListCollaborationSwitchProposals :many
SELECT * FROM collaboration_switch_proposal WHERE issue_id=$1 AND target_id=$2 ORDER BY created_at, id;

-- name: CompleteCollaborationSwitchProposal :one
UPDATE collaboration_switch_proposal SET committed_at=now()
WHERE id=$1 AND committed_at IS NULL RETURNING *;

-- name: ListCollaborationSwitchConsents :many
SELECT * FROM collaboration_switch_consent WHERE proposal_id=$1 ORDER BY agent_id;

-- name: CreateCollaborationSwitchConsent :one
INSERT INTO collaboration_switch_consent(proposal_id, agent_id, owner_user_id,
    owner_auth_epoch, owner_participant_epoch, identity_kind, installation_id, binding_id, binding_epoch)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;

-- name: RevokeCollaborationSwitchConsent :one
UPDATE collaboration_switch_consent SET revoked_at=COALESCE(revoked_at, now())
WHERE proposal_id=$1 AND agent_id=$2 RETURNING *;

-- name: ListCollaborationSwitchGroupDelegations :many
SELECT * FROM collaboration_delegation WHERE group_id=$1 ORDER BY id;
