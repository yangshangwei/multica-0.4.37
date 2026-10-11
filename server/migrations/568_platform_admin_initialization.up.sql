CREATE TABLE platform_admin_initialization (
    organization_id uuid NOT NULL,
    deployment_id uuid,
    initialization_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('created', 'preserved_existing_admin')),
    user_id uuid,
    initial_credential_version bigint,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((state = 'created' AND deployment_id IS NOT NULL AND user_id IS NOT NULL
        AND initial_credential_version IS NOT NULL AND initial_credential_version > 0)
        OR (state = 'preserved_existing_admin' AND user_id IS NULL AND initial_credential_version IS NULL))
);
