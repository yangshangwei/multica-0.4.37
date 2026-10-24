DO $$
BEGIN
    LOCK TABLE collaboration_runtime_scan_cursor IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_runtime_scan_cursor) THEN
        RAISE EXCEPTION 'collaboration runtime scan identity must be retained';
    END IF;
    DROP INDEX idx_collaboration_runtime_scan_cursor_identity;
END $$;
