DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM admin_alert) THEN
        RAISE EXCEPTION 'administrative alert data exists; preserve episode history lookup';
    END IF;
END $$;
DROP INDEX admin_alert_latest_episode_idx;
