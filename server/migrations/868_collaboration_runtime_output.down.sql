DO $$
BEGIN
    LOCK TABLE collaboration_attachment IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_attachment
        WHERE uploader_agent_id IS NOT NULL OR run_id IS NOT NULL OR attempt_seq IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove retained collaboration runtime outputs';
    END IF;
    ALTER TABLE collaboration_attachment
        DROP CONSTRAINT collaboration_attachment_uploader_identity,
        DROP CONSTRAINT collaboration_attachment_delivery_context,
        DROP COLUMN attempt_seq,
        DROP COLUMN run_id,
        DROP COLUMN uploader_agent_id,
        ALTER COLUMN uploader_user_id SET NOT NULL,
        ADD CONSTRAINT collaboration_attachment_delivery_context CHECK (
            encryption_key IS NULL OR (
                source_workspace_id IS NOT NULL AND draft_id IS NOT NULL
                AND upload_request_id IS NOT NULL AND length(upload_request_id) BETWEEN 1 AND 128
                AND content_sha256 IS NOT NULL AND content_sha256 ~ '^[0-9a-f]{64}$'
                AND size_bytes <= 104857600
                AND storage_key LIKE 'private/collaboration/%'
                AND (intended_issue_id IS NULL OR issue_id IS NULL OR intended_issue_id = issue_id)
            )
        );
END $$;
