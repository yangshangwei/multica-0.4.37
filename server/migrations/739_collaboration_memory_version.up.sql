-- Shared context ("memory") for one collaboration issue. Every write or restore
-- appends an immutable version; a missing row set is the empty version 0. The
-- deployment byte limit is enforced by the application, the CHECK is the hard
-- ceiling. Indexes are built concurrently by the following migrations.
BEGIN;

CREATE TABLE IF NOT EXISTS collaboration_memory_version (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    issue_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version >= 1),
    content text NOT NULL CHECK (octet_length(content) <= 524288),
    bytes integer NOT NULL CHECK (bytes BETWEEN 0 AND 524288),
    author_kind text NOT NULL CHECK (author_kind IN ('member', 'agent')),
    author_user_id uuid,
    agent_id uuid,
    run_id uuid,
    resource_owner_user_id uuid,
    request_id text CHECK (request_id IS NULL OR length(request_id) BETWEEN 1 AND 128),
    request_hash text CHECK (request_hash IS NULL OR request_hash ~ '^[a-f0-9]{64}$'),
    restored_from_version bigint CHECK (restored_from_version IS NULL OR restored_from_version >= 1),
    purged_at timestamptz,
    purged_by_user_id uuid,
    purged_by_admin boolean NOT NULL DEFAULT false,
    purge_reason text CHECK (purge_reason IS NULL OR length(purge_reason) BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (restored_from_version IS NULL OR restored_from_version < version),
    CHECK (
        (author_kind = 'member' AND author_user_id IS NOT NULL AND agent_id IS NULL AND run_id IS NULL
            AND resource_owner_user_id IS NULL AND request_id IS NULL AND request_hash IS NULL)
        OR (author_kind = 'agent' AND author_user_id IS NULL AND agent_id IS NOT NULL AND run_id IS NOT NULL
            AND resource_owner_user_id IS NOT NULL AND request_id IS NOT NULL AND request_hash IS NOT NULL AND restored_from_version IS NULL)
    ),
    CHECK (
        (purged_at IS NULL AND purged_by_user_id IS NULL AND purge_reason IS NULL AND NOT purged_by_admin
            AND bytes = octet_length(content))
        OR (purged_at IS NOT NULL AND purged_by_user_id IS NOT NULL AND purge_reason IS NOT NULL AND content = '')
    )
);

-- Versions are append-only. The single permitted update clears one version's
-- text and records who purged it; byte count and authorship stay as history.
CREATE OR REPLACE FUNCTION collaboration_preserve_memory_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.id, NEW.issue_id, NEW.version, NEW.bytes, NEW.author_kind, NEW.author_user_id, NEW.agent_id, NEW.run_id,
            NEW.resource_owner_user_id, NEW.request_id, NEW.request_hash, NEW.restored_from_version, NEW.created_at)
        IS DISTINCT FROM ROW(OLD.id, OLD.issue_id, OLD.version, OLD.bytes, OLD.author_kind, OLD.author_user_id, OLD.agent_id, OLD.run_id,
            OLD.resource_owner_user_id, OLD.request_id, OLD.request_hash, OLD.restored_from_version, OLD.created_at)
        OR OLD.purged_at IS NOT NULL
        OR (NEW.purged_at IS NULL AND NEW.content IS DISTINCT FROM OLD.content) THEN
        RAISE EXCEPTION 'collaboration memory versions are immutable except for one purge' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_memory_version_identity ON collaboration_memory_version;
CREATE TRIGGER collaboration_memory_version_identity BEFORE UPDATE ON collaboration_memory_version
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_memory_version();

COMMIT;
