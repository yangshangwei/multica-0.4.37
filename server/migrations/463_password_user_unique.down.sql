-- Existing password accounts must retain credentials and uniqueness on rollback.
-- The lock also prevents registration racing the empty-table check and DDL.
DO $$
BEGIN
    IF to_regclass('user_password_credential') IS NOT NULL THEN
        LOCK TABLE user_password_credential IN ACCESS EXCLUSIVE MODE;
        IF EXISTS (SELECT 1 FROM user_password_credential) THEN
            RAISE EXCEPTION 'cannot roll back password authentication: password accounts exist';
        END IF;
    END IF;
    DROP INDEX IF EXISTS user_password_credential_user_uidx;
END;
$$;
