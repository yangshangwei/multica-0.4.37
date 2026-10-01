ALTER TABLE "user" ADD COLUMN disabled_at timestamptz, ADD COLUMN disabled_reason text;

CREATE TABLE organization (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(name) > 0),
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'inactive')),
    internal boolean NOT NULL DEFAULT true CHECK (internal),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organization_workspace (
    organization_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE platform_role_binding (
    user_id uuid NOT NULL,
    role text NOT NULL CHECK (role IN ('super_admin', 'platform_observer')),
    granted_by uuid,
    granted_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_operation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('user', 'deployment_operator')),
    actor_id uuid,
    actor_auth_version bigint NOT NULL DEFAULT 0 CHECK (actor_auth_version >= 0),
    target_kind text NOT NULL,
    target_id uuid NOT NULL,
    kind text NOT NULL,
    idempotency_key uuid NOT NULL,
    payload_hash text NOT NULL,
    reason text NOT NULL CHECK (length(reason) BETWEEN 1 AND 1000),
    state text NOT NULL DEFAULT 'accepted',
    result_code text NOT NULL DEFAULT 'accepted',
    version bigint NOT NULL DEFAULT 1 CHECK (version >= 0),
    confirmation text NOT NULL DEFAULT 'not_required',
    reconciliation_state text NOT NULL DEFAULT 'complete',
    target_installation_id uuid,
    target_task_id uuid,
    binding_id uuid,
    binding_epoch bigint,
    execution_fence bigint,
    root_operation_id uuid,
    ack_deadline timestamptz,
    accepted_at timestamptz NOT NULL DEFAULT now(),
    applied_at timestamptz,
    confirmed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((actor_kind = 'user' AND actor_id IS NOT NULL AND actor_auth_version > 0) OR (actor_kind = 'deployment_operator' AND actor_id IS NULL))
);

CREATE TABLE admin_audit_event (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    operation_id uuid,
    organization_id uuid NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('user', 'deployment_operator')),
    actor_user_id uuid,
    target_kind text NOT NULL,
    target_id uuid NOT NULL,
    action text NOT NULL,
    phase text NOT NULL,
    request_id text NOT NULL,
    reason text NOT NULL CHECK (length(reason) BETWEEN 1 AND 1000),
    before_state jsonb NOT NULL DEFAULT '{}',
    after_state jsonb NOT NULL DEFAULT '{}',
    result_code text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (pg_column_size(before_state) <= 8192 AND pg_column_size(after_state) <= 8192)
);
