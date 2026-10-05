-- Shared planning_timezone is owned by 536_project_p1_tables and reused by I1.
ALTER TABLE issue ADD COLUMN current_iteration_id uuid;
ALTER TABLE issue ADD COLUMN iteration_rollover_count integer NOT NULL DEFAULT 0 CHECK (iteration_rollover_count >= 0);

CREATE TABLE workspace_iteration_settings (
    workspace_id uuid NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision BETWEEN 1 AND 9007199254740991)
);

CREATE TABLE iteration (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    description text,
    coordinator_user_id uuid,
    timezone text NOT NULL,
    start_date date NOT NULL,
    end_date date NOT NULL CHECK (end_date >= start_date),
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','active','completed','cancelled')),
    mode text NOT NULL DEFAULT 'manual' CHECK (mode = 'manual'),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision BETWEEN 1 AND 9007199254740991),
    scope_revision bigint NOT NULL DEFAULT 1 CHECK (scope_revision BETWEEN 1 AND 9007199254740991),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    started_by uuid,
    started_at timestamptz,
    logical_ended_at timestamptz,
    processed_at timestamptz,
    end_reason text
);

CREATE TABLE iteration_participation (
    workspace_id uuid NOT NULL,
    iteration_id uuid NOT NULL,
    issue_id uuid NOT NULL,
    first_joined_at timestamptz NOT NULL,
    current_joined_at timestamptz,
    has_started_current_participation boolean NOT NULL DEFAULT false,
    last_left_at timestamptz,
    in_original boolean NOT NULL DEFAULT false,
    original_facts jsonb
);

CREATE TABLE iteration_event (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    iteration_id uuid NOT NULL,
    sequence bigint NOT NULL CHECK (sequence BETWEEN 1 AND 9007199254740991),
    operation_id uuid NOT NULL,
    issue_id uuid,
    kind text NOT NULL,
    actor jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    sampled_at timestamptz NOT NULL,
    before_facts jsonb NOT NULL DEFAULT 'null',
    after_facts jsonb NOT NULL DEFAULT 'null',
    reason text
);

CREATE TABLE iteration_snapshot (
    iteration_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    operation_id uuid NOT NULL,
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE iteration_operation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    actor_user_id uuid NOT NULL,
    request_id uuid NOT NULL,
    operation text NOT NULL,
    payload_hash text NOT NULL,
    result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE iteration_notification (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    iteration_id uuid,
    operation_id uuid NOT NULL,
    recipient_user_id uuid NOT NULL,
    kind text NOT NULL,
    local_date date,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','delivered','suppressed','dead_letter')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    last_error_code text
);
