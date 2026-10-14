-- Constraints that build indexes are installed concurrently in later files.
CREATE TABLE issue_collaboration (
    issue_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    authority_epoch bigint NOT NULL DEFAULT 1 CHECK (authority_epoch > 0),
    lifecycle_epoch bigint NOT NULL DEFAULT 1 CHECK (lifecycle_epoch > 0),
    active_information_request_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE issue_participant (
    issue_id uuid NOT NULL,
    user_id uuid NOT NULL,
    epoch bigint NOT NULL DEFAULT 1 CHECK (epoch > 0),
    joined_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    revocation_reason text
);

CREATE TABLE issue_handoff (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    actor_user_id uuid NOT NULL,
    from_user_id uuid NOT NULL,
    to_user_id uuid NOT NULL,
    purpose text NOT NULL CHECK (purpose IN ('transfer', 'information', 'information_return', 'acceptance', 'removal_return', 'admin_reopen')),
    note text NOT NULL DEFAULT '',
    requester_user_id uuid,
    parent_request_id uuid,
    previous_status text NOT NULL,
    resolution text NOT NULL DEFAULT 'open' CHECK (resolution IN ('open', 'returned', 'superseded')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_session (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    queue_state text NOT NULL DEFAULT 'paused' CHECK (queue_state IN ('paused', 'active', 'closed')),
    control_version bigint NOT NULL DEFAULT 1 CHECK (control_version > 0),
    event_seq bigint NOT NULL DEFAULT 0 CHECK (event_seq >= 0),
    next_command_seq bigint NOT NULL DEFAULT 1 CHECK (next_command_seq > 0),
    resource_epoch bigint NOT NULL DEFAULT 1 CHECK (resource_epoch > 0),
    active_grant_id uuid,
    active_run_id uuid,
    pause_reason text NOT NULL DEFAULT 'new_session',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_grant (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    owner_auth_epoch bigint NOT NULL CHECK (owner_auth_epoch > 0),
    execution_workspace_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    runtime_id uuid NOT NULL,
    identity_kind text NOT NULL CHECK (identity_kind IN ('managed', 'enrolled')),
    installation_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    binding_epoch bigint NOT NULL CHECK (binding_epoch > 0),
    epoch bigint NOT NULL DEFAULT 1 CHECK (epoch > 0),
    consented_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    revocation_reason text
);

CREATE TABLE collaboration_command (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    seq bigint NOT NULL CHECK (seq > 0),
    author_user_id uuid NOT NULL,
    participant_epoch bigint NOT NULL CHECK (participant_epoch > 0),
    author_auth_epoch bigint NOT NULL CHECK (author_auth_epoch > 0),
    lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
    content text NOT NULL,
    attachments jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(attachments) = 'array'),
    state text NOT NULL DEFAULT 'waiting' CHECK (state IN ('waiting', 'in_flight', 'succeeded', 'failed', 'stopped', 'cancelled')),
    reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_run (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    command_id uuid NOT NULL,
    task_id uuid,
    grant_id uuid NOT NULL,
    grant_epoch bigint NOT NULL CHECK (grant_epoch > 0),
    resource_epoch bigint NOT NULL CHECK (resource_epoch > 0),
    execution_workspace_id uuid NOT NULL,
    runtime_id uuid NOT NULL,
    installation_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    binding_epoch bigint NOT NULL CHECK (binding_epoch > 0),
    fence bigint NOT NULL DEFAULT 1 CHECK (fence > 0),
    claim_generation bigint NOT NULL DEFAULT 1 CHECK (claim_generation > 0),
    launch_nonce uuid NOT NULL DEFAULT gen_random_uuid(),
    progress_state text NOT NULL DEFAULT 'preparing',
    business_outcome text CHECK (business_outcome IN ('success', 'failure')),
    forced_disposition text CHECK (forced_disposition IN ('stopped', 'cancelled')),
    disposition_reason text,
    proof_state text NOT NULL DEFAULT 'unknown' CHECK (proof_state IN ('unknown', 'never_launched_proven', 'never_started_proven', 'exited_proven')),
    authorized_attempt_seq bigint NOT NULL DEFAULT 0 CHECK (authorized_attempt_seq >= 0),
    sealed_attempt_seq bigint CHECK (sealed_attempt_seq >= 0),
    recording_incomplete boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    settled_at timestamptz,
    CHECK (settled_at IS NULL OR proof_state <> 'unknown')
);

CREATE TABLE collaboration_run_event (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    run_id uuid NOT NULL,
    producer_event_id text NOT NULL,
    seq bigint NOT NULL CHECK (seq > 0),
    kind text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_operation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    scope_kind text NOT NULL CHECK (scope_kind IN ('create', 'issue')),
    scope_id uuid NOT NULL,
    actor_user_id uuid NOT NULL,
    request_id text NOT NULL CHECK (length(request_id) BETWEEN 1 AND 128),
    kind text NOT NULL,
    payload_hash text NOT NULL,
    result_code text,
    result jsonb,
    resolved_issue_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE collaboration_event (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    event_seq bigint NOT NULL CHECK (event_seq > 0),
    actor_user_id uuid,
    kind text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    outbox_state text NOT NULL DEFAULT 'pending' CHECK (outbox_state IN ('pending', 'delivered')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_notification (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    recipient_user_id uuid NOT NULL,
    issue_id uuid NOT NULL,
    event_seq bigint NOT NULL CHECK (event_seq > 0),
    kind text NOT NULL,
    is_mention boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at timestamptz,
    archived_at timestamptz
);

CREATE TABLE collaboration_attachment (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid,
    draft_id uuid,
    uploader_user_id uuid NOT NULL,
    storage_key text NOT NULL,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    name text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    CHECK (issue_id IS NOT NULL OR draft_id IS NOT NULL),
    CHECK ((issue_id IS NULL AND delivered_at IS NULL) OR (issue_id IS NOT NULL AND delivered_at IS NOT NULL))
);

CREATE TABLE collaboration_attachment_reference (
    attachment_id uuid NOT NULL,
    reference_kind text NOT NULL CHECK (reference_kind IN ('comment', 'command', 'run', 'issue')),
    reference_id uuid NOT NULL,
    reference_revision bigint NOT NULL CHECK (reference_revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    removed_at timestamptz
);

CREATE TABLE collaboration_termination (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    run_id uuid NOT NULL,
    fence bigint NOT NULL CHECK (fence > 0),
    launch_nonce uuid NOT NULL,
    deployment_id uuid NOT NULL,
    installation_id uuid NOT NULL,
    identity_kind text NOT NULL CHECK (identity_kind IN ('managed', 'enrolled')),
    public_key bytea NOT NULL,
    token_hash text NOT NULL,
    stop_requested_at timestamptz,
    revoked_at timestamptz,
    receipt_id text,
    receipt_digest text,
    proof jsonb,
    confirmed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_account_fence (
    user_id uuid NOT NULL,
    epoch bigint NOT NULL DEFAULT 1 CHECK (epoch > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_runtime_enrollment (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    deployment_id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    execution_workspace_id uuid NOT NULL,
    runtime_id uuid NOT NULL,
    installation_id uuid NOT NULL,
    public_key bytea NOT NULL,
    binding_epoch bigint NOT NULL DEFAULT 1 CHECK (binding_epoch > 0),
    capabilities jsonb NOT NULL DEFAULT '{}',
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_runtime_challenge (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    deployment_id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    execution_workspace_id uuid NOT NULL,
    runtime_id uuid NOT NULL,
    installation_id uuid NOT NULL,
    public_key bytea NOT NULL,
    payload_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_execution_gate (
    id smallint NOT NULL DEFAULT 1 CHECK (id = 1),
    enabled boolean NOT NULL DEFAULT false,
    creation_enabled boolean NOT NULL DEFAULT false,
    epoch bigint NOT NULL DEFAULT 1 CHECK (epoch > 0),
    freeze_operation_id uuid,
    freeze_progress uuid
);

ALTER TABLE comment ADD COLUMN deleted_at timestamptz;
