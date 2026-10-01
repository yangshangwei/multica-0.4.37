DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM task_token WHERE installation_binding_id IS NOT NULL) THEN
        RAISE EXCEPTION 'bound task credentials exist; preserve their installation authorization scope';
    END IF;
END $$;
ALTER TABLE task_token DROP COLUMN installation_binding_epoch, DROP COLUMN installation_binding_id;
