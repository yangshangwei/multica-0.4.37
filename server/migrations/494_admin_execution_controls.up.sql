ALTER TABLE agent_task_queue
    ADD COLUMN state_version bigint NOT NULL DEFAULT 1 CHECK (state_version > 0),
    ADD COLUMN execution_admission_version bigint CHECK (execution_admission_version > 0);

CREATE FUNCTION multica_advance_task_state_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.state_version := OLD.state_version + 1;
    RETURN NEW;
END;
$$;

CREATE TRIGGER agent_task_queue_state_version
BEFORE UPDATE OF status, runtime_id, dispatched_at ON agent_task_queue
FOR EACH ROW
WHEN (ROW(OLD.status, OLD.runtime_id, OLD.dispatched_at) IS DISTINCT FROM ROW(NEW.status, NEW.runtime_id, NEW.dispatched_at))
EXECUTE FUNCTION multica_advance_task_state_version();

ALTER TABLE admin_operation
    ADD COLUMN execution_runtime_id uuid,
    ADD COLUMN execution_dispatched_at timestamptz,
    ADD COLUMN next_reconcile_at timestamptz,
    ADD COLUMN effects_completed_at timestamptz;
