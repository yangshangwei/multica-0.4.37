-- An offer records a participant owner's explicit disclosure to this issue.
-- It neither creates a target nor authorizes execution. The live target and
-- adoption fact are written together by the existing queue transaction.
CREATE TABLE IF NOT EXISTS collaboration_resource_offer (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    offered_by uuid NOT NULL,
    lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
    kind text NOT NULL CHECK (kind IN ('agent', 'squad')),
    resource_id uuid NOT NULL,
    execution_workspace_id uuid NOT NULL,
    roster jsonb NOT NULL CHECK (jsonb_typeof(roster) = 'array' AND jsonb_array_length(roster) BETWEEN 1 AND 256),
    configuration_fingerprint text NOT NULL CHECK (length(configuration_fingerprint) = 64),
    resource_authority jsonb NOT NULL CHECK (jsonb_typeof(resource_authority) = 'array' AND jsonb_array_length(resource_authority) = jsonb_array_length(roster)),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    state text NOT NULL DEFAULT 'offered' CHECK (state IN ('offered', 'withdrawn', 'adopted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    withdrawn_at timestamptz,
    adopted_at timestamptz,
    adopted_by uuid,
    target_id uuid,
    CHECK (
        (state = 'offered' AND withdrawn_at IS NULL AND adopted_at IS NULL AND adopted_by IS NULL AND target_id IS NULL)
        OR (state = 'withdrawn' AND withdrawn_at IS NOT NULL AND adopted_at IS NULL AND adopted_by IS NULL AND target_id IS NULL)
        OR (state = 'adopted' AND withdrawn_at IS NULL AND adopted_at IS NOT NULL AND adopted_by IS NOT NULL AND target_id IS NOT NULL)
    )
);
