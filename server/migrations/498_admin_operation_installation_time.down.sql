-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE admin_operation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM admin_operation WHERE kind IN ('task.cancel','installation.admission')) THEN
        RAISE EXCEPTION 'administrative execution controls exist; preserve operation coordination indexes';
    END IF;
    DROP INDEX admin_operation_installation_time_idx;
END;
$$;
