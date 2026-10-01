DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM platform_role_binding) OR EXISTS (SELECT 1 FROM admin_operation)
       OR EXISTS (SELECT 1 FROM admin_audit_event) OR EXISTS (SELECT 1 FROM organization_workspace)
       OR EXISTS (SELECT 1 FROM "user" WHERE disabled_at IS NOT NULL) THEN
        RAISE EXCEPTION 'platform administration data exists; preserve authorization, operations and audit; use a forward fix';
    END IF;
END $$;
DROP INDEX IF EXISTS platform_role_user_uidx;
