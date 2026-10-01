DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM agent_task_queue WHERE submitted_installation_id IS NOT NULL OR execution_installation_id IS NOT NULL OR execution_binding_id IS NOT NULL) THEN
        RAISE EXCEPTION 'execution installation attribution exists; preserve historical snapshots';
    END IF;
END $$;
ALTER TABLE agent_task_queue
    DROP COLUMN execution_binding_epoch,
    DROP COLUMN execution_binding_id,
    DROP COLUMN execution_installation_id,
    DROP COLUMN submitted_installation_id;
