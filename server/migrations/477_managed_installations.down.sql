-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE daemon_token, installation_challenge, installation_daemon_binding, installation_report_cursor, installation_user, managed_installation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM managed_installation) OR EXISTS (SELECT 1 FROM installation_daemon_binding)
       OR EXISTS (SELECT 1 FROM installation_challenge) THEN
        RAISE EXCEPTION 'managed installation data exists; preserve identities, bindings and proof history';
    END IF;
    ALTER TABLE daemon_token DROP COLUMN installation_binding_epoch, DROP COLUMN installation_binding_id;
    DROP TABLE installation_report_cursor, installation_challenge, installation_daemon_binding, installation_user, managed_installation;
END;
$$;
