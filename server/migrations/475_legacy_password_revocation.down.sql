DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM "user" WHERE legacy_password_sessions_revoked_at IS NOT NULL) THEN
        RAISE EXCEPTION 'legacy password sessions have been revoked; preserve the revocation boundary';
    END IF;
END $$;
ALTER TABLE "user" DROP COLUMN legacy_password_sessions_revoked_at;
