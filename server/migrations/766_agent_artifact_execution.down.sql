-- Maintenance only: stop all writers throughout the multi-file rollback.
DO $$
BEGIN
    IF to_regclass('agent_artifact_execution') IS NOT NULL THEN
        LOCK TABLE agent_artifact_execution IN ACCESS EXCLUSIVE MODE NOWAIT;
        IF EXISTS (SELECT 1 FROM agent_artifact_execution) THEN
            RAISE EXCEPTION 'cannot roll back retained artifact approval or execution evidence';
        END IF;
        DROP TABLE agent_artifact_execution;
    END IF;
END;
$$;
