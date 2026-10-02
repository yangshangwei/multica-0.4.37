-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE agent_task_queue IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM agent_task_queue WHERE claim_generation > 0) THEN
        RAISE EXCEPTION 'task delivery generations exist; preserve stale delivery protection';
    END IF;
    ALTER TABLE agent_task_queue DROP COLUMN claim_generation;
END;
$$;
