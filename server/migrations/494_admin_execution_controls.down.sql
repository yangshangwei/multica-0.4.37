-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE admin_operation, agent_task_queue, managed_installation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM admin_operation WHERE kind IN ('task.cancel','installation.admission'))
       OR EXISTS (SELECT 1 FROM managed_installation WHERE admission='stopped')
       OR EXISTS (SELECT 1 FROM agent_task_queue WHERE execution_admission_version IS NOT NULL) THEN
        RAISE EXCEPTION 'administrative execution controls exist; preserve admission, execution fences and operation history';
    END IF;
    ALTER TABLE admin_operation DROP COLUMN effects_completed_at, DROP COLUMN next_reconcile_at,
        DROP COLUMN execution_dispatched_at, DROP COLUMN execution_runtime_id;
    DROP TRIGGER agent_task_queue_state_version ON agent_task_queue;
    DROP FUNCTION multica_advance_task_state_version();
    ALTER TABLE agent_task_queue DROP COLUMN execution_admission_version, DROP COLUMN state_version;
END;
$$;
