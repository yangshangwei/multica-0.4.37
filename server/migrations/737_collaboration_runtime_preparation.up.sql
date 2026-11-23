BEGIN;

-- Ordinary registration metadata cannot replace this managed-only summary.
ALTER TABLE agent_runtime ADD COLUMN collaboration_capabilities jsonb
    CHECK (collaboration_capabilities IS NULL OR
        (jsonb_typeof(collaboration_capabilities) = 'object' AND pg_column_size(collaboration_capabilities) <= 16384));
ALTER TABLE collaboration_run ADD COLUMN preparation_snapshot jsonb
    CHECK (preparation_snapshot IS NULL OR
        (jsonb_typeof(preparation_snapshot) = 'object' AND pg_column_size(preparation_snapshot) <= 98304));

-- This trigger sorts before collaboration_runtime_config. Request one advance
-- through the existing epoch/outbox contract, including concurrent ordinary
-- configuration changes; never create a second kind of resource grant.
CREATE FUNCTION collaboration_prepare_capability_epoch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.collaboration_capabilities IS DISTINCT FROM OLD.collaboration_capabilities THEN
        IF NEW.collaboration_config_epoch NOT IN (OLD.collaboration_config_epoch, OLD.collaboration_config_epoch + 1) THEN
            RAISE EXCEPTION 'invalid collaboration capability epoch advance' USING ERRCODE = '23514';
        END IF;
        IF NEW.collaboration_capabilities IS NOT NULL AND
            (NEW.collaboration_capabilities->>'runtime_config_epoch') IS DISTINCT FROM (OLD.collaboration_config_epoch + 1)::text THEN
            RAISE EXCEPTION 'collaboration capability must bind its registered epoch' USING ERRCODE = '23514';
        END IF;
        NEW.collaboration_config_epoch := OLD.collaboration_config_epoch + 1;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_runtime_capability_epoch BEFORE UPDATE ON agent_runtime
    FOR EACH ROW EXECUTE FUNCTION collaboration_prepare_capability_epoch();

CREATE FUNCTION collaboration_preserve_run_preparation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.preparation_snapshot IS NOT NULL AND NEW.preparation_snapshot IS DISTINCT FROM OLD.preparation_snapshot THEN
        RAISE EXCEPTION 'collaboration claimed preparation is immutable' USING ERRCODE = '23514';
    END IF;
    IF OLD.preparation_snapshot IS NULL AND NEW.preparation_snapshot IS NOT NULL AND
        (OLD.prompt_snapshot IS NOT NULL OR NEW.prompt_snapshot IS NULL OR OLD.authorized_attempt_seq > 0 OR OLD.settled_at IS NOT NULL) THEN
        RAISE EXCEPTION 'collaboration preparation must freeze with its first claimed prompt' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_run_preparation_identity BEFORE UPDATE ON collaboration_run
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_run_preparation();

COMMIT;
