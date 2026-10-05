DO $$ BEGIN
LOCK TABLE workspace, project, project_state_change, project_update, project_update_revision, project_update_request, project_update_notification IN ACCESS EXCLUSIVE MODE NOWAIT;
IF EXISTS (SELECT 1 FROM project_state_change) OR EXISTS (SELECT 1 FROM project_update) OR EXISTS (SELECT 1 FROM project_update_revision) OR EXISTS (SELECT 1 FROM project_update_request) OR EXISTS (SELECT 1 FROM project_update_notification) OR EXISTS (SELECT 1 FROM project WHERE revision <> 1 OR description_revision <> 1 OR in_progress_since_source = 'transition') OR EXISTS (SELECT 1 FROM workspace WHERE planning_timezone IS NOT NULL) THEN RAISE EXCEPTION 'project P1 data exists; downgrade refused'; END IF;
DROP INDEX IF EXISTS project_update_revision_id;
END $$;
