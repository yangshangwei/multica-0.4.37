DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM admin_alert) OR EXISTS (SELECT 1 FROM admin_alert_detector_state) THEN
        RAISE EXCEPTION 'administrative alert data exists; preserve detection and recovery indexes';
    END IF;
END $$;
DROP INDEX admin_alert_queue_scan_idx;
