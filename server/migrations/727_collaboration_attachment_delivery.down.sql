DO $$
BEGIN
    LOCK TABLE collaboration_attachment IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_attachment) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration attachment delivery data';
    END IF;
    ALTER TABLE collaboration_attachment
        DROP CONSTRAINT collaboration_attachment_discard_undelivered,
        DROP CONSTRAINT collaboration_attachment_delivery_context,
        DROP CONSTRAINT collaboration_attachment_private_key,
        DROP COLUMN discarded_at,
        DROP COLUMN encryption_key,
        DROP COLUMN content_sha256,
        DROP COLUMN upload_request_id,
        DROP COLUMN intended_issue_id,
        DROP COLUMN source_workspace_id;
END $$;
