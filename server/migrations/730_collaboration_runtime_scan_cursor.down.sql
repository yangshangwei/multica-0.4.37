DO $$
BEGIN
    LOCK TABLE collaboration_runtime_scan_cursor IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_runtime_scan_cursor) THEN
        RAISE EXCEPTION 'collaboration runtime scan positions must be retained';
    END IF;
    DROP TABLE collaboration_runtime_scan_cursor;
END $$;
