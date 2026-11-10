-- Highest migration refuses retained rows before any index removal.
-- This per-file lock does not replace maintenance throughout rollback.
DO $$
BEGIN
    IF to_regclass('agent_artifact_execution') IS NOT NULL THEN
        LOCK TABLE agent_artifact_execution IN ACCESS EXCLUSIVE MODE NOWAIT;
        IF EXISTS (SELECT 1 FROM agent_artifact_execution) THEN
            RAISE EXCEPTION 'cannot roll back retained artifact approval or execution evidence';
        END IF;
    END IF;
END;
$$;
