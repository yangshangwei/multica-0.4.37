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
DROP INDEX IF EXISTS iteration_workspace_status_date;
END $$;
