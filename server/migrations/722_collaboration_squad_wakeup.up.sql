ALTER TABLE collaboration_execution_group ADD COLUMN wake_pending boolean NOT NULL DEFAULT false,
    ADD COLUMN wake_parent_run_id uuid,
    ADD CONSTRAINT collaboration_squad_wakeup_parent CHECK (NOT wake_pending OR wake_parent_run_id IS NOT NULL);
