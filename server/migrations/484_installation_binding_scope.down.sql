DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM managed_installation) OR EXISTS (SELECT 1 FROM installation_daemon_binding)
       OR EXISTS (SELECT 1 FROM installation_challenge) THEN
        RAISE EXCEPTION 'managed installation data exists; preserve identities, bindings and proof history';
    END IF;
END $$;
DROP INDEX IF EXISTS installation_binding_scope_idx;
