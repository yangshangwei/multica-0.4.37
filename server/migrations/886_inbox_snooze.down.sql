DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM inbox_snooze WHERE snoozed_until IS NOT NULL) THEN
        RAISE EXCEPTION 'restore retained inbox snoozes before rollback';
    END IF;
END $$;
DROP TABLE IF EXISTS inbox_critical_source;
DROP TABLE IF EXISTS inbox_snooze;
