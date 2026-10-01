DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM agent_task_queue WHERE claim_generation > 0) THEN
        RAISE EXCEPTION 'task delivery generations exist; preserve stale delivery protection';
    END IF;
END $$;
ALTER TABLE agent_task_queue DROP COLUMN claim_generation;
