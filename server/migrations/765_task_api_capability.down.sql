DO $$
BEGIN
    LOCK TABLE task_token IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM task_token WHERE api_capability IS NOT NULL) THEN
        RAISE EXCEPTION 'revoke task capabilities before rolling back this migration';
    END IF;
    ALTER TABLE task_token DROP COLUMN api_capability;
END $$;
