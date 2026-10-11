DO $$
BEGIN
    LOCK TABLE platform_admin_initialization IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM platform_admin_initialization) THEN
        RAISE EXCEPTION 'refusing to remove retained platform administrator initialization';
    END IF;
    DROP INDEX platform_admin_initialization_singleton_uidx;
END $$;
