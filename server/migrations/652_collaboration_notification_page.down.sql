DO $$
BEGIN
    LOCK TABLE collaboration_notification IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_notification) THEN
        RAISE EXCEPTION 'collaboration notifications are retained; rollback refused';
    END IF;
    DROP INDEX IF EXISTS collaboration_notification_page_idx;
END $$;
