CREATE TABLE collaboration_account_invalidation (
    user_id uuid NOT NULL,
    account_epoch bigint NOT NULL CHECK (account_epoch > 1),
    revocation_key text NOT NULL CHECK (length(revocation_key) BETWEEN 1 AND 128),
    credential_version bigint CHECK (credential_version > 0),
    reason text NOT NULL CHECK (reason = 'credentials_revoked'),
    created_at timestamptz NOT NULL DEFAULT now(),
    reconciled_at timestamptz
);
