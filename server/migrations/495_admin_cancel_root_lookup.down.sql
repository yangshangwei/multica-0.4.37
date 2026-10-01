DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM admin_operation WHERE kind IN ('task.cancel','installation.admission')) THEN
        RAISE EXCEPTION 'administrative execution controls exist; preserve operation coordination indexes';
    END IF;
END $$;
DROP INDEX admin_cancel_root_lookup_idx;
