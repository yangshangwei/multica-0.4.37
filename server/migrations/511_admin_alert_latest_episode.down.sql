-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE admin_alert IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM admin_alert) THEN
        RAISE EXCEPTION 'administrative alert data exists; preserve episode history lookup';
    END IF;
    DROP INDEX admin_alert_latest_episode_idx;
END;
$$;
