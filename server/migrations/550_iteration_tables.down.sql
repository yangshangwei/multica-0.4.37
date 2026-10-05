DO $$ BEGIN
LOCK TABLE issue, workspace_iteration_settings, iteration, iteration_participation, iteration_event, iteration_snapshot, iteration_operation, iteration_notification IN ACCESS EXCLUSIVE MODE NOWAIT;
IF EXISTS (SELECT 1 FROM issue WHERE current_iteration_id IS NOT NULL OR iteration_rollover_count <> 0)
OR EXISTS (SELECT 1 FROM workspace_iteration_settings)
OR EXISTS (SELECT 1 FROM iteration)
OR EXISTS (SELECT 1 FROM iteration_participation)
OR EXISTS (SELECT 1 FROM iteration_event)
OR EXISTS (SELECT 1 FROM iteration_snapshot)
OR EXISTS (SELECT 1 FROM iteration_operation)
OR EXISTS (SELECT 1 FROM iteration_notification)
THEN RAISE EXCEPTION 'iteration data exists; downgrade refused'; END IF;
DROP TABLE iteration_notification, iteration_operation, iteration_snapshot, iteration_event, iteration_participation, iteration, workspace_iteration_settings;
ALTER TABLE issue DROP COLUMN current_iteration_id, DROP COLUMN iteration_rollover_count;
END $$;
