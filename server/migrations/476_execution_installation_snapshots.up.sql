ALTER TABLE agent_task_queue
    ADD COLUMN submitted_installation_id uuid,
    ADD COLUMN execution_installation_id uuid,
    ADD COLUMN execution_binding_id uuid,
    ADD COLUMN execution_binding_epoch bigint;
