DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM admin_alert_detector_state) THEN
        RAISE EXCEPTION 'detector observations exist; preserve cycle evidence';
    END IF;
END $$;
ALTER TABLE admin_alert_detector_state DROP COLUMN cycle_unknown_count;
