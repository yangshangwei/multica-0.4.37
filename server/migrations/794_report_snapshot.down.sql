DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM report_snapshot) THEN
        RAISE EXCEPTION 'retained report snapshots prevent rollback';
    END IF;
    DROP TABLE report_snapshot;
END $$;
