DO $$
BEGIN
    LOCK TABLE collaboration_business_capability, collaboration_run, collaboration_termination IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_termination WHERE runtime_identity IS NOT NULL OR token_ciphertext IS NOT NULL)
        OR EXISTS (SELECT 1 FROM collaboration_business_capability)
        OR EXISTS (SELECT 1 FROM collaboration_run WHERE deployment_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration runtime capabilities';
    END IF;
    ALTER TABLE collaboration_termination DROP COLUMN runtime_identity,DROP COLUMN token_ciphertext;
    ALTER TABLE collaboration_business_capability DROP COLUMN token_ciphertext;
    ALTER TABLE collaboration_run DROP COLUMN deployment_id,DROP COLUMN resource_owner_user_id,DROP COLUMN containment;
END;
$$;
