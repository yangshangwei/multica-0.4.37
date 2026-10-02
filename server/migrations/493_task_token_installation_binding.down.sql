-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE task_token IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM task_token WHERE installation_binding_id IS NOT NULL) THEN
        RAISE EXCEPTION 'bound task credentials exist; preserve their installation authorization scope';
    END IF;
    ALTER TABLE task_token DROP COLUMN installation_binding_epoch, DROP COLUMN installation_binding_id;
END;
$$;
