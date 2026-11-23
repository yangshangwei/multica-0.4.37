DO $$
BEGIN
    LOCK TABLE collaboration_attachment IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_attachment) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration attachment upload identity';
    END IF;
    DROP INDEX collaboration_attachment_upload_request_idx;
END $$;
