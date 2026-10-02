-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE installation_challenge, installation_daemon_binding, managed_installation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM managed_installation) OR EXISTS (SELECT 1 FROM installation_daemon_binding)
       OR EXISTS (SELECT 1 FROM installation_challenge) THEN
        RAISE EXCEPTION 'managed installation data exists; preserve identities, bindings and proof history';
    END IF;
    DROP INDEX IF EXISTS installation_challenge_id_uidx;
END;
$$;
