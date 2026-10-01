CREATE TABLE managed_installation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    deployment_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    public_key bytea NOT NULL CHECK (octet_length(public_key) = 32),
    key_fingerprint text NOT NULL CHECK (length(key_fingerprint) = 64),
    key_version bigint NOT NULL DEFAULT 1 CHECK (key_version > 0),
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'retired')),
    responsible_user_id uuid,
    display_name text NOT NULL DEFAULT '',
    groups text[] NOT NULL DEFAULT '{}',
    desktop_version text,
    os text,
    client_seen_at timestamptz,
    admission text NOT NULL DEFAULT 'accepting' CHECK (admission IN ('accepting', 'stopped')),
    admission_version bigint NOT NULL DEFAULT 1 CHECK (admission_version > 0),
    replaced_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE installation_user (
    installation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE installation_daemon_binding (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    installation_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    daemon_id text NOT NULL,
    principal_user_id uuid NOT NULL,
    auth_version bigint NOT NULL CHECK (auth_version > 0),
    binding_epoch bigint NOT NULL CHECK (binding_epoch > 0),
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'revoked')),
    capability_version text NOT NULL DEFAULT '1',
    authenticated_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE installation_challenge (
    id uuid NOT NULL,
    purpose text NOT NULL CHECK (purpose IN ('enroll', 'bind', 'renew')),
    deployment_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    auth_version bigint NOT NULL CHECK (auth_version > 0),
    installation_id uuid,
    workspace_id uuid,
    daemon_id text,
    public_key bytea NOT NULL CHECK (octet_length(public_key) = 32),
    key_fingerprint text NOT NULL,
    expected_binding_epoch bigint,
    nonce_hash text NOT NULL,
    payload_hash text NOT NULL,
    body_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    result_id uuid,
    result_credential bytea,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE installation_report_cursor (
    installation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    auth_version bigint NOT NULL,
    boot_id uuid NOT NULL,
    sequence bigint NOT NULL CHECK (sequence > 0),
    reported_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE daemon_token ADD COLUMN installation_binding_id uuid, ADD COLUMN installation_binding_epoch bigint;
