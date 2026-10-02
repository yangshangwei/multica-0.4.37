-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE admin_alert_detector_state IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM admin_alert_detector_state) THEN
        RAISE EXCEPTION 'detector observations exist; preserve cycle evidence';
    END IF;
    ALTER TABLE admin_alert_detector_state DROP COLUMN cycle_unknown_count;
END;
$$;
