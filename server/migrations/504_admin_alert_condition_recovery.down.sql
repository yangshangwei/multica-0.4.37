-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE admin_alert, admin_alert_detector_state IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM admin_alert) OR EXISTS (SELECT 1 FROM admin_alert_detector_state) THEN
        RAISE EXCEPTION 'administrative alert data exists; preserve detection and recovery indexes';
    END IF;
    DROP INDEX admin_alert_condition_recovery_idx;
END;
$$;
