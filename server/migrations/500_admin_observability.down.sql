DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM admin_alert) OR EXISTS (SELECT 1 FROM admin_alert_detector_state)
       OR EXISTS (SELECT 1 FROM admin_audit_event WHERE actor_display_name IS NOT NULL)
       OR EXISTS (SELECT 1 FROM agent_task_queue WHERE queued_at IS NOT NULL) THEN
        RAISE EXCEPTION 'administrative observations exist; preserve alert, audit and queue evidence';
    END IF;
END $$;
DROP TABLE admin_alert_detector_state,admin_alert;
ALTER TABLE admin_audit_event DROP COLUMN actor_display_name;
DROP TRIGGER agent_task_queue_clock ON agent_task_queue;
DROP FUNCTION multica_track_task_queue_time();
ALTER TABLE agent_task_queue DROP CONSTRAINT agent_task_queue_clock_consistent,DROP COLUMN queued_at_source,DROP COLUMN queued_at;
