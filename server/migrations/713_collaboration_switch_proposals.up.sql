-- Staged consent is deliberately separate from live target grants. A proposal
-- becomes live only in the source transaction that completes the Switch.
CREATE TABLE IF NOT EXISTS collaboration_switch_proposal (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    session_id uuid NOT NULL,
    target_id uuid NOT NULL,
    proposed_by uuid NOT NULL,
    lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
    base_target_epoch bigint NOT NULL CHECK (base_target_epoch > 0),
    base_config_version bigint NOT NULL CHECK (base_config_version > 0),
    roster jsonb NOT NULL CHECK (jsonb_typeof(roster) = 'array'),
    configuration_fingerprint text NOT NULL CHECK (length(configuration_fingerprint) = 64),
    created_at timestamptz NOT NULL DEFAULT now(),
    committed_at timestamptz
);

CREATE TABLE IF NOT EXISTS collaboration_switch_consent (
    proposal_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    owner_auth_epoch bigint NOT NULL CHECK (owner_auth_epoch > 0),
    owner_participant_epoch bigint NOT NULL CHECK (owner_participant_epoch > 0),
    identity_kind text NOT NULL CHECK (identity_kind IN ('managed', 'enrolled')),
    installation_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    binding_epoch bigint NOT NULL CHECK (binding_epoch > 0),
    consented_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
